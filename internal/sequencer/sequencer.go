package sequencer

import (
	"math"
	"strconv"
	"strings"

	"github.com/cbegin/mmlfm-go/internal/mml"
)

type VoiceEngine interface {
	NoteOn(note int, velocity int, pan int, program int) int
	NoteOff(id int)
	RenderFrame() (float32, float32)
	SetMasterGain(gain float64)
	// ActiveVoiceCount returns the number of voices still sounding (attack/decay/sustain/release).
	// Used to detect when playback has fully ended including release tails.
	ActiveVoiceCount() int
	// SetFilterType sets output filter: 0=LP, 1=BP, 2=HP. Called when %f changes.
	SetFilterType(filterType int)
	// SetNoteOnPhase sets phase for next NoteOn: 0=reset, -1=random, 1-255=phase/128*PI.
	SetNoteOnPhase(phase int)
	// SetPortamento sets glide for next NoteOn: fromNote<0 = no portamento, frames = glide duration in samples.
	SetPortamento(fromNote int, frames int)
	// SetPitchLFO configures per-frame pitch modulation. depth is in semitones.
	SetPitchLFO(depth float64, rateHz float64, waveform int)
	// SetAmpLFO configures per-frame amplitude modulation. depth is a 0-1 factor.
	SetAmpLFO(depth float64, rateHz float64, waveform int)
	// SetFilterLFO configures per-frame filter cutoff modulation. depth is in cutoff units.
	SetFilterLFO(depth float64, rateHz float64, waveform int)
}

// EventKind identifies sequencer lifecycle events.
type EventKind int

const (
	EventLoopCompleted EventKind = iota
	EventPlaybackEnded
	EventTrigger
)

// TriggerEvent carries %t/%e trigger data when EventKind is EventTrigger.
type TriggerEvent struct {
	TriggerID   int
	NoteOnType  int
	NoteOffType int
}

type Options struct {
	LoopWholeScore    bool
	OnEvent           func(EventKind)
	OnTrigger         func(TriggerEvent)
	ReleaseTailFrames int // extra frames to render after last voice ends (0 = use 0.1s default)
	MasterTranspose   int // master octave shift applied to all notes (in octaves, e.g. -2..+2)
}

type tableData struct {
	values    []int
	loopStart int // index where looping begins (-1 = loop from 0)
}

type Sequencer struct {
	score               *mml.Score
	engine              VoiceEngine
	sampleRate          int
	ticksPerSamp        float64
	initialTicksPerSamp float64
	tickFrac            float64
	tickInt             int
	trackState          []trackCursor
	trackRuntime        []runtimeState
	tableDefs           map[int]tableData
	noteOffs            []noteOff
	loopWholeScore      bool
	pendingReset        bool
	onEvent             func(EventKind)
	onTrigger           func(TriggerEvent)
	playbackEndedFired  bool
	commandExhausted    bool // score done + all note-offs; waiting for engine release
	releaseTailFrames   int  // countdown after last voice; fire when 0
	loopPending         bool // reached end of loop; waiting for engine release before reset
	loopTailCountdown   int  // frames of silence after last voice before loop reset
	masterTranspose     int  // master octave shift in semitones
	patchMods           map[int]patchMod
}

type trackCursor struct {
	events    []mml.Event
	index     int
	loopIndex int
	loopTick  int
	endTick   int
	loopCycle int
}

type runtimeState struct {
	volume      int
	fineVolume  int
	expression  int
	vScaleMode  int
	vScaleMax   int
	xScaleMode  int
	pan         int
	program     int
	module      int
	channel     int
	delay       int
	slur        mml.SlurMode
	transpose   int
	detune      int
	filterCut   int
	filterType  int
	filterEnv   filterEnvelope
	filterEnvOn bool
	phase       int
	portamento  int
	lfoRate     int
	lfoDepth    int
	lfoWave     int
	modPitch    int
	modAmp      int
	modPan      int
	modFilter   int
	tableStep   map[string]int
	tableStart  map[string]int
	tableRate   map[string]int
	mask        int
	lastVoice   int
	lastNote    int
	mpEnd       int
	mpDelay     int
	mpChange    int
	maEnd       int
	maDelay     int
	maChange    int
	mfEnd       int
	mfDelay     int
	mfChange    int
	fpsRate     int
}

type patchMod struct {
	mpArgs []int // mp depth, end, delay, change
	maArgs []int // ma depth, end, delay, change
	mfArgs []int // mf depth, end, delay, change
}

type noteOff struct {
	tick  int
	voice int
	fired bool
}

// filterEnvelope holds the @f envelope parameters.
// @f co, res, ar, dr, sr, rr, co2, co3, sc, rc
// co=initial cutoff, ar/dr/sr/rr=attack/decay/sustain/release rates,
// co2=attack peak, co3=sustain level, sc=sustain cutoff, rc=release cutoff.
type filterEnvelope struct {
	co      int // initial cutoff
	ar      int // attack rate (frames to reach co2)
	dr      int // decay rate (frames from co2 to co3)
	sr      int // sustain rate (frames from co3 to sc)
	rr      int // release rate
	co2     int // attack peak cutoff
	co3     int // sustain cutoff
	sc      int // sustain end cutoff
	rc      int // release cutoff
	state   int // 0=attack, 1=decay, 2=sustain, 3=release
	frame   int // frames elapsed in current stage
	current int // current cutoff value
}

