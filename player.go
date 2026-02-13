package mmlfm

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	intaudio "github.com/cbegin/mmlfm-go/internal/audio"
	intchip "github.com/cbegin/mmlfm-go/internal/chiptune"
	intfx "github.com/cbegin/mmlfm-go/internal/effects"
	intfm "github.com/cbegin/mmlfm-go/internal/fm"
	intmml "github.com/cbegin/mmlfm-go/internal/mml"
	intnes "github.com/cbegin/mmlfm-go/internal/nesapu"
	intseq "github.com/cbegin/mmlfm-go/internal/sequencer"
	intwt "github.com/cbegin/mmlfm-go/internal/wavetable"
)

// PlaybackEvent carries playback and trigger events from Watch().
type PlaybackEvent struct {
	Kind        int // EventLoopCompleted, EventPlaybackEnded, or EventTrigger
	TriggerID   int
	NoteOnType  int
	NoteOffType int
}

const (
	EventLoopCompleted int = iota
	EventPlaybackEnded
	EventTrigger
)

type SynthMode string

const (
	SynthModeFM        SynthMode = "fm"
	SynthModeChiptune  SynthMode = "chiptune"
	SynthModeNESAPU    SynthMode = "nesapu"
	SynthModeWavetable SynthMode = "wavetable"
)

type PlayerOption func(*playerConfig)

type playerConfig struct {
	mode         SynthMode
	loopPlayback bool
	sampleTap    func([]float32)
}

func defaultPlayerConfig() playerConfig {
	return playerConfig{mode: SynthModeFM, loopPlayback: true}
}

func WithSynthMode(mode SynthMode) PlayerOption {
	return func(cfg *playerConfig) {
		cfg.mode = mode
	}
}

func WithLoopPlayback(enabled bool) PlayerOption {
	return func(cfg *playerConfig) {
		cfg.loopPlayback = enabled
	}
}

// WithSampleTap installs a callback invoked with each generated stereo buffer.
// The callback runs on the audio thread; keep work brief and non-blocking.
func WithSampleTap(tap func([]float32)) PlayerOption {
	return func(cfg *playerConfig) {
		cfg.sampleTap = tap
	}
}

type Player struct {
	mu           sync.Mutex
	parser       *intmml.Parser
	sampleRate   int
	mode         SynthMode
	engine       intseq.VoiceEngine
	audio        *intaudio.Player
	baseGain     float64
	volume       float64
	transpose    int
	loopPlayback bool
	sampleTap    func([]float32)
	masterEQ     *intfx.EQ5Band
	done         chan struct{}
	eventCh      chan PlaybackEvent
	eventChMu    sync.Mutex
}

// eventWrapper wraps a sequencer and implements SampleSource + FinishingSource
// to report playback events and signal when non-looping playback ends.
type eventWrapper struct {
	seq       *intseq.Sequencer
	finished  atomic.Bool
	onEvent   func(intseq.EventKind)
	onTrigger func(intseq.TriggerEvent)
	effects   *intfx.Chain
	masterEQ  *intfx.EQ5Band
	sampleTap func([]float32)
}

func (w *eventWrapper) Process(dst []float32) {
	w.seq.Process(dst)
	if w.effects != nil {
		for i := 0; i+1 < len(dst); i += 2 {
			dst[i], dst[i+1] = w.effects.Process(dst[i], dst[i+1])
		}
	}
	if w.masterEQ != nil {
		for i := 0; i+1 < len(dst); i += 2 {
			dst[i], dst[i+1] = w.masterEQ.Process(dst[i], dst[i+1])
		}
	}
	if w.sampleTap != nil {
		w.sampleTap(dst)
	}
}

func (w *eventWrapper) Finished() bool {
	return w.finished.Load()
}

func NewPlayer(sampleRate int, opts ...PlayerOption) (*Player, error) {
	if sampleRate <= 0 {
		return nil, errors.New("sampleRate must be positive")
	}
	cfg := defaultPlayerConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	engine, baseGain, err := newEngineForMode(cfg.mode, sampleRate)
	if err != nil {
		return nil, err
	}
	engine.SetMasterGain(baseGain)
	return &Player{
		parser:       intmml.NewParser(intmml.DefaultParserConfig()),
		sampleRate:   sampleRate,
		mode:         cfg.mode,
		engine:       engine,
		baseGain:     baseGain,
		volume:       1,
		loopPlayback: cfg.loopPlayback,
		sampleTap:    cfg.sampleTap,
		masterEQ:     intfx.NewEQ5Band(sampleRate),
	}, nil
}

