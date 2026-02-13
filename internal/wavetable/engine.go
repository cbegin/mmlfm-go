package wavetable

import (
	"encoding/hex"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/cbegin/mmlfm-go/internal/lfo"
)

const twoPi = math.Pi * 2

const (
	maxSlots  = 16
	maxVoices = 16
)

// Params controls the wavetable engine.
type Params struct {
	Polyphony   int
	AttackSec   float64
	DecaySec    float64
	SustainLvl  float64
	ReleaseSec  float64
	MasterGain  float64
	VelocityAmp float64
	LPFCutoff   float64 // lowpass filter cutoff in Hz (0 = disabled)
}

// DefaultParams returns sensible defaults for wavetable synthesis.
func DefaultParams() Params {
	return Params{
		Polyphony:   maxVoices,
		AttackSec:   0.005,
		DecaySec:    0.12,
		SustainLvl:  0.75,
		ReleaseSec:  0.2,
		MasterGain:  0.42,
		VelocityAmp: 0.8,
		LPFCutoff:   12000,
	}
}

type filterType int

const (
	filterLP filterType = iota
	filterHP
	filterBP
)

type envState int

const (
	envAttack envState = iota
	envDecay
	envSustain
	envRelease
	envOff
)

type voice struct {
	active           bool
	id               int
	velocity         float64
	freq             float64
	phase            float64 // current position in the wavetable [0, tableLen)
	env              float64
	envState         envState
	pan              float64
	slot             int // wavetable slot index
	portamentoTarget float64
	portamentoFrames int
	portamentoStep   float64
}

// Engine is a wavetable synthesis engine that implements sequencer.VoiceEngine.
type Engine struct {
	sampleRate       float64
	params           Params
	voices           []voice
	tables           [maxSlots][]float64
	nextID           int
	masterGain       uint64
	nextPhase        int
	portamentoFrom   int
	portamentoFrames int
	lpfL             float64
	lpfR             float64
	bpfL             float64
	bpfR             float64
	lpfAlpha         float64
	baseLPFCutoff    float64
	filterKind       filterType
	pitchLFO         lfo.LFO
	ampLFO           lfo.LFO
	filterLFO        lfo.LFO
}

// New creates a wavetable engine at the given sample rate.
func New(sampleRate int, params Params) *Engine {
	if params.Polyphony <= 0 {
		params.Polyphony = maxVoices
	}
	if params.Polyphony > maxVoices {
		params.Polyphony = maxVoices
	}
	e := &Engine{
		sampleRate: float64(sampleRate),
		params:     params,
		voices:     make([]voice, params.Polyphony),
		masterGain: math.Float64bits(params.MasterGain),
	}
	if params.LPFCutoff > 0 && params.LPFCutoff < float64(sampleRate)/2 {
		rc := 1.0 / (twoPi * params.LPFCutoff)
		dt := 1.0 / float64(sampleRate)
		e.lpfAlpha = dt / (rc + dt)
		e.baseLPFCutoff = params.LPFCutoff
	}
	// Install a default sine wavetable in slot 0.
	sine := make([]float64, 64)
	for i := range sine {
		sine[i] = math.Sin(twoPi * float64(i) / float64(len(sine)))
	}
	e.tables[0] = sine
	return e
}

// SetWavetable loads a single-cycle waveform into the given slot (0-15).
// samples should be 32-256 values representing one cycle.
func (e *Engine) SetWavetable(slot int, samples []float64) {
	if slot < 0 || slot >= maxSlots {
		return
	}
	cp := make([]float64, len(samples))
	copy(cp, samples)
	e.tables[slot] = cp
}

// NoteOn starts a voice. The low byte of program selects the wavetable slot.
func (e *Engine) NoteOn(note int, velocity int, pan int, encodedProgram int) int {
	slot := e.stealVoice()
	id := e.nextID
	e.nextID++

	program, _, _ := decodeProgram(encodedProgram)
	tableSlot := program & 0x0F
	if tableSlot >= maxSlots || len(e.tables[tableSlot]) == 0 {
		tableSlot = 0
	}

	p := clamp(float64(pan), -64, 64)
	v := &e.voices[slot]
	targetFreq := midiToFreq(note)
	freq := targetFreq

	var portTgt float64
	var portFrames int
	var portStep float64
	if e.portamentoFrom >= 0 && e.portamentoFrames > 0 {
		freq = midiToFreq(e.portamentoFrom)
		portTgt = targetFreq
		portFrames = e.portamentoFrames
		portStep = (targetFreq - freq) / float64(e.portamentoFrames)
	}
	e.portamentoFrom = -1
	e.portamentoFrames = 0

	var phase float64
	switch e.nextPhase {
	case -1:
		phase = rand.Float64() * float64(len(e.tables[tableSlot]))
	case 0:
		phase = 0
	default:
		tableLen := float64(len(e.tables[tableSlot]))
		phase = math.Mod(float64(e.nextPhase)/128.0*tableLen/2.0, tableLen)
	}
	e.nextPhase = 0

	*v = voice{
		active:           true,
		id:               id,
		velocity:         clamp(float64(velocity)/127.0, 0, 1),
		freq:             freq,
		phase:            phase,
		env:              0,
		envState:         envAttack,
		pan:              p,
		slot:             tableSlot,
		portamentoTarget: portTgt,
		portamentoFrames: portFrames,
		portamentoStep:   portStep,
	}
	return id
}