func (fe *filterEnvelope) step() int {
	switch fe.state {
	case 0: // attack: co -> co2 over ar frames
		if fe.ar <= 0 {
			fe.current = fe.co2
			fe.state = 1
			fe.frame = 0
		} else {
			fe.current = fe.co + ((fe.co2-fe.co)*fe.frame)/fe.ar
			fe.frame++
			if fe.frame >= fe.ar {
				fe.current = fe.co2
				fe.state = 1
				fe.frame = 0
			}
		}
	case 1: // decay: co2 -> co3 over dr frames
		if fe.dr <= 0 {
			fe.current = fe.co3
			fe.state = 2
			fe.frame = 0
		} else {
			fe.current = fe.co2 + ((fe.co3-fe.co2)*fe.frame)/fe.dr
			fe.frame++
			if fe.frame >= fe.dr {
				fe.current = fe.co3
				fe.state = 2
				fe.frame = 0
			}
		}
	case 2: // sustain: co3 -> sc over sr frames
		if fe.sr <= 0 {
			fe.current = fe.sc
		} else {
			fe.current = fe.co3 + ((fe.sc-fe.co3)*fe.frame)/fe.sr
			fe.frame++
			if fe.frame >= fe.sr {
				fe.current = fe.sc
			}
		}
	case 3: // release: current -> rc over rr frames
		if fe.rr <= 0 {
			fe.current = fe.rc
		} else {
			start := fe.current
			fe.current = start + ((fe.rc-start)*fe.frame)/fe.rr
			fe.frame++
			if fe.frame >= fe.rr {
				fe.current = fe.rc
			}
		}
	}
	return clampInt(fe.current, 0, 128)
}

func New(score *mml.Score, engine VoiceEngine, sampleRate int) *Sequencer {
	return NewWithOptions(score, engine, sampleRate, Options{})
}

func NewWithOptions(score *mml.Score, engine VoiceEngine, sampleRate int, opts Options) *Sequencer {
	tailFrames := opts.ReleaseTailFrames
	if tailFrames <= 0 {
		tailFrames = sampleRate / 2
	}
	s := &Sequencer{
		score:             score,
		engine:            engine,
		sampleRate:        sampleRate,
		loopWholeScore:    opts.LoopWholeScore,
		onEvent:           opts.OnEvent,
		onTrigger:         opts.OnTrigger,
		releaseTailFrames: tailFrames,
		masterTranspose:   opts.MasterTranspose * 12,
	}
	bpm := score.InitialBPM
	if bpm <= 0 {
		bpm = 120
	}
	s.ticksPerSamp = (bpm * float64(score.Resolution)) / (240.0 * float64(sampleRate))
	s.initialTicksPerSamp = s.ticksPerSamp
	s.trackState = make([]trackCursor, len(score.Tracks))
	s.trackRuntime = make([]runtimeState, len(score.Tracks))
	s.tableDefs = parseTableDefinitions(score.Definitions)
	s.patchMods = parsePatchMods(score.Definitions)
	for i, tr := range score.Tracks {
		s.trackState[i] = trackCursor{
			events:    tr.Events,
			index:     0,
			loopIndex: tr.LoopIndex,
			loopTick:  tr.LoopTick,
			endTick:   tr.EndTick,
		}
		s.trackRuntime[i] = runtimeState{
			volume:     16,
			fineVolume: 127,
			expression: 128,
			vScaleMax:  16,
			filterCut:  128,
			filterType: 0,
			phase:      0,
			portamento: 0,
			tableStep:  map[string]int{},
			tableStart: map[string]int{},
			tableRate:  map[string]int{},
			lastVoice:  -1,
		}
	}
	return s
}

func (s *Sequencer) Process(dst []float32) {
	frames := len(dst) / 2
	for f := 0; f < frames; f++ {
		s.tickFrac += s.ticksPerSamp
		nextTick := int(s.tickFrac)
		for s.tickInt <= nextTick {
			s.dispatchTick(s.tickInt)
			s.tickInt++
		}
		if s.pendingReset {
			s.resetForWholeScoreLoop()
		}
		l, r := s.engine.RenderFrame()
		dst[f*2] = l
		dst[f*2+1] = r
		if s.loopPending && s.engine.ActiveVoiceCount() == 0 {
			if s.loopTailCountdown <= 0 {
				s.loopPending = false
				s.pendingReset = true
				if s.onEvent != nil {
					s.onEvent(EventLoopCompleted)
				}
			} else {
				s.loopTailCountdown--
			}
		}
		if s.commandExhausted && !s.playbackEndedFired && s.engine.ActiveVoiceCount() == 0 {
			if s.releaseTailFrames <= 0 {
				s.playbackEndedFired = true
				if s.onEvent != nil {
					s.onEvent(EventPlaybackEnded)
				}
			} else {
				s.releaseTailFrames--
			}
		}
	}
}