func Compile(mmlText string) (*intmml.Score, error) {
	return intmml.NewParser(intmml.DefaultParserConfig()).Parse(mmlText)
}

func (p *Player) PlayMML(mmlText string) error {
	score, err := p.parser.Parse(mmlText)
	if err != nil {
		return err
	}
	return p.Play(score)
}

func (p *Player) Play(score *intmml.Score) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Signal any existing Wait() that the previous playback was replaced
	if p.done != nil {
		close(p.done)
	}
	p.done = make(chan struct{})

	wrapper := &eventWrapper{}
	wrapper.onEvent = func(kind intseq.EventKind) {
		if kind == intseq.EventPlaybackEnded {
			wrapper.finished.Store(true)
		}
		p.sendEvent(PlaybackEvent{Kind: int(kind)})
		if kind == intseq.EventPlaybackEnded {
			p.signalDone()
		}
	}
	wrapper.onTrigger = func(te intseq.TriggerEvent) {
		p.sendEvent(PlaybackEvent{Kind: EventTrigger, TriggerID: te.TriggerID, NoteOnType: te.NoteOnType, NoteOffType: te.NoteOffType})
	}

	// Recreate the base engine on every Play to avoid voice/envelope state
	// leaking between songs.
	baseEngine, baseGain, err := newEngineForMode(p.mode, p.sampleRate)
	if err != nil {
		return err
	}
	baseEngine.SetMasterGain(baseGain * p.volume)
	p.engine = baseEngine
	p.baseGain = baseGain

	engine := baseEngine
	usedMods := scoreUsedModules(score)
	if len(usedMods) > 1 {
		multi := intseq.NewMultiEngine(0, p.sampleRate)
		defaultEng, defaultGain := baseEngine, baseGain
		multi.AddEngine(0, defaultEng)
		for mod := range usedMods {
			if mod == 0 {
				continue
			}
			e, g := engineForModule(mod, p.sampleRate, defaultEng, defaultGain)
			e.SetMasterGain(g)
			multi.AddEngine(mod, e)
		}
		for _, e := range multi.AllEngines() {
			if fmEng, ok := e.(*intfm.Engine); ok && score.Definitions != nil {
				fmEng.LoadOPMPatchFromDefs(score.Definitions)
			}
			if wtEng, ok := e.(*intwt.Engine); ok && score.Definitions != nil {
				wtEng.LoadWAVBFromDefs(score.Definitions)
			}
		}
		engine = multi
	} else {
		if fmEng, ok := baseEngine.(*intfm.Engine); ok && score.Definitions != nil {
			fmEng.LoadOPMPatchFromDefs(score.Definitions)
		}
		if wtEng, ok := baseEngine.(*intwt.Engine); ok && score.Definitions != nil {
			wtEng.LoadWAVBFromDefs(score.Definitions)
		}
	}

	seq := intseq.NewWithOptions(score, engine, p.sampleRate, intseq.Options{
		LoopWholeScore:  p.loopPlayback,
		OnEvent:         wrapper.onEvent,
		OnTrigger:       wrapper.onTrigger,
		MasterTranspose: p.transpose,
	})
	wrapper.seq = seq
	wrapper.effects = buildEffectChain(score.Definitions, p.sampleRate)
	wrapper.masterEQ = p.masterEQ
	wrapper.sampleTap = p.sampleTap

	backend, err := intaudio.NewPlayer(p.sampleRate, wrapper)
	if err != nil {
		return err
	}
	if p.audio != nil {
		_ = p.audio.Stop()
	}
	p.audio = backend
	p.audio.Play()
	return nil
}

