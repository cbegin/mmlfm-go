package nesapu

import (
	"math"
	"math/rand"
	"sync/atomic"

	"github.com/cbegin/mmlfm-go/internal/lfo"
)

const (
	twoPi            = math.Pi * 2
	defaultFrameRate = 240.0
)

type Params struct {
	MasterGain   float64
	PulseDutyA   float64
	PulseDutyB   float64
	ReleaseStep  float64
	NoiseCutoff  int
	TriangleGain float64
	PulseGain    float64
	NoiseGain    float64
	LPFCutoff    float64 // lowpass filter cutoff in Hz (0 = disabled)
}

func DefaultParams() Params {
	return Params{
		MasterGain:   0.32,
		PulseDutyA:   0.125,
		PulseDutyB:   0.25,
		ReleaseStep:  1.0 / 48.0,
		NoiseCutoff:  84,
		TriangleGain: 0.85,
		PulseGain:    1.0,
		NoiseGain:    0.45,
		LPFCutoff:    12000,
	}
}

type slotKind int

const (
	slotPulse1 slotKind = iota
	slotPulse2
	slotTriangle
	slotNoise
	slotCount
)

type slotRef struct {
	kind slotKind
}

type pulse struct {
	active           bool
	id               int
	age              int
	freq             float64
	phase            float64
	vol              float64
	pan              float64
	released         bool
	portamentoTarget float64
	portamentoFrames int
	portamentoStep   float64
}

type triangle struct {
	active           bool
	id               int
	age              int
	freq             float64
	phase            float64
	vol              float64
	pan              float64
	released         bool
	portamentoTarget float64
	portamentoFrames int
	portamentoStep   float64
}

type noise struct {
	active   bool
	id       int
	age      int
	vol      float64
	pan      float64
	released bool
	lfsr     uint16
}

type filterType int

const (
	nesFilterLP filterType = iota
	nesFilterHP
	nesFilterBP
)

type Engine struct {
	sampleRate       float64
	params           Params
	pulseA           pulse
	pulseB           pulse
	triangle         triangle
	noise            noise
	activeByID       map[int]slotRef
	nextID           int
	assignCounter    int
	frameCounter     int
	framePeriod      int
	masterGain       uint64
	lpfL             float64
	lpfR             float64
	bpfL             float64
	bpfR             float64
	lpfAlpha         float64
	filterKind       filterType
	baseLPFCutoff    float64
	nextPhase        int
	portamentoFrom   int
	portamentoFrames int
	pitchLFO         lfo.LFO
	ampLFO           lfo.LFO
	filterLFO        lfo.LFO
}