func (s *Sequencer) dispatchTick(tick int) {
	for trkIdx := range s.trackState {
		tc := &s.trackState[trkIdx]
		for {
			ev, effectiveTick, ok := s.peekEvent(tc)
			if !ok || effectiveTick > tick {
				break
			}
			s.applyEvent(trkIdx, tc, ev, effectiveTick)
			tc.index++
			if tc.index >= len(tc.events) && tc.loopIndex >= 0 && tc.endTick > tc.loopTick {
				tc.index = tc.loopIndex
				tc.loopCycle++
			}
		}
	}
	for i := range s.noteOffs {
		if !s.noteOffs[i].fired && s.noteOffs[i].tick <= tick {
			s.engine.NoteOff(s.noteOffs[i].voice)
			s.noteOffs[i].fired = true
		}
	}
	s.compactNoteOffs()
	if len(s.noteOffs) == 0 && s.scoreExhausted() {
		if s.loopWholeScore {
			if !s.loopPending {
				s.loopPending = true
				s.loopTailCountdown = s.releaseTailFrames
			}
		} else {
			s.commandExhausted = true
		}
	}
}

func (s *Sequencer) scoreExhausted() bool {
	for _, tc := range s.trackState {
		if tc.index < len(tc.events) {
			return false
		}
		if tc.loopIndex >= 0 && tc.endTick > tc.loopTick {
			return false
		}
	}
	return true
}

func (s *Sequencer) resetForWholeScoreLoop() {
	s.pendingReset = false
	s.loopPending = false
	s.tickFrac = 0
	s.tickInt = 0
	s.ticksPerSamp = s.initialTicksPerSamp
	s.noteOffs = s.noteOffs[:0]
	for i, tr := range s.score.Tracks {
		s.trackState[i].index = 0
		s.trackState[i].loopCycle = 0
		s.trackState[i].events = tr.Events
		s.trackState[i].loopIndex = tr.LoopIndex
		s.trackState[i].loopTick = tr.LoopTick
		s.trackState[i].endTick = tr.EndTick
		s.trackRuntime[i] = runtimeState{
			volume:     16,
			fineVolume: 127,
			expression: 128,
			vScaleMax:  16,
			filterCut:  128,
			filterType: 0,
			phase:      0,
			portamento: 0,
			tableStep:  map[string]int{},
			tableStart: map[string]int{},
			tableRate:  map[string]int{},
			lastVoice:  -1,
		}
	}
}

func (s *Sequencer) applyEvent(trackIndex int, tc *trackCursor, ev mml.Event, eventTick int) {
	rt := &s.trackRuntime[trackIndex]
	if ma, ok := s.engine.(interface{ SetCurrentModule(int) }); ok {
		ma.SetCurrentModule(rt.module)
	}
	switch ev.Type {
	case mml.EventTempo:
		// repeat handling: ignore tempo commands inside repeated loop bodies.
		if tc.loopCycle > 0 && tc.loopIndex >= 0 && ev.Tick >= tc.loopTick {
			return
		}
		s.ticksPerSamp = (float64(ev.Value) * float64(s.score.Resolution)) / (240.0 * float64(s.sampleRate))
	case mml.EventVolume:
		if rt.mask&0x01 != 0 {
			return
		}
		rt.volume = ev.Value
	case mml.EventFineVolume:
		if rt.mask&0x01 != 0 {
			return
		}
		rt.fineVolume = ev.Value
	case mml.EventExpression:
		if rt.mask&0x01 != 0 {
			return
		}
		rt.expression = ev.Value
	case mml.EventPan:
		if rt.mask&0x02 != 0 {
			return
		}
		rt.pan = ev.Value
	case mml.EventProgram:
		rt.program = ev.Value
		if pm, ok := s.patchMods[ev.Value]; ok {
			if pm.mpArgs != nil {
				rt.modPitch = pm.mpArgs[0]
				if len(pm.mpArgs) >= 2 {
					rt.mpEnd = pm.mpArgs[1]
					rt.lfoDepth = absInt(pm.mpArgs[1])
				}
				if len(pm.mpArgs) >= 3 {
					rt.mpDelay = pm.mpArgs[2]
				}
				if len(pm.mpArgs) >= 4 {
					rt.mpChange = pm.mpArgs[3]
				}
			}
			if pm.maArgs != nil {
				rt.modAmp = pm.maArgs[0]
				if len(pm.maArgs) >= 2 {
					rt.maEnd = pm.maArgs[1]
				}
				if len(pm.maArgs) >= 3 {
					rt.maDelay = pm.maArgs[2]
				}
				if len(pm.maArgs) >= 4 {
					rt.maChange = pm.maArgs[3]
				}
			}
			if pm.mfArgs != nil {
				rt.modFilter = pm.mfArgs[0]
				if len(pm.mfArgs) >= 2 {
					rt.mfEnd = pm.mfArgs[1]
				}
				if len(pm.mfArgs) >= 3 {
					rt.mfDelay = pm.mfArgs[2]
				}
				if len(pm.mfArgs) >= 4 {
					rt.mfChange = pm.mfArgs[3]
				}
			}
			s.updateEngineLFO(rt)
		}
	case mml.EventModule:
		rt.module = ev.Module
		rt.channel = ev.Channel
	case mml.EventQuantize:
		if rt.mask&0x04 != 0 {
			return
		}
		// q is already applied when parser computes note duration.
	case mml.EventKeyOnDelay:
		rt.delay = ev.Delay
	case mml.EventTranspose:
		rt.transpose = ev.Value
	case mml.EventDetune:
		rt.detune = ev.Value
	case mml.EventSlur:
		rt.slur = ev.Slur
	case mml.EventTableEnv:
		if rt.mask&0x10 != 0 {
			return
		}
		s.applyTableEnv(rt, ev)
	case mml.EventControl:
		s.applyControl(rt, ev)
	case mml.EventNote:
		if ev.Slur != mml.SlurNone && rt.lastVoice >= 0 {
			// Close previous voice at the slur boundary to avoid hanging-note
			// accumulation when using polyphonic NoteOn-per-event engines.
			s.engine.NoteOff(rt.lastVoice)
			s.cancelPendingNoteOff(rt.lastVoice)
		}
		vel := ev.Value
		if vel <= 0 {
			vel = applyScaledVelocity(rt.volume, rt.expression, rt.fineVolume, rt.vScaleMode, rt.vScaleMax, rt.xScaleMode)
		}
		note := ev.Note + rt.transpose + rt.detune/64 + s.masterTranspose
		note += s.sampleTable(rt, "nt", 16, eventTick)
		note += s.sampleLFO(rt, eventTick)
		note = clampInt(note, 0, 127)
		pan := rt.pan
		if rt.mask&0x02 == 0 && ev.Pan != 0 {
			pan = ev.Pan
		}
		pan += s.sampleTable(rt, "np", 1, eventTick)
		pan = clampInt(pan, -64, 64)
		program := ev.Program
		if program == 0 {
			program = rt.program
		}
		// Encode module/channel into high bits for compatibility routing.
		program = program + (rt.module << 8) + (rt.channel << 16)
		vel = s.applyAmpControls(rt, vel, eventTick)
		program += (clampInt(rt.filterCut, 0, 255) << 24)
		s.engine.SetNoteOnPhase(rt.phase)
		portamentoFrames := 0
		if rt.portamento > 0 && rt.lastVoice >= 0 {
			portamentoFrames = (rt.portamento * s.sampleRate) / 1000
			if portamentoFrames < 1 {
				portamentoFrames = 1
			}
		}
		s.engine.SetPortamento(rt.lastNote, portamentoFrames)
		s.updateEngineLFO(rt)
		voiceID := s.engine.NoteOn(note, vel, pan, program)
		rt.lastVoice = voiceID
		rt.lastNote = note
		offTick := eventTick + ev.Duration
		if ev.GateTick >= 0 {
			offTick = eventTick + ev.GateTick
		}
		if ev.Delay > 0 {
			offTick += ev.Delay
		}
		s.noteOffs = append(s.noteOffs, noteOff{
			tick:  offTick,
			voice: voiceID,
		})
	}
}

