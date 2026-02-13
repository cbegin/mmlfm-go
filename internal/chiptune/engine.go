package chiptune

import (
	"math"
	"math/rand"
	"sync/atomic"

	"github.com/cbegin/mmlfm-go/internal/lfo"
)

const twoPi = math.Pi * 2

type Params struct {
	Voices      int
	MasterGain  float64
	AttackSec   float64
	DecaySec    float64
	SustainLvl  float64
	ReleaseSec  float64
	StepLevels  int
	PulseDutyA  float64
	PulseDutyB  float64
	VelocityAmp float64
	LPFCutoff   float64 // lowpass filter cutoff in Hz (0 = disabled)
}

func DefaultParams() Params {
	return Params{
		Voices:      12,
		MasterGain:  0.28,
		AttackSec:   0.005,
		DecaySec:    0.15,
		SustainLvl:  0.65,
		ReleaseSec:  0.20,
		StepLevels:  16,
		PulseDutyA:  0.125,
		PulseDutyB:  0.25,
		VelocityAmp: 0.85,
		LPFCutoff:   12000,
	}
}

type waveType int

const (
	wavePulseA waveType = iota
	wavePulseB
	waveTriangle
	waveNoise
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
	age              int
	wave             waveType
	freq             float64
	phase            float64
	velocity         float64
	env              float64
	envState         envState
	pan              float64
	noiseLFSR        uint16
	portamentoTarget float64
	portamentoFrames int
	portamentoStep   float64
}

type filterType int

const (
	filterLP filterType = iota
	filterHP
	filterBP
)

type Engine struct {
	sampleRate      float64
	params          Params
	voices          []voice
	nextID          int
	masterGain      uint64
	dcPrevInL       float64
	dcPrevOutL      float64
	dcPrevInR       float64
	dcPrevOutR      float64
	lpfL            float64 // lowpass filter state
	lpfR            float64
	bpfL            float64 // bandpass stage
	bpfR            float64
	lpfAlpha        float64 // filter coefficient
	baseLPFCutoff   float64
	filterKind      filterType
	nextPhase       int
	portamentoFrom  int
	portamentoFrames int
	pitchLFO        lfo.LFO
	ampLFO          lfo.LFO
	filterLFO       lfo.LFO
}

func New(sampleRate int, params Params) *Engine {
	if params.Voices <= 0 {
		params.Voices = 12
	}
	if params.StepLevels <= 1 {
		params.StepLevels = 16
	}
	e := &Engine{
		sampleRate: float64(sampleRate),
		params:     params,
		voices:     make([]voice, params.Voices),
		masterGain: math.Float64bits(params.MasterGain),
	}
	for i := range e.voices {
		e.voices[i].noiseLFSR = uint16(0xACE1 + i*97)
	}
	if params.LPFCutoff > 0 && params.LPFCutoff < float64(sampleRate)/2 {
		rc := 1.0 / (twoPi * params.LPFCutoff)
		dt := 1.0 / float64(sampleRate)
		e.lpfAlpha = dt / (rc + dt)
		e.baseLPFCutoff = params.LPFCutoff
	}
	return e
}

func (e *Engine) NoteOn(note int, velocity int, pan int, encodedProgram int) int {
	slot := e.stealVoice()
	id := e.nextID
	e.nextID++
	program, module, channel := decodeProgram(encodedProgram)
	v := &e.voices[slot]
	v.active = true
	v.id = id
	v.age = 0
	v.wave = waveForProgram(program, module, channel)
	targetFreq := midiToFreq(note)
	if e.portamentoFrom >= 0 && e.portamentoFrames > 0 {
		v.freq = midiToFreq(e.portamentoFrom)
		v.portamentoTarget = targetFreq
		v.portamentoFrames = e.portamentoFrames
		v.portamentoStep = (targetFreq - v.freq) / float64(e.portamentoFrames)
	} else {
		v.freq = targetFreq
		v.portamentoFrames = 0
	}
	e.portamentoFrom = -1
	e.portamentoFrames = 0
	switch e.nextPhase {
	case -1:
		v.phase = rand.Float64()
	case 0:
		v.phase = 0
	default:
		v.phase = math.Mod(float64(e.nextPhase)/128.0*math.Pi, twoPi) / twoPi
	}
	e.nextPhase = 0
	v.velocity = clamp(float64(velocity)/127.0, 0, 1)
	v.env = 0
	v.envState = envAttack
	v.pan = clamp(float64(pan), -64, 64)
	if v.noiseLFSR == 0 {
		v.noiseLFSR = 0xACE1
	}
	return id
}

func (e *Engine) NoteOff(id int) {
	for i := range e.voices {
		v := &e.voices[i]
		if v.active && v.id == id && v.envState != envRelease {
			v.envState = envRelease
		}
	}
}

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
		v.age++
		if v.portamentoFrames > 0 {
			v.portamentoFrames--
			v.freq += v.portamentoStep
			if v.portamentoFrames <= 0 {
				v.freq = v.portamentoTarget
			}
		}
		// Apply pitch LFO to effective frequency for rendering
		origFreq := v.freq
		v.freq *= freqMul
		env := e.advanceEnv(v)
		if !v.active {
			v.freq = origFreq
			continue
		}
		sample := e.renderWave(v)
		v.freq = origFreq
		level := quantize(env*(0.15+v.velocity*e.params.VelocityAmp), e.params.StepLevels)
		sig := sample * level * (1.0 + ampMod)
		angle := ((v.pan + 64.0) / 128.0) * (math.Pi / 2.0)
		l += sig * math.Cos(angle) * e.masterGainValue()
		r += sig * math.Sin(angle) * e.masterGainValue()
	}
	l = e.dcBlockL(l)
	r = e.dcBlockR(r)
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