func New(sampleRate int, params Params) *Engine {
	period := int(float64(sampleRate) / defaultFrameRate)
	if period <= 0 {
		period = 1
	}
	e := &Engine{
		sampleRate:  float64(sampleRate),
		params:      params,
		activeByID:  make(map[int]slotRef),
		framePeriod: period,
		masterGain:  math.Float64bits(params.MasterGain),
		noise:       noise{lfsr: 0xACE1},
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
	id := e.nextID
	e.nextID++
	program, module, channel := decodeProgram(encodedProgram)
	vel := clamp(float64(velocity)/127.0, 0, 1)
	panNorm := clamp(float64(pan), -64, 64)

	// Determine which hardware slot to use.
	slot := assignSlot(note, program, module, channel, e.params.NoiseCutoff, e.assignCounter)

	switch slot {
	case slotNoise:
		if e.noise.active && !e.noise.released {
			// Steal: mark old note as gone.
			delete(e.activeByID, e.noise.id)
		}
		e.noise = noise{
			active: true, id: id, age: 0, vol: vel, pan: panNorm, released: false, lfsr: seedLFSR(e.noise.lfsr, note, id),
		}
		e.activeByID[id] = slotRef{kind: slotNoise}
	case slotTriangle:
		if e.triangle.active && !e.triangle.released {
			delete(e.activeByID, e.triangle.id)
		}
		freq, portTgt, portFrames, portStep := e.noteFreqParams(note)
		ph := e.phaseForSlot(slot)
		e.triangle = triangle{active: true, id: id, age: 0, freq: freq, phase: ph, vol: vel, pan: panNorm, portamentoTarget: portTgt, portamentoFrames: portFrames, portamentoStep: portStep}
		e.activeByID[id] = slotRef{kind: slotTriangle}
	case slotPulse2:
		if e.pulseB.active && !e.pulseB.released {
			delete(e.activeByID, e.pulseB.id)
		}
		freq, portTgt, portFrames, portStep := e.noteFreqParams(note)
		ph := e.phaseForSlot(slot)
		e.pulseB = pulse{active: true, id: id, age: 0, freq: freq, phase: ph, vol: vel, pan: panNorm, portamentoTarget: portTgt, portamentoFrames: portFrames, portamentoStep: portStep}
		e.activeByID[id] = slotRef{kind: slotPulse2}
	default: // slotPulse1
		if e.pulseA.active && !e.pulseA.released {
			delete(e.activeByID, e.pulseA.id)
		}
		freq, portTgt, portFrames, portStep := e.noteFreqParams(note)
		ph := e.phaseForSlot(slot)
		e.pulseA = pulse{active: true, id: id, age: 0, freq: freq, phase: ph, vol: vel, pan: panNorm, portamentoTarget: portTgt, portamentoFrames: portFrames, portamentoStep: portStep}
		e.activeByID[id] = slotRef{kind: slotPulse1}
	}
	e.assignCounter++
	e.portamentoFrom = -1
	e.portamentoFrames = 0
	e.nextPhase = 0
	return id
}

func (e *Engine) noteFreqParams(note int) (freq, portTgt float64, portFrames int, portStep float64) {
	freq = midiToFreq(note)
	if e.portamentoFrom >= 0 && e.portamentoFrames > 0 {
		freq = midiToFreq(e.portamentoFrom)
		portTgt = midiToFreq(note)
		portFrames = e.portamentoFrames
		portStep = (portTgt - freq) / float64(e.portamentoFrames)
	}
	return
}

func (e *Engine) phaseForSlot(slot slotKind) float64 {
	switch e.nextPhase {
	case -1:
		return rand.Float64()
	case 0:
		return 0
	default:
		return math.Mod(float64(e.nextPhase)/128.0*math.Pi, twoPi) / twoPi
	}
}

// assignSlot determines which hardware slot a note should go to based on
// the musical context rather than pure round-robin.
func assignSlot(note int, program int, module int, channel int, noiseCutoff int, counter int) slotKind {
	// Noise: high notes, drum module, or drum program.
	if note >= noiseCutoff || module == 9 || program == 9 {
		return slotNoise
	}
	// Module-based routing: module 2 → triangle (bass/sub), others → pulse.
	if module == 2 {
		return slotTriangle
	}
	// Program-based: 64-95 → triangle.
	if program >= 64 && program < 96 {
		return slotTriangle
	}
	// Low notes (below C3) prefer triangle for a warmer bass.
	if note < 48 {
		return slotTriangle
	}
	// Alternate between the two pulse channels using channel + counter.
	if (channel+counter)%2 == 0 {
		return slotPulse1
	}
	return slotPulse2
}

func (e *Engine) NoteOff(id int) {
	slot, ok := e.activeByID[id]
	if !ok {
		return
	}
	switch slot.kind {
	case slotPulse1:
		if e.pulseA.id == id {
			e.pulseA.released = true
		}
	case slotPulse2:
		if e.pulseB.id == id {
			e.pulseB.released = true
		}
	case slotTriangle:
		if e.triangle.id == id {
			e.triangle.released = true
		}
	case slotNoise:
		if e.noise.id == id {
			e.noise.released = true
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

	e.frameCounter++
	if e.frameCounter >= e.framePeriod {
		e.frameCounter = 0
		e.clockFrame()
	}

	// Age active voices.
	if e.pulseA.active {
		e.pulseA.age++
	}
	if e.pulseB.active {
		e.pulseB.age++
	}
	if e.triangle.active {
		e.triangle.age++
	}
	if e.noise.active {
		e.noise.age++
	}

	// Apply pitch LFO temporarily
	if freqMul != 1.0 {
		if e.pulseA.active {
			e.pulseA.freq *= freqMul
		}
		if e.pulseB.active {
			e.pulseB.freq *= freqMul
		}
		if e.triangle.active {
			e.triangle.freq *= freqMul
		}
	}

	p1, p1l, p1r := e.renderPulse(&e.pulseA, e.params.PulseDutyA)
	p2, p2l, p2r := e.renderPulse(&e.pulseB, e.params.PulseDutyB)
	t, tl, tr := e.renderTriangle(&e.triangle)
	n, nl, nr := e.renderNoise(&e.noise)

	// Restore original frequencies
	if freqMul != 1.0 {
		if e.pulseA.active {
			e.pulseA.freq /= freqMul
		}
		if e.pulseB.active {
			e.pulseB.freq /= freqMul
		}
		if e.triangle.active {
			e.triangle.freq /= freqMul
		}
	}

	// Apply amp LFO
	ampScale := 1.0 + ampMod

	l := (p1*p1l*e.params.PulseGain + p2*p2l*e.params.PulseGain + t*tl*e.params.TriangleGain + n*nl*e.params.NoiseGain) * e.masterGainValue() * ampScale
	r := (p1*p1r*e.params.PulseGain + p2*p2r*e.params.PulseGain + t*tr*e.params.TriangleGain + n*nr*e.params.NoiseGain) * e.masterGainValue() * ampScale

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
		case nesFilterLP:
			l = e.lpfL
			r = e.lpfR
		case nesFilterHP:
			l = l - e.lpfL
			r = r - e.lpfR
		case nesFilterBP:
			e.bpfL += e.lpfAlpha * (e.lpfL - e.bpfL)
			e.bpfR += e.lpfAlpha * (e.lpfR - e.bpfR)
			l = e.lpfL - e.bpfL
			r = e.lpfR - e.bpfR
		}
	}

	return float32(clamp(l, -1, 1)), float32(clamp(r, -1, 1))
}

func (e *Engine) clockFrame() {
	release := e.params.ReleaseStep
	if release <= 0 {
		release = 1.0 / 48.0
	}
	if e.pulseA.active && e.pulseA.released {
		e.pulseA.vol -= release
		if e.pulseA.vol <= 0 {
			delete(e.activeByID, e.pulseA.id)
			e.pulseA = pulse{}
		}
	}
	if e.pulseB.active && e.pulseB.released {
		e.pulseB.vol -= release
		if e.pulseB.vol <= 0 {
			delete(e.activeByID, e.pulseB.id)
			e.pulseB = pulse{}
		}
	}
	if e.triangle.active && e.triangle.released {
		e.triangle.vol -= release
		if e.triangle.vol <= 0 {
			delete(e.activeByID, e.triangle.id)
			e.triangle = triangle{}
		}
	}
	if e.noise.active && e.noise.released {
		e.noise.vol -= release
		if e.noise.vol <= 0 {
			delete(e.activeByID, e.noise.id)
			e.noise = noise{lfsr: 0xACE1}
		}
	}
}

// polyBLEP reduces aliasing at waveform discontinuities.
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

func (e *Engine) renderPulse(p *pulse, duty float64) (float64, float64, float64) {
	if !p.active {
		return 0, 0, 0
	}
	if p.portamentoFrames > 0 {
		p.portamentoFrames--
		p.freq += p.portamentoStep
		if p.portamentoFrames <= 0 {
			p.freq = p.portamentoTarget
		}
	}
	dt := p.freq / e.sampleRate
	p.phase += dt
	if p.phase >= 1 {
		p.phase -= 1
	}
	v := -1.0
	if p.phase < duty {
		v = 1
	}
	// Apply PolyBLEP anti-aliasing at both transitions.
	v += polyBLEP(p.phase, dt)
	v -= polyBLEP(math.Mod(p.phase-duty+1, 1), dt)
	level := quantize(p.vol, 16)
	angle := ((p.pan + 64.0) / 128.0) * (math.Pi / 2.0)
	return v * level, math.Cos(angle), math.Sin(angle)
}

func (e *Engine) renderTriangle(t *triangle) (float64, float64, float64) {
	if !t.active {
		return 0, 0, 0
	}
	if t.portamentoFrames > 0 {
		t.portamentoFrames--
		t.freq += t.portamentoStep
		if t.portamentoFrames <= 0 {
			t.freq = t.portamentoTarget
		}
	}
	dt := t.freq / e.sampleRate
	t.phase += dt
	if t.phase >= 1 {
		t.phase -= 1
	}
	raw := 2*math.Abs(2*t.phase-1) - 1
	level := quantize(t.vol, 16)
	angle := ((t.pan + 64.0) / 128.0) * (math.Pi / 2.0)
	return raw * level, math.Cos(angle), math.Sin(angle)
}

func (e *Engine) renderNoise(n *noise) (float64, float64, float64) {
	if !n.active {
		return 0, 0, 0
	}
	bit := (n.lfsr ^ (n.lfsr >> 1)) & 1
	n.lfsr = (n.lfsr >> 1) | (bit << 15)
	v := -1.0
	if n.lfsr&1 == 1 {
		v = 1
	}
	level := quantize(n.vol, 16)
	angle := ((n.pan + 64.0) / 128.0) * (math.Pi / 2.0)
	return v * level, math.Cos(angle), math.Sin(angle)
}

func midiToFreq(note int) float64 {
	return 440 * math.Pow(2, float64(note-69)/12)
}

func quantize(v float64, steps int) float64 {
	if steps <= 1 {
		return clamp(v, 0, 1)
	}
	return clamp(math.Round(v*float64(steps-1))/float64(steps-1), 0, 1)
}

func seedLFSR(prev uint16, note int, id int) uint16 {
	s := prev ^ uint16((note&0x7f)<<1) ^ uint16(id*73)
	if s == 0 {
		return 0xACE1
	}
	return s
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

func (e *Engine) SetFilterType(filterType int) {
	switch filterType {
	case 1:
		e.filterKind = nesFilterBP
	case 2:
		e.filterKind = nesFilterHP
	default:
		e.filterKind = nesFilterLP
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

func (e *Engine) SetMasterGain(gain float64) {
	if gain < 0 {
		gain = 0
	}
	atomic.StoreUint64(&e.masterGain, math.Float64bits(gain))
}

func (e *Engine) ActiveVoiceCount() int {
	n := 0
	if e.pulseA.active {
		n++
	}
	if e.pulseB.active {
		n++
	}
	if e.triangle.active {
		n++
	}
	if e.noise.active {
		n++
	}
	return n
}

func (e *Engine) masterGainValue() float64 {
	return math.Float64frombits(atomic.LoadUint64(&e.masterGain))
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