func (s *Sequencer) applyControl(rt *runtimeState, ev mml.Event) {
	cmd := strings.ToLower(strings.TrimSpace(ev.Command))
	switch cmd {
	case "@mask":
		rt.mask = clampInt(ev.Value, 0, 63)
	case "%v":
		rt.vScaleMode = ev.Value
		if len(ev.Values) > 1 && ev.Values[1] > 0 {
			rt.vScaleMax = ev.Values[1]
		}
	case "%x":
		rt.xScaleMode = ev.Value
	case "%f":
		if ev.Value >= 0 && ev.Value <= 2 {
			rt.filterType = ev.Value
			s.engine.SetFilterType(ev.Value)
		}
	case "%t":
		if s.onTrigger != nil {
			te := TriggerEvent{TriggerID: ev.Value}
			if len(ev.Values) >= 2 {
				te.NoteOnType = ev.Values[1]
			}
			if len(ev.Values) >= 3 {
				te.NoteOffType = ev.Values[2]
			}
			s.onTrigger(te)
		}
	case "%e":
		if s.onTrigger != nil {
			te := TriggerEvent{TriggerID: ev.Value}
			if len(ev.Values) >= 2 {
				te.NoteOnType = ev.Values[1]
			}
			s.onTrigger(te)
		}
	case "po":
		rt.portamento = ev.Value
	case "*":
		// Pitch slide: *N glides to next note. * or *0 uses default 50ms.
		if ev.Value > 0 {
			rt.portamento = ev.Value
		} else {
			rt.portamento = 50
		}
	case "@ph":
		rt.phase = ev.Value
	case "@f":
		if rt.mask&0x20 != 0 {
			return
		}
		rt.filterCut = ev.Value
		// Full @f envelope: co, res, ar, dr, sr, rr, co2, co3, sc, rc
		if len(ev.Values) >= 3 {
			fe := filterEnvelope{co: ev.Value, current: ev.Value}
			// defaults: co2=co, co3=co, sc=co, rc=0
			fe.co2 = ev.Value
			fe.co3 = ev.Value
			fe.sc = ev.Value
			if len(ev.Values) >= 3 {
				fe.ar = ev.Values[2]
			}
			if len(ev.Values) >= 4 {
				fe.dr = ev.Values[3]
			}
			if len(ev.Values) >= 5 {
				fe.sr = ev.Values[4]
			}
			if len(ev.Values) >= 6 {
				fe.rr = ev.Values[5]
			}
			if len(ev.Values) >= 7 {
				fe.co2 = ev.Values[6]
			}
			if len(ev.Values) >= 8 {
				fe.co3 = ev.Values[7]
			}
			if len(ev.Values) >= 9 {
				fe.sc = ev.Values[8]
			}
			if len(ev.Values) >= 10 {
				fe.rc = ev.Values[9]
			}
			rt.filterEnv = fe
			rt.filterEnvOn = true
		}
	case "@lfo":
		if rt.mask&0x20 != 0 {
			return
		}
		rt.lfoWave = ev.Value
		parseCSVInts(ev.Text, func(idx int, v int) {
			if idx == 0 {
				rt.lfoRate = v
			}
		})
		s.updateEngineLFO(rt)
	case "mp":
		if rt.mask&0x20 != 0 {
			return
		}
		rt.modPitch = ev.Value
		args := parseCSV(ev.Text)
		if len(args) >= 1 {
			rt.mpEnd = args[0]
			rt.lfoDepth = absInt(args[0])
		}
		if len(args) >= 2 {
			rt.mpDelay = args[1]
		}
		if len(args) >= 3 {
			rt.mpChange = args[2]
		}
		s.updateEngineLFO(rt)
	case "ma":
		if rt.mask&0x20 != 0 {
			return
		}
		rt.modAmp = ev.Value
		args := parseCSV(ev.Text)
		if len(args) >= 1 {
			rt.maEnd = args[0]
		}
		if len(args) >= 2 {
			rt.maDelay = args[1]
		}
		if len(args) >= 3 {
			rt.maChange = args[2]
		}
		s.updateEngineLFO(rt)
	case "mf":
		if rt.mask&0x20 != 0 {
			return
		}
		rt.modFilter = ev.Value
		args := parseCSV(ev.Text)
		if len(args) >= 1 {
			rt.mfEnd = args[0]
		}
		if len(args) >= 2 {
			rt.mfDelay = args[1]
		}
		if len(args) >= 3 {
			rt.mfChange = args[2]
		}
		s.updateEngineLFO(rt)
	case "s":
		// approximate sustain/release shaping via filter/amp bias
		if ev.Value > 0 {
			rt.modAmp = ev.Value
		}
	case "@al":
		if e, ok := s.engine.(interface {
			SetAlgorithm(int)
			SetOperatorCount(int)
		}); ok {
			e.SetOperatorCount(ev.Value)
			alg := -1
			if ev.Text != "" {
				args := parseCSV(ev.Text)
				if len(args) >= 1 {
					alg = args[0]
				}
			}
			if alg >= 0 {
				e.SetAlgorithm(alg)
			}
		}
	case "@fb":
		if e, ok := s.engine.(interface{ SetFeedback(float64) }); ok {
			e.SetFeedback(float64(ev.Value) / 7.0)
		}
	case "@fps":
		if ev.Value > 0 {
			rt.fpsRate = ev.Value
		}
	}
}