func newEngineForMode(mode SynthMode, sampleRate int) (intseq.VoiceEngine, float64, error) {
	switch mode {
	case SynthModeFM:
		params := intfm.DefaultParams()
		return intfm.New(sampleRate, params), params.MasterGain, nil
	case SynthModeChiptune:
		params := intchip.DefaultParams()
		return intchip.New(sampleRate, params), params.MasterGain, nil
	case SynthModeNESAPU:
		params := intnes.DefaultParams()
		return intnes.New(sampleRate, params), params.MasterGain, nil
	case SynthModeWavetable:
		params := intwt.DefaultParams()
		return intwt.New(sampleRate, params), params.MasterGain, nil
	default:
		return nil, 0, errors.New("unknown synth mode")
	}
}

func (p *Player) sendEvent(ev PlaybackEvent) {
	p.eventChMu.Lock()
	ch := p.eventCh
	p.eventChMu.Unlock()
	if ch != nil {
		select {
		case ch <- ev:
		default:
			// Channel full or closed; drop event
		}
	}
}

func (p *Player) signalDone() {
	p.mu.Lock()
	done := p.done
	p.done = nil
	p.mu.Unlock()
	if done != nil {
		close(done)
	}
}

func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.audio != nil {
		p.audio.Pause()
	}
}

func (p *Player) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.audio != nil {
		p.audio.Play()
	}
}

func (p *Player) Stop() error {
	p.mu.Lock()
	if p.audio == nil {
		p.mu.Unlock()
		return nil
	}
	err := p.audio.Stop()
	p.audio = nil
	done := p.done
	p.done = nil
	p.mu.Unlock()
	p.sendEvent(PlaybackEvent{Kind: EventPlaybackEnded})
	if done != nil {
		close(done)
	}
	return err
}

// Wait blocks until the current playback ends. When loop playback is enabled,
// Wait blocks indefinitely (use Watch for loop-counting instead).
// Wait returns immediately if no playback is active or if it was stopped.
func (p *Player) Wait() {
	p.mu.Lock()
	done := p.done
	p.mu.Unlock()
	if done != nil {
		<-done
	}
}

// Watch returns a channel that receives playback events. Events are sent when:
//   - EventLoopCompleted: a whole-score loop iteration finished (when looping)
//   - EventPlaybackEnded: playback finished (when not looping)
//   - EventTrigger: %t or %e command fired (TriggerID, NoteOnType, NoteOffType set)
//
// The channel is buffered (cap 8); receive in a goroutine to avoid blocking the sequencer.
// Only the most recent Watch() channel receives events; call Watch before Play.
func (p *Player) Watch() <-chan PlaybackEvent {
	ch := make(chan PlaybackEvent, 8)
	p.eventChMu.Lock()
	p.eventCh = ch
	p.eventChMu.Unlock()
	return ch
}

// SetMasterVolume sets runtime volume scalar. 1.0 is default.
func (p *Player) SetMasterVolume(volume float64) {
	if volume < 0 {
		volume = 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = volume
	p.engine.SetMasterGain(p.baseGain * p.volume)
}

func (p *Player) MasterVolume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// SetTranspose sets the master octave shift applied to all notes.
// Positive values shift up, negative shift down (e.g. -2, -1, 0, +1, +2).
// Takes effect on the next Play/PlayMML call.
func (p *Player) SetTranspose(octaves int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.transpose = octaves
}

// Transpose returns the current master octave shift.
func (p *Player) Transpose() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.transpose
}

// SetEQBand sets the gain for a master EQ band (0-4). 1.0 = unity.
// Band frequencies: 0=<200Hz, 1=200-800Hz, 2=800-2.5kHz, 3=2.5-8kHz, 4=>8kHz.
// This takes effect immediately on the audio thread (lock-free).
func (p *Player) SetEQBand(band int, gain float32) {
	p.masterEQ.SetGain(band, gain)
}

// EQBand returns the current gain for a master EQ band (0-4).
func (p *Player) EQBand(band int) float32 {
	return p.masterEQ.Gain(band)
}

// PlaybackPosition returns the current output position of the audio driver,
// i.e. what the listener actually hears right now. Returns 0 if not playing.
func (p *Player) PlaybackPosition() int64 {
	p.mu.Lock()
	a := p.audio
	p.mu.Unlock()
	if a == nil {
		return 0
	}
	pos := a.Position()
	return int64(pos.Seconds() * float64(p.sampleRate))
}