// NoteOff releases a voice by id.
func (e *Engine) NoteOff(id int) {
	for i := range e.voices {
		v := &e.voices[i]
		if v.active && v.id == id && v.envState != envRelease {
			v.envState = envRelease
		}
	}
}

// RenderFrame produces one stereo sample pair.
func (e *Engine) RenderFrame() (float32, float32) {
	pitchMod := e.pitchLFO.Sample(e.sampleRate)
	ampMod := e.ampLFO.Sample(e.sampleRate)
	filterMod := e.filterLFO.Sample(e.sampleRate)

	freqMul := 1.0
	if pitchMod != 0 {
		freqMul = math.Pow(2, pitchMod/12.0)
	}

	var l, r float64
	for i := range e.voices {
		v := &e.voices[i]
		if !v.active {
			continue
		}

		env := e.advanceEnv(v)
		if !v.active {
			continue
		}

		table := e.tables[v.slot]
		if len(table) == 0 {
			continue
		}
		tableLen := float64(len(table))

		// Linear interpolation between adjacent samples.
		idx := math.Floor(v.phase)
		frac := v.phase - idx
		i0 := int(idx) % len(table)
		if i0 < 0 {
			i0 += len(table)
		}
		i1 := (i0 + 1) % len(table)
		sig := table[i0]*(1-frac) + table[i1]*frac

		sig *= env * e.masterGainValue() * (0.2 + v.velocity*e.params.VelocityAmp)
		// Apply amp LFO
		sig *= (1.0 + ampMod)

		// Equal-power stereo panning.
		angle := ((v.pan + 64.0) / 128.0) * (math.Pi / 2.0)
		leftGain := math.Cos(angle)
		rightGain := math.Sin(angle)
		l += sig * leftGain
		r += sig * rightGain

		// Portamento.
		if v.portamentoFrames > 0 {
			v.portamentoFrames--
			v.freq += v.portamentoStep
			if v.portamentoFrames <= 0 {
				v.freq = v.portamentoTarget
			}
		}

		// Advance phase with pitch LFO modulation
		v.phase += v.freq * freqMul * tableLen / e.sampleRate
		for v.phase >= tableLen {
			v.phase -= tableLen
		}
		for v.phase < 0 {
			v.phase += tableLen
		}
	}

	// Filter LFO
	if e.baseLPFCutoff > 0 && filterMod != 0 {
		cutoff := e.baseLPFCutoff + filterMod*100.0
		if cutoff < 20 {
			cutoff = 20
		}
		if cutoff > e.sampleRate/2 {
			cutoff = e.sampleRate / 2
		}
		rc := 1.0 / (twoPi * cutoff)
		dt := 1.0 / e.sampleRate
		e.lpfAlpha = dt / (rc + dt)
	}

	// Filter.
	if e.lpfAlpha > 0 {
		e.lpfL += e.lpfAlpha * (l - e.lpfL)
		e.lpfR += e.lpfAlpha * (r - e.lpfR)
		switch e.filterKind {
		case filterLP:
			l = e.lpfL
			r = e.lpfR
		case filterHP:
			l = l - e.lpfL
			r = r - e.lpfR
		case filterBP:
			e.bpfL += e.lpfAlpha * (e.lpfL - e.bpfL)
			e.bpfR += e.lpfAlpha * (e.lpfR - e.bpfR)
			l = e.lpfL - e.bpfL
			r = e.lpfR - e.bpfR
		}
	}

	return float32(clamp(l, -1, 1)), float32(clamp(r, -1, 1))
}

// SetMasterGain sets the master gain atomically.
func (e *Engine) SetMasterGain(gain float64) {
	if gain < 0 {
		gain = 0
	}
	atomic.StoreUint64(&e.masterGain, math.Float64bits(gain))
}

// ActiveVoiceCount returns the number of currently active voices.
func (e *Engine) ActiveVoiceCount() int {
	n := 0
	for i := range e.voices {
		if e.voices[i].active {
			n++
		}
	}
	return n
}