func (s *Sequencer) applyTableEnv(rt *runtimeState, ev mml.Event) {
	cmd := strings.ToLower(strings.TrimSpace(ev.Command))
	isRelease := strings.HasPrefix(cmd, "_")
	cmd = strings.TrimPrefix(cmd, "_")
	cmd = strings.TrimPrefix(cmd, "@")
	kind := cmd
	if isRelease {
		kind = "r" + kind
	}
	switch cmd {
	case "na", "nt", "np", "nf", "@":
		rt.tableStep[kind] = 0
		rt.tableStart[kind] = ev.Tick
		step := ev.Delay
		if step <= 0 {
			step = 1
		}
		rt.tableRate[kind] = step
		switch cmd {
		case "na":
			rt.modAmp = ev.Value
		case "nt":
			rt.modPitch = ev.Value
		case "np":
			rt.modPan = ev.Value
		case "nf":
			rt.modFilter = ev.Value
		case "@":
			// @@ table for timbre maps to generic modulation depth.
			rt.modPitch = ev.Value
		}
	}
}

func (s *Sequencer) sampleTable(rt *runtimeState, kind string, scale int, eventTick int) int {
	_, ok := rt.tableStep[kind]
	if !ok {
		return 0
	}
	tableID := 0
	switch kind {
	case "na":
		tableID = rt.modAmp
	case "nt":
		tableID = rt.modPitch
	case "np":
		tableID = rt.modPan
	case "nf":
		tableID = rt.modFilter
	}
	td := s.tableDefs[tableID]
	if len(td.values) == 0 {
		return 0
	}
	rate := rt.tableRate[kind]
	if rate <= 0 {
		rate = 1
	}
	ticksPerFrame := s.score.Resolution / 4
	fps := rt.fpsRate
	if fps <= 0 {
		if raw, ok := s.score.Definitions["FPS"]; ok {
			if fv, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && fv > 0 {
				fps = fv
			}
		}
	}
	if fps > 0 {
		ticksPerFrame = int(math.Max(1, (60.0/float64(fps))*float64(s.score.Resolution)/4.0))
	}
	start := rt.tableStart[kind]
	idx := 0
	if eventTick > start {
		idx = (eventTick - start) / (ticksPerFrame * rate)
	}
	var v int
	if idx < len(td.values) {
		v = td.values[idx]
	} else {
		loopStart := td.loopStart
		if loopStart < 0 {
			loopStart = 0
		}
		loopLen := len(td.values) - loopStart
		if loopLen <= 0 {
			v = td.values[len(td.values)-1]
		} else {
			v = td.values[loopStart+(idx-loopStart)%loopLen]
		}
	}
	if scale <= 0 {
		return v
	}
	return v / scale
}