func (e *Engine) dcBlockL(x float64) float64 {
	const r = 0.995
	y := x - e.dcPrevInL + r*e.dcPrevOutL
	e.dcPrevInL = x
	e.dcPrevOutL = y
	return y
}

func (e *Engine) dcBlockR(x float64) float64 {
	const r = 0.995
	y := x - e.dcPrevInR + r*e.dcPrevOutR
	e.dcPrevInR = x
	e.dcPrevOutR = y
	return y
}

// polyBLEP reduces aliasing at waveform discontinuities.
// t is the phase position [0,1), dt is the phase increment per sample.
func polyBLEP(t, dt float64) float64 {
	if t < dt {
		t /= dt
		return t + t - t*t - 1
	}
	if t > 1-dt {
		t = (t - 1) / dt
		return t*t + t + t + 1
	}
	return 0
}

func (e *Engine) renderWave(v *voice) float64 {
	dt := v.freq / e.sampleRate
	v.phase += dt
	if v.phase >= 1 {
		v.phase -= 1
	}
	switch v.wave {
	case wavePulseA:
		out := -1.0
		if v.phase < e.params.PulseDutyA {
			out = 1
		}
		out += polyBLEP(v.phase, dt)
		out -= polyBLEP(math.Mod(v.phase-e.params.PulseDutyA+1, 1), dt)
		return out
	case wavePulseB:
		out := -1.0
		if v.phase < e.params.PulseDutyB {
			out = 1
		}
		out += polyBLEP(v.phase, dt)
		out -= polyBLEP(math.Mod(v.phase-e.params.PulseDutyB+1, 1), dt)
		return out
	case waveTriangle:
		return 2*math.Abs(2*v.phase-1) - 1
	case waveNoise:
		if v.phase < dt {
			bit := (v.noiseLFSR ^ (v.noiseLFSR >> 1)) & 1
			v.noiseLFSR = (v.noiseLFSR >> 1) | (bit << 15)
		}
		if v.noiseLFSR&1 == 1 {
			return 1
		}
		return -1
	default:
		return 0
	}
}

// waveForProgram maps program/module/channel to a waveform deterministically.
// Program ranges: 0-31 = pulseA, 32-63 = pulseB, 64-95 = triangle, 96+ = noise.
// Module overrides: module 0-1 = pulse, module 2 = triangle, module 9 = noise.
func waveForProgram(program int, module int, channel int) waveType {
	if module == 9 {
		return waveNoise
	}
	if module == 2 {
		return waveTriangle
	}
	if program >= 96 {
		return waveNoise
	}
	if program >= 64 {
		return waveTriangle
	}
	if program >= 32 {
		return wavePulseB
	}
	// Use channel to differentiate pulse duties for low program numbers.
	if channel%2 == 1 {
		return wavePulseB
	}
	return wavePulseA
}

func (e *Engine) stealVoice() int {
	// Prefer an inactive slot.
	for i := range e.voices {
		if !e.voices[i].active {
			return i
		}
	}
	// Steal the oldest releasing voice, or failing that the oldest active voice.
	oldestRelease := -1
	oldestReleaseAge := -1
	oldestActive := 0
	oldestActiveAge := -1
	for i := range e.voices {
		v := &e.voices[i]
		if v.envState == envRelease && v.age > oldestReleaseAge {
			oldestRelease = i
			oldestReleaseAge = v.age
		}
		if v.age > oldestActiveAge {
			oldestActive = i
			oldestActiveAge = v.age
		}
	}
	if oldestRelease >= 0 {
		return oldestRelease
	}
	return oldestActive
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

func quantize(v float64, steps int) float64 {
	if steps <= 1 {
		return v
	}
	n := math.Round(v*float64(steps-1)) / float64(steps-1)
	return clamp(n, 0, 1)
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

func (e *Engine) SetMasterGain(gain float64) {
	if gain < 0 {
		gain = 0
	}
	atomic.StoreUint64(&e.masterGain, math.Float64bits(gain))
}

func (e *Engine) ActiveVoiceCount() int {
	n := 0
	for i := range e.voices {
		if e.voices[i].active {
			n++
		}
	}
	return n
}

func (e *Engine) masterGainValue() float64 {
	return math.Float64frombits(atomic.LoadUint64(&e.masterGain))
}

func (e *Engine) SetFilterType(filterType int) {
	switch filterType {
	case 1:
		e.filterKind = filterBP
	case 2:
		e.filterKind = filterHP
	default:
		e.filterKind = filterLP
	}
}

func (e *Engine) SetNoteOnPhase(phase int) {
	e.nextPhase = phase
}

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

func decodeProgram(encoded int) (program int, module int, channel int) {
	if encoded < 0 {
		encoded = 0
	}
	program = encoded & 0xFF
	module = (encoded >> 8) & 0xFF
	channel = (encoded >> 16) & 0xFF
	return
}