// SetFilterType sets the output filter mode: 0=LP, 1=BP, 2=HP.
func (e *Engine) SetFilterType(ft int) {
	switch ft {
	case 1:
		e.filterKind = filterBP
	case 2:
		e.filterKind = filterHP
	default:
		e.filterKind = filterLP
	}
}

// SetNoteOnPhase sets the phase for the next NoteOn: 0=reset, -1=random, 1-255=fixed.
func (e *Engine) SetNoteOnPhase(phase int) {
	e.nextPhase = phase
}

// SetPortamento sets glide parameters for the next NoteOn.
func (e *Engine) SetPortamento(fromNote int, frames int) {
	e.portamentoFrom = fromNote
	e.portamentoFrames = frames
}

func (e *Engine) SetPitchLFO(depth float64, rateHz float64, waveform int) {
	e.pitchLFO.Set(depth, rateHz, waveform)
}

func (e *Engine) SetAmpLFO(depth float64, rateHz float64, waveform int) {
	e.ampLFO.Set(depth, rateHz, waveform)
}

func (e *Engine) SetFilterLFO(depth float64, rateHz float64, waveform int) {
	e.filterLFO.Set(depth, rateHz, waveform)
}

// LoadWAVBFromDefs loads #WAVB definitions from parsed score definitions into
// wavetable slots. Keys like "WAVB0" map to slot 0, etc.
func (e *Engine) LoadWAVBFromDefs(defs map[string]string) {
	if defs == nil {
		return
	}
	for key, body := range defs {
		upper := strings.ToUpper(key)
		if !strings.HasPrefix(upper, "WAVB") {
			continue
		}
		slotStr := strings.TrimSpace(upper[4:])
		slot, err := strconv.Atoi(slotStr)
		if err != nil || slot < 0 || slot >= maxSlots {
			continue
		}
		// Extract hex data from between { and }.
		open := strings.IndexByte(body, '{')
		close := strings.IndexByte(body, '}')
		if open < 0 || close <= open {
			continue
		}
		hexData := strings.TrimSpace(body[open+1 : close])
		samples := ParseWAVB(hexData)
		if len(samples) > 0 {
			e.SetWavetable(slot, samples)
		}
	}
}

// ParseWAVB converts a hex string (pairs of hex digits representing signed 8-bit
// values) into a slice of float64 samples normalized to the range [-1, 1].
func ParseWAVB(h string) []float64 {
	data, err := hex.DecodeString(h)
	if err != nil {
		return nil
	}
	out := make([]float64, len(data))
	for i, b := range data {
		out[i] = float64(int8(b)) / 127.0
	}
	return out
}

// --- internal helpers ---

func (e *Engine) masterGainValue() float64 {
	return math.Float64frombits(atomic.LoadUint64(&e.masterGain))
}

func (e *Engine) stealVoice() int {
	for i := range e.voices {
		if !e.voices[i].active {
			return i
		}
	}
	quiet := 0
	minEnv := e.voices[0].env
	for i := 1; i < len(e.voices); i++ {
		if e.voices[i].env < minEnv {
			minEnv = e.voices[i].env
			quiet = i
		}
	}
	return quiet
}

func (e *Engine) advanceEnv(v *voice) float64 {
	switch v.envState {
	case envAttack:
		step := 1.0 / (e.params.AttackSec * e.sampleRate)
		if step <= 0 {
			step = 1
		}
		v.env += step
		if v.env >= 1 {
			v.env = 1
			v.envState = envDecay
		}
	case envDecay:
		step := (1 - e.params.SustainLvl) / (e.params.DecaySec * e.sampleRate)
		if step <= 0 {
			step = 1
		}
		v.env -= step
		if v.env <= e.params.SustainLvl {
			v.env = e.params.SustainLvl
			v.envState = envSustain
		}
	case envSustain:
		// hold
	case envRelease:
		step := e.params.SustainLvl / (e.params.ReleaseSec * e.sampleRate)
		if step <= 0 {
			step = 1
		}
		v.env -= step
		if v.env <= 0.0001 {
			v.env = 0
			v.envState = envOff
			v.active = false
		}
	case envOff:
		v.active = false
		v.env = 0
	}
	return v.env
}

func midiToFreq(note int) float64 {
	return 440 * math.Pow(2, float64(note-69)/12)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func decodeProgram(encoded int) (program int, module int, channel int) {
	if encoded < 0 {
		encoded = 0
	}
	program = encoded & 0xFF
	module = (encoded >> 8) & 0xFF
	channel = (encoded >> 16) & 0xFF
	return
}