func (s *Sequencer) sampleLFO(rt *runtimeState, tick int) int {
	if rt.lfoDepth == 0 || rt.lfoRate <= 0 {
		return 0
	}
	depth := rt.lfoDepth
	if rt.mpChange > 0 && tick > rt.mpDelay {
		progress := clampInt(tick-rt.mpDelay, 0, rt.mpChange)
		depth = rt.modPitch + ((rt.mpEnd-rt.modPitch)*progress)/rt.mpChange
	}
	// Continuous LFO waveforms. Period = lfoRate * 2 ticks.
	period := rt.lfoRate * 2
	phase := float64(tick%period) / float64(period) // 0..1
	var waveVal float64
	switch rt.lfoWave {
	case 0: // saw
		waveVal = 1.0 - 2.0*phase
	case 1: // square
		if phase < 0.5 {
			waveVal = 1.0
		} else {
			waveVal = -1.0
		}
	case 3: // random (sample-and-hold per cycle)
		// Approximate with a hash-based deterministic random per cycle.
		cycle := tick / period
		waveVal = float64((cycle*16807+1)%127)/63.0 - 1.0
	default: // 2 = triangle (default)
		if phase < 0.5 {
			waveVal = 4.0*phase - 1.0
		} else {
			waveVal = 3.0 - 4.0*phase
		}
	}
	return int(waveVal * float64(depth) / 8.0)
}

func (s *Sequencer) applyAmpControls(rt *runtimeState, vel int, tick int) int {
	if vel <= 0 {
		vel = 1
	}
	filterCut := rt.filterCut
	if rt.filterEnvOn {
		filterCut = rt.filterEnv.step()
	}
	vel = (vel * clampInt(filterCut, 1, 128)) / 128
	if vel < 1 {
		vel = 1
	}
	ampTable := s.sampleTable(rt, "na", 1, tick)
	if ampTable != 0 {
		vel = (vel * clampInt(ampTable, 1, 128)) / 128
		if vel < 1 {
			vel = 1
		}
	}
	if rt.modAmp > 0 {
		ampDepth := rt.modAmp
		if rt.maChange > 0 && tick > rt.maDelay {
			progress := clampInt(tick-rt.maDelay, 0, rt.maChange)
			ampDepth = rt.modAmp + ((rt.maEnd-rt.modAmp)*progress)/rt.maChange
		}
		waveVal := 0.0
		if rt.lfoRate > 0 {
			period := rt.lfoRate * 2
			phase := float64(tick%period) / float64(period)
			switch rt.lfoWave {
			case 0: // saw
				waveVal = 1.0 - 2.0*phase
			case 1: // square
				if phase < 0.5 {
					waveVal = 1.0
				} else {
					waveVal = -1.0
				}
			case 3: // random
				cycle := tick / period
				waveVal = float64((cycle*16807+1)%127)/63.0 - 1.0
			default: // triangle
				if phase < 0.5 {
					waveVal = 4.0*phase - 1.0
				} else {
					waveVal = 3.0 - 4.0*phase
				}
			}
		}
		vel += int(waveVal * float64(ampDepth) / 16.0)
	}
	return clampInt(vel, 1, 127)
}

func parseTableDefinitions(defs map[string]string) map[int]tableData {
	out := map[int]tableData{}
	for k, raw := range defs {
		if !strings.HasPrefix(strings.ToUpper(k), "TABLE") {
			continue
		}
		idRaw := strings.TrimPrefix(strings.ToUpper(k), "TABLE")
		id, err := strconv.Atoi(strings.TrimSpace(idRaw))
		if err != nil {
			continue
		}
		open := strings.IndexByte(raw, '{')
		closeBrace := strings.IndexByte(raw, '}')
		if open < 0 || closeBrace <= open {
			continue
		}
		body := raw[open+1 : closeBrace]
		// Parse trailing stretch/magnify/offset after '}'.
		trailing := raw[closeBrace+1:]
		stretch, magnify, offset := parseTrailingOps(trailing)
		// Split on '|' for loop point.
		loopStart := -1
		pipeIdx := strings.IndexByte(body, '|')
		if pipeIdx >= 0 {
			before := parseTableFormula(body[:pipeIdx])
			after := parseTableFormula(body[pipeIdx+1:])
			loopStart = len(before)
			values := append(before, after...)
			values = applyTableOps(values, stretch, magnify, offset)
			if loopStart > 0 {
				loopStart *= maxInt(stretch, 1)
			}
			if len(values) > 0 {
				out[id] = tableData{values: values, loopStart: loopStart}
			}
		} else {
			values := parseTableFormula(body)
			values = applyTableOps(values, stretch, magnify, offset)
			if len(values) > 0 {
				out[id] = tableData{values: values, loopStart: -1}
			}
		}
	}
	return out
}

func parseTrailingOps(s string) (stretch, magnify, offset int) {
	stretch = 1
	magnify = 1
	offset = 0
	s = strings.TrimSpace(s)
	i := 0
	// First number is stretch factor.
	if i < len(s) && s[i] >= '0' && s[i] <= '9' {
		v, ni, ok := parseSignedAt(s, i)
		if ok && v > 0 {
			stretch = v
			i = ni
		}
	}
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		switch s[i] {
		case '*':
			v, ni, ok := parseSignedAt(s, i+1)
			if ok {
				magnify = v
				i = ni
			} else {
				i++
			}
		case '+':
			v, ni, ok := parseSignedAt(s, i+1)
			if ok {
				offset += v
				i = ni
			} else {
				i++
			}
		case '-':
			v, ni, ok := parseSignedAt(s, i+1)
			if ok {
				offset -= v
				i = ni
			} else {
				i++
			}
		default:
			i++
		}
	}
	return
}