// buildEffectChain parses #EFFECT directives from score definitions and builds
// an effect chain. Supports: delay, reverb, chorus, distortion, eq, compressor.
// Format: #EFFECT0{type param1,param2,...}
func buildEffectChain(defs map[string]string, sampleRate int) *intfx.Chain {
	chain := intfx.NewChain()
	added := false
	// Check EFFECT0 through EFFECT7
	for i := 0; i < 8; i++ {
		key := "EFFECT" + strconv.Itoa(i)
		raw, ok := defs[key]
		if !ok {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		// Strip braces if present
		if len(raw) > 0 && raw[0] == '{' {
			raw = raw[1:]
		}
		if len(raw) > 0 && raw[len(raw)-1] == '}' {
			raw = raw[:len(raw)-1]
		}
		raw = strings.TrimSpace(raw)
		// Split into type and params
		parts := strings.SplitN(raw, " ", 2)
		effectType := strings.ToLower(strings.TrimSpace(parts[0]))
		var params []float64
		if len(parts) > 1 {
			for _, p := range strings.Split(parts[1], ",") {
				p = strings.TrimSpace(p)
				if v, err := strconv.ParseFloat(p, 64); err == nil {
					params = append(params, v)
				}
			}
		}
		if eff := createEffect(effectType, params, sampleRate); eff != nil {
			chain.Add(eff)
			added = true
		}
	}
	if !added {
		return nil
	}
	return chain
}

func scoreUsedModules(score *intmml.Score) map[int]struct{} {
	mods := map[int]struct{}{0: {}}
	for _, tr := range score.Tracks {
		for _, ev := range tr.Events {
			if ev.Type == intmml.EventModule {
				mods[ev.Module] = struct{}{}
			}
		}
	}
	return mods
}

func engineForModule(module int, sampleRate int, defaultEng intseq.VoiceEngine, defaultGain float64) (intseq.VoiceEngine, float64) {
	switch module {
	case 1, 8:
		params := intchip.DefaultParams()
		return intchip.New(sampleRate, params), params.MasterGain
	case 4:
		params := intwt.DefaultParams()
		return intwt.New(sampleRate, params), params.MasterGain
	case 6:
		params := intfm.DefaultParams()
		return intfm.New(sampleRate, params), params.MasterGain
	case 0:
		return defaultEng, defaultGain
	default:
		return defaultEng, defaultGain
	}
}

func createEffect(effectType string, params []float64, sampleRate int) intfx.Effector {
	getParam := func(idx int, def float64) float64 {
		if idx < len(params) {
			return params[idx]
		}
		return def
	}
	switch effectType {
	case "delay":
		return intfx.NewDelay(sampleRate,
			getParam(0, 250),          // delay ms
			float32(getParam(1, 0.4)), // feedback
			float32(getParam(2, 0.2)), // cross
			float32(getParam(3, 0.3)), // wet
		)
	case "reverb":
		return intfx.NewReverb(sampleRate,
			float32(getParam(0, 0.5)),  // room size
			float32(getParam(1, 0.7)),  // feedback
			float32(getParam(2, 0.25)), // wet
		)
	case "chorus":
		return intfx.NewChorus(sampleRate,
			float32(getParam(0, 15)),  // delay ms
			float32(getParam(1, 0.3)), // feedback
			float32(getParam(2, 3)),   // depth ms
			float32(getParam(3, 1.5)), // rate Hz
			float32(getParam(4, 0.4)), // wet
		)
	case "dist", "distortion":
		return intfx.NewDistortion(sampleRate,
			float32(getParam(0, 4)),    // pre gain
			float32(getParam(1, 0.5)),  // post gain
			float32(getParam(2, 8000)), // lpf cutoff
		)
	case "eq":
		return intfx.NewEQ3Band(sampleRate,
			float32(getParam(0, 1.0)),  // low gain
			float32(getParam(1, 1.0)),  // mid gain
			float32(getParam(2, 1.0)),  // high gain
			float32(getParam(3, 300)),  // low freq
			float32(getParam(4, 3000)), // high freq
		)
	case "comp", "compressor":
		return intfx.NewCompressor(sampleRate,
			float32(getParam(0, -20)), // threshold dB
			float32(getParam(1, 4)),   // ratio
			float32(getParam(2, 5)),   // attack ms
			float32(getParam(3, 100)), // release ms
			float32(getParam(4, 6)),   // makeup dB
		)
	}
	return nil
}