func applyTableOps(values []int, stretch, magnify, offset int) []int {
	if stretch > 1 {
		stretched := make([]int, 0, len(values)*stretch)
		for _, v := range values {
			for j := 0; j < stretch; j++ {
				stretched = append(stretched, v)
			}
		}
		values = stretched
	}
	if magnify != 1 || offset != 0 {
		for i := range values {
			values[i] = values[i]*magnify + offset
		}
	}
	return values
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseTableFormula(body string) []int {
	values := make([]int, 0, 32)
	i := 0
	for i < len(body) {
		for i < len(body) && (body[i] == ',' || body[i] == ' ' || body[i] == '\t' || body[i] == '\n' || body[i] == '\r') {
			i++
		}
		if i >= len(body) {
			break
		}
		switch body[i] {
		case '[':
			end := strings.IndexByte(body[i+1:], ']')
			if end < 0 {
				i++
				continue
			}
			block := body[i+1 : i+1+end]
			i = i + 1 + end + 1
			repeat, ni := parseTrailingNumber(body, i, 1)
			i = ni
			part := parseTableFormula(block)
			for r := 0; r < repeat; r++ {
				values = append(values, part...)
			}
		case '(':
			end := strings.IndexByte(body[i+1:], ')')
			if end < 0 {
				i++
				continue
			}
			inside := strings.TrimSpace(body[i+1 : i+1+end])
			i = i + 1 + end + 1
			repeat, ni := parseTrailingNumber(body, i, 1)
			i = ni
			pts := parseCSV(inside)
			if len(pts) == 1 {
				for r := 0; r < repeat; r++ {
					values = append(values, pts[0])
				}
			} else if len(pts) >= 2 {
				for seg := 0; seg < len(pts)-1; seg++ {
					a, b := pts[seg], pts[seg+1]
					for r := 0; r < repeat; r++ {
						v := a + ((b-a)*r)/repeat
						values = append(values, v)
					}
				}
			}
		case '*', '+', '-':
			if len(values) == 0 {
				i++
				continue
			}
			op := body[i]
			n, ni, ok := parseSignedAt(body, i+1)
			if !ok {
				i++
				continue
			}
			i = ni
			for j := range values {
				switch op {
				case '*':
					values[j] *= n
				case '+':
					values[j] += n
				case '-':
					values[j] -= n
				}
			}
		default:
			v, ni, ok := parseSignedAt(body, i)
			if !ok {
				i++
				continue
			}
			values = append(values, v)
			i = ni
		}
	}
	return values
}

func parseTrailingNumber(src string, at int, def int) (int, int) {
	v, next, ok := parseSignedAt(src, at)
	if !ok {
		return def, at
	}
	if v <= 0 {
		v = def
	}
	return v, next
}

func parseSignedAt(src string, at int) (int, int, bool) {
	i := at
	for i < len(src) && (src[i] == ' ' || src[i] == '\t') {
		i++
	}
	if i >= len(src) {
		return 0, at, false
	}
	sign := 1
	if src[i] == '+' {
		i++
	} else if src[i] == '-' {
		sign = -1
		i++
	}
	start := i
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
	}
	if start == i {
		return 0, at, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(src[start:i]))
	if err != nil {
		return 0, at, false
	}
	return sign * v, i, true
}

func parseCSVInts(src string, fn func(idx int, v int)) {
	if fn == nil {
		return
	}
	src = strings.TrimSpace(src)
	if src == "" {
		return
	}
	parts := strings.Split(src, ",")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		fn(i, v)
	}
}

func parseCSV(src string) []int {
	out := []int{}
	parseCSVInts(src, func(_ int, v int) {
		out = append(out, v)
	})
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func parsePatchMods(defs map[string]string) map[int]patchMod {
	mods := map[int]patchMod{}
	for key, val := range defs {
		if !strings.HasPrefix(key, "OPM@") && !strings.HasPrefix(key, "OPL@") &&
			!strings.HasPrefix(key, "OPN@") && !strings.HasPrefix(key, "OPX@") {
			continue
		}
		numStr := key[strings.IndexByte(key, '@')+1:]
		prog, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		// Find text after closing brace
		closeIdx := strings.LastIndexByte(val, '}')
		if closeIdx < 0 {
			continue
		}
		suffix := val[closeIdx+1:]
		if strings.TrimSpace(suffix) == "" {
			continue
		}
		var pm patchMod
		// Parse mp, ma, mf commands from suffix
		for _, cmd := range []struct {
			prefix string
			dest   *[]int
		}{
			{"mp", &pm.mpArgs},
			{"ma", &pm.maArgs},
			{"mf", &pm.mfArgs},
		} {
			idx := strings.Index(suffix, cmd.prefix)
			if idx < 0 {
				continue
			}
			rest := suffix[idx+len(cmd.prefix):]
			// Find end of this command (next command or end of string)
			end := len(rest)
			for _, other := range []string{"mp", "ma", "mf"} {
				if other == cmd.prefix {
					continue
				}
				if j := strings.Index(rest, other); j >= 0 && j < end {
					end = j
				}
			}
			argStr := strings.TrimRight(strings.TrimSpace(rest[:end]), ";")
			args := parseCSV(argStr)
			*cmd.dest = args
		}
		if pm.mpArgs != nil || pm.maArgs != nil || pm.mfArgs != nil {
			mods[prog] = pm
		}
	}
	return mods
}

func applyScaledVelocity(volume int, expression int, fineVolume int, vScaleMode int, vScaleMax int, xScaleMode int) int {
	volMax := vScaleMax
	if volMax <= 0 {
		volMax = 16
	}
	v := clampInt(volume, 0, 127)
	x := clampInt(expression, 0, 128)
	fv := clampInt(fineVolume, 0, 128)
	vn := float64(v) / float64(volMax)
	if vn < 0 {
		vn = 0
	}
	if vn > 1 {
		vn = 1
	}
	switch vScaleMode {
	case 1:
		vn = dbScale(vn, 96)
	case 2:
		vn = dbScale(vn, 64)
	case 3:
		vn = dbScale(vn, 48)
	case 4:
		vn = dbScale(vn, 32)
	}
	xn := float64(x) / 128.0
	switch xScaleMode {
	case 1:
		xn = math.Sqrt(xn)
	case 2:
		xn = xn * xn
	case 3:
		xn = dbScale(xn, 48)
	case 4:
		xn = dbScale(xn, 32)
	}
	out := vn * xn * (float64(fv) / 128.0) * 127.0
	return clampInt(int(math.Round(out)), 0, 127)
}

func dbScale(norm float64, dbRange float64) float64 {
	if norm <= 0 {
		return 0
	}
	if norm >= 1 {
		return 1
	}
	return math.Pow(10, -dbRange*(1-norm)/20)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (s *Sequencer) peekEvent(tc *trackCursor) (mml.Event, int, bool) {
	if tc.index < 0 || tc.index >= len(tc.events) {
		return mml.Event{}, 0, false
	}
	ev := tc.events[tc.index]
	if tc.loopCycle == 0 || tc.loopIndex < 0 || tc.index < tc.loopIndex {
		return ev, ev.Tick, true
	}
	loopLen := tc.endTick - tc.loopTick
	return ev, ev.Tick + tc.loopCycle*loopLen, true
}

func (s *Sequencer) compactNoteOffs() {
	if len(s.noteOffs) == 0 {
		return
	}
	j := 0
	for i := range s.noteOffs {
		if !s.noteOffs[i].fired {
			s.noteOffs[j] = s.noteOffs[i]
			j++
		}
	}
	s.noteOffs = s.noteOffs[:j]
	// Insertion sort: the slice is nearly sorted since new entries are appended
	// with increasing ticks; this avoids sort.Slice overhead each tick.
	for i := 1; i < len(s.noteOffs); i++ {
		key := s.noteOffs[i]
		k := i - 1
		for k >= 0 && s.noteOffs[k].tick > key.tick {
			s.noteOffs[k+1] = s.noteOffs[k]
			k--
		}
		s.noteOffs[k+1] = key
	}
}

// lfoRateToHz converts the tick-based lfoRate to Hz using the current tempo and sample rate.
func (s *Sequencer) lfoRateToHz(lfoRate int) float64 {
	if lfoRate <= 0 || s.ticksPerSamp <= 0 {
		return 0
	}
	// lfoRate is in ticks for a half-period, so full period = lfoRate * 2 ticks.
	// Ticks per second = ticksPerSamp * sampleRate.
	ticksPerSec := s.ticksPerSamp * float64(s.sampleRate)
	period := float64(lfoRate*2) / ticksPerSec
	if period <= 0 {
		return 0
	}
	return 1.0 / period
}

// updateEngineLFO pushes the current MP/MA/MF state to the engine.
func (s *Sequencer) updateEngineLFO(rt *runtimeState) {
	rateHz := s.lfoRateToHz(rt.lfoRate)

	// Pitch LFO (MP): depth is in 1/8 semitone units in the sequencer; convert to semitones.
	if rt.mpEnd != 0 && rt.lfoRate > 0 {
		s.engine.SetPitchLFO(float64(rt.mpEnd)/8.0, rateHz, rt.lfoWave)
	} else {
		s.engine.SetPitchLFO(0, 0, 0)
	}

	// Amp LFO (MA): depth is in 1/16 units; convert to a 0-1 factor.
	if rt.maEnd != 0 && rt.lfoRate > 0 {
		s.engine.SetAmpLFO(float64(rt.maEnd)/16.0, rateHz, rt.lfoWave)
	} else {
		s.engine.SetAmpLFO(0, 0, 0)
	}

	// Filter LFO (MF): depth in cutoff units.
	if rt.mfEnd != 0 && rt.lfoRate > 0 {
		s.engine.SetFilterLFO(float64(rt.mfEnd)/8.0, rateHz, rt.lfoWave)
	} else {
		s.engine.SetFilterLFO(0, 0, 0)
	}
}

func (s *Sequencer) cancelPendingNoteOff(voice int) {
	for i := range s.noteOffs {
		if s.noteOffs[i].voice == voice && !s.noteOffs[i].fired {
			s.noteOffs[i].fired = true
		}
	}
}
