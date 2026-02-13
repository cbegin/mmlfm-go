package fm

import (
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/cbegin/mmlfm-go/internal/lfo"
)

const twoPi = math.Pi * 2

type Params struct {
	Polyphony   int
	CarrierMul  float64
	ModMul      float64
	ModIndex    float64
	AttackSec   float64
	DecaySec    float64
	SustainLvl  float64
	ReleaseSec  float64
	MasterGain  float64
	DefaultPan  float64
	VelocityAmp float64
	LPFCutoff   float64 // lowpass filter cutoff in Hz (0 = disabled)
}

func DefaultParams() Params {
	return Params{
		Polyphony:   32,
		CarrierMul:  1.0,
		ModMul:      2.0,
		ModIndex:    1.6,
		AttackSec:   0.005,
		DecaySec:    0.12,
		SustainLvl:  0.75,
		ReleaseSec:  0.2,
		MasterGain:  0.45,
		DefaultPan:  0,
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

// opmPatch holds OPM-format operator parameters for one program.
type opmPatch struct {
	alg  int
	fb   float64
	op   [4]opmOperator
}

type opmOperator struct {
	ar, dr, sr, rr float64 // envelope rates (converted to sec)
	sl             float64 // sustain level 0-1
	tl             float64 // total level 0-1
	mul            float64
}

var opmNumRegex = regexp.MustCompile(`-?\d+`)

type Engine struct {
	sampleRate       float64
	params           Params
	voices           []voice
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
	algorithm        int
	feedback         float64
	opCount          int
	patches          map[int]*opmPatch
	pitchLFO         lfo.LFO
	ampLFO           lfo.LFO
	filterLFO        lfo.LFO
}

type envState int

const (
	envAttack envState = iota
	envDecay
	envSustain
	envRelease
	envOff
)

type operator struct {
	phase    float64
	env      float64
	envState envState
	mul      float64
	tl       float64 // total level (1.0 = full output, 0 = silent)
	ar       float64
	dr       float64
	sl       float64
	rr       float64
	prevOut  float64
}

type voice struct {
	active           bool
	id               int
	velocity         float64
	freq             float64
	ops              [4]operator
	numOps           int
	alg              int
	fb               float64
	fbPrev           float64
	pan              float64
	module           int
	channel          int
	program          int
	waveform         int
	portamentoTarget float64
	portamentoFrames int
	portamentoStep   float64
}

func New(sampleRate int, params Params) *Engine {
	if params.Polyphony <= 0 {
		params.Polyphony = 32
	}
	e := &Engine{
		sampleRate: float64(sampleRate),
		params:     params,
		voices:     make([]voice, params.Polyphony),
		masterGain: math.Float64bits(params.MasterGain),
		opCount:    2,
		patches:    make(map[int]*opmPatch),
	}
	if params.LPFCutoff > 0 && params.LPFCutoff < float64(sampleRate)/2 {
		rc := 1.0 / (twoPi * params.LPFCutoff)
		dt := 1.0 / float64(sampleRate)
		e.lpfAlpha = dt / (rc + dt)
		e.baseLPFCutoff = params.LPFCutoff
	}
	return e
}

// SetAlgorithm sets the operator connection topology (0-7).
func (e *Engine) SetAlgorithm(alg int) {
	if alg < 0 {
		alg = 0
	}
	if alg > 7 {
		alg = 7
	}
	e.algorithm = alg
}

// SetFeedback sets self-feedback for operator 1 (0.0-1.0).
func (e *Engine) SetFeedback(fb float64) {
	e.feedback = clamp(fb, 0, 1)
}

// SetOperatorCount sets the number of active operators (1-4).
func (e *Engine) SetOperatorCount(count int) {
	if count < 1 {
		count = 1
	}
	if count > 4 {
		count = 4
	}
	e.opCount = count
}

// LoadOPMPatch parses OPM-format patch data and stores it for the given program number.
// OPM format: alg, fb, then 4 operators with AR,D1R,D2R,RR,D1L,TL,KS,MUL,DT1,DT2,AMS each.
func (e *Engine) LoadOPMPatch(program int, data []int) {
	if len(data) < 2+4*11 {
		return
	}
	p := &opmPatch{
		alg: clampInt(data[0], 0, 7),
		fb:  float64(clampInt(data[1], 0, 7)) / 7.0,
	}
	for oi := 0; oi < 4; oi++ {
		base := 2 + oi*11
		ar, d1r, d2r, rr, d1l, tl, _, mul := data[base], data[base+1], data[base+2], data[base+3], data[base+4], data[base+5], data[base+6], data[base+7]
		op := &p.op[oi]
		op.ar = clamp(0.001+float64(31-clampInt(ar, 0, 31))/31.0*0.3, 0.001, 8)
		op.dr = clamp(0.01+float64(31-clampInt(d1r, 0, 31))/31.0*0.2, 0.01, 4)
		op.sr = clamp(0.01+float64(31-clampInt(d2r, 0, 31))/31.0*0.2, 0.01, 4)
		op.rr = clamp(0.01+float64(15-clampInt(rr, 0, 15))/15.0*0.3, 0.01, 4)
		op.sl = clamp(float64(clampInt(d1l, 0, 15))/15.0, 0, 1)
		op.tl = clamp((127-float64(clampInt(tl, 0, 127)))/127.0, 0, 1)
		if mul == 0 {
			op.mul = 0.5
		} else {
			op.mul = float64(clampInt(mul, 0, 15))
		}
	}
	e.patches[program] = p
}

// LoadOPMPatchFromDefs loads all #OPM@ definitions from a map (e.g. score.Definitions).
func (e *Engine) LoadOPMPatchFromDefs(defs map[string]string) {
	if defs == nil {
		return
	}
	for key, body := range defs {
		if !strings.HasPrefix(strings.ToUpper(key), "OPM@") {
			continue
		}
		idx := strings.Index(key, "@")
		if idx < 0 {
			continue
		}
		program, err := strconv.Atoi(strings.TrimSpace(key[idx+1:]))
		if err != nil {
			continue
		}
		// Parse only from inside braces to avoid OPM@052 etc. poisoning the number stream
		braceIdx := strings.Index(body, "{")
		if braceIdx < 0 {
			continue
		}
		nums := opmNumRegex.FindAllString(body[braceIdx:], -1)
		data := make([]int, 0, len(nums))
		for _, s := range nums {
			n, err := strconv.Atoi(s)
			if err != nil {
				continue
			}
			data = append(data, n)
		}
		if len(data) >= 2+4*11 {
			e.LoadOPMPatch(program, data)
		}
	}
}

func (e *Engine) NoteOn(note int, velocity int, pan int, encodedProgram int) int {
	slot := e.stealVoice()
	id := e.nextID
	e.nextID++
	program, module, channel := decodeProgram(encodedProgram)
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
	var initPhase float64
	switch e.nextPhase {
	case -1:
		initPhase = rand.Float64() * twoPi
	case 0:
	default:
		initPhase = math.Mod(float64(e.nextPhase)/128.0*math.Pi, twoPi)
	}
	e.nextPhase = 0
	waveform := program % 8
	numOps := e.opCount
	if numOps <= 0 {
		numOps = 2
	}
	alg := e.algorithm
	fb := e.feedback
	if pat := e.patches[program]; pat != nil {
		alg = pat.alg
		fb = pat.fb
		numOps = 4
	}
	*v = voice{
		active:           true,
		id:               id,
		velocity:         clamp(float64(velocity)/127.0, 0, 1),
		freq:             freq,
		numOps:           numOps,
		alg:              alg,
		fb:               fb,
		pan:              p,
		module:           module,
		channel:          channel,
		program:          program,
		waveform:         waveform,
		portamentoTarget: portTgt,
		portamentoFrames: portFrames,
		portamentoStep:   portStep,
	}
	// Initialize operators from patch or defaults
	pat := e.patches[program]
	for oi := 0; oi < numOps; oi++ {
		if pat != nil && oi < 4 {
			op := &pat.op[oi]
			v.ops[oi] = operator{
				phase:    initPhase,
				envState: envAttack,
				mul:      op.mul,
				tl:       op.tl,
				ar:       op.ar,
				dr:       op.dr,
				sl:       op.sl,
				rr:       op.rr,
			}
		} else {
			muls := [4]float64{e.params.CarrierMul, e.params.ModMul, 3.0, 4.0}
			v.ops[oi] = operator{
				phase:    initPhase,
				envState: envAttack,
				mul:      muls[oi],
				tl:       1.0,
				ar:       e.params.AttackSec,
				dr:       e.params.DecaySec,
				sl:       e.params.SustainLvl,
				rr:       e.params.ReleaseSec,
			}
		}
	}
	if pat == nil {
		for oi := 1; oi < numOps; oi++ {
			v.ops[oi].tl = e.params.ModIndex / 8.0
		}
	}
	return id
}

func (e *Engine) NoteOff(id int) {
	for i := range e.voices {
		v := &e.voices[i]
		if v.active && v.id == id {
			for oi := 0; oi < v.numOps; oi++ {
				if v.ops[oi].envState != envRelease {
					v.ops[oi].envState = envRelease
				}
			}
		}
	}
}

func (e *Engine) RenderFrame() (float32, float32) {
	// Sample LFOs once per frame (global, not per-voice)
	pitchMod := e.pitchLFO.Sample(e.sampleRate)  // in semitones
	ampMod := e.ampLFO.Sample(e.sampleRate)       // gain factor offset
	filterMod := e.filterLFO.Sample(e.sampleRate) // cutoff offset

	var l, r float64
	for i := range e.voices {
		v := &e.voices[i]
		if !v.active {
			continue
		}
		// Advance all operator envelopes
		allOff := true
		for oi := 0; oi < v.numOps; oi++ {
			advanceOpEnv(&v.ops[oi], e.sampleRate)
			if v.ops[oi].envState != envOff {
				allOff = false
			}
		}
		if allOff {
			v.active = false
			continue
		}
		// Compute operator outputs based on algorithm
		sig := e.renderVoice(v)
		sig *= e.masterGainValue() * (0.2 + v.velocity*e.params.VelocityAmp)
		// Apply amp LFO
		sig *= (1.0 + ampMod)
		// Pan
		angle := ((v.pan + 64.0) / 128.0) * (math.Pi / 2.0)
		l += sig * math.Cos(angle)
		r += sig * math.Sin(angle)
		// Portamento
		if v.portamentoFrames > 0 {
			v.portamentoFrames--
			v.freq += v.portamentoStep
			if v.portamentoFrames <= 0 {
				v.freq = v.portamentoTarget
			}
		}
		// Advance phases with pitch LFO modulation
		freqMul := 1.0
		if pitchMod != 0 {
			freqMul = math.Pow(2, pitchMod/12.0)
		}
		for oi := 0; oi < v.numOps; oi++ {
			op := &v.ops[oi]
			op.phase += twoPi * (v.freq * freqMul * op.mul) / e.sampleRate
			if op.phase > twoPi {
				op.phase -= twoPi
			}
		}
	}
	// Filter LFO: recalculate lpfAlpha if filter LFO is active
	if e.baseLPFCutoff > 0 && filterMod != 0 {
		cutoff := e.baseLPFCutoff + filterMod*100.0 // scale to Hz
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
	// Output filter
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

// renderVoice computes the FM synthesis output for a voice based on its algorithm.
// Algorithms define how operators connect (serial modulation vs parallel carriers).
func (e *Engine) renderVoice(v *voice) float64 {
	n := v.numOps
	ops := &v.ops
	// Compute each operator's raw output (env * tl * waveform)
	var out [4]float64
	for oi := 0; oi < n; oi++ {
		out[oi] = ops[oi].env * ops[oi].tl
	}
	switch {
	case n == 1:
		// Single operator: carrier only
		fb := ops[0].prevOut * v.fb * math.Pi
		s := waveformSample(ops[0].phase+fb, v.waveform) * out[0]
		ops[0].prevOut = s
		return s
	case n == 2:
		switch v.alg {
		case 1: // parallel: op0 + op1 both carriers
			s0 := waveformSample(ops[0].phase, v.waveform) * out[0]
			s1 := waveformSample(ops[1].phase, v.waveform) * out[1]
			return (s0 + s1) * (1.0 / math.Sqrt2) // RMS-aware scaling
		default: // alg 0: serial: op1 → op0
			fb := ops[1].prevOut * v.fb * math.Pi
			mod := math.Sin(ops[1].phase+fb) * out[1] * e.params.ModIndex
			ops[1].prevOut = math.Sin(ops[1].phase+fb) * out[1]
			return waveformSample(ops[0].phase+mod, v.waveform) * out[0]
		}
	case n == 3:
		switch v.alg {
		case 1: // op2→op1→op0 (full serial)
			fb := ops[2].prevOut * v.fb * math.Pi
			s2 := math.Sin(ops[2].phase+fb) * out[2] * e.params.ModIndex
			ops[2].prevOut = math.Sin(ops[2].phase+fb) * out[2]
			s1 := math.Sin(ops[1].phase+s2) * out[1] * e.params.ModIndex
			return waveformSample(ops[0].phase+s1, v.waveform) * out[0]
		case 2: // (op1+op2)→op0
			s1 := math.Sin(ops[1].phase) * out[1] * e.params.ModIndex
			s2 := math.Sin(ops[2].phase) * out[2] * e.params.ModIndex
			return waveformSample(ops[0].phase+s1+s2, v.waveform) * out[0]
		case 3: // all parallel
			s0 := waveformSample(ops[0].phase, v.waveform) * out[0]
			s1 := waveformSample(ops[1].phase, v.waveform) * out[1]
			s2 := waveformSample(ops[2].phase, v.waveform) * out[2]
			return (s0 + s1 + s2) * (1.0 / math.Sqrt(3)) // RMS-aware scaling
		default: // alg 0: op2→op1, op1→op0
			s2 := math.Sin(ops[2].phase) * out[2] * e.params.ModIndex
			s1 := math.Sin(ops[1].phase+s2) * out[1] * e.params.ModIndex
			return waveformSample(ops[0].phase+s1, v.waveform) * out[0]
		}
	default: // n == 4
		switch v.alg {
		case 1: // op3→op2→op1→op0 (full serial)
			s3 := math.Sin(ops[3].phase) * out[3] * e.params.ModIndex
			s2 := math.Sin(ops[2].phase+s3) * out[2] * e.params.ModIndex
			s1 := math.Sin(ops[1].phase+s2) * out[1] * e.params.ModIndex
			return waveformSample(ops[0].phase+s1, v.waveform) * out[0]
		case 2: // (op2+op3)→op1→op0
			s2 := math.Sin(ops[2].phase) * out[2] * e.params.ModIndex
			s3 := math.Sin(ops[3].phase) * out[3] * e.params.ModIndex
			s1 := math.Sin(ops[1].phase+s2+s3) * out[1] * e.params.ModIndex
			return waveformSample(ops[0].phase+s1, v.waveform) * out[0]
		case 3: // op2→op1, op3→op0 (two pairs)
			s2 := math.Sin(ops[2].phase) * out[2] * e.params.ModIndex
			s3 := math.Sin(ops[3].phase) * out[3] * e.params.ModIndex
			c0 := waveformSample(ops[0].phase+s3, v.waveform) * out[0]
			c1 := waveformSample(ops[1].phase+s2, v.waveform) * out[1]
			return (c0 + c1) * (1.0 / math.Sqrt2) // RMS-aware scaling
		case 4: // op3→op2→op1, op0 carrier
			s3 := math.Sin(ops[3].phase) * out[3] * e.params.ModIndex
			s2 := math.Sin(ops[2].phase+s3) * out[2] * e.params.ModIndex
			s1 := math.Sin(ops[1].phase+s2) * out[1]
			s0 := waveformSample(ops[0].phase, v.waveform) * out[0]
			return (s0 + s1) * (1.0 / math.Sqrt2) // RMS-aware scaling
		case 5: // all parallel
			s := 0.0
			for oi := 0; oi < 4; oi++ {
				s += waveformSample(ops[oi].phase, v.waveform) * out[oi]
			}
			return s * 0.5 // 1/sqrt(4), RMS-aware scaling
		default: // alg 0: op3→op2, op2→op1, op1→op0 (cascade)
			fb := ops[3].prevOut * v.fb * math.Pi
			s3 := math.Sin(ops[3].phase+fb) * out[3] * e.params.ModIndex
			ops[3].prevOut = math.Sin(ops[3].phase+fb) * out[3]
			s2 := math.Sin(ops[2].phase+s3) * out[2] * e.params.ModIndex
			s1 := math.Sin(ops[1].phase+s2) * out[1] * e.params.ModIndex
			return waveformSample(ops[0].phase+s1, v.waveform) * out[0]
		}
	}
}

func (e *Engine) stealVoice() int {
	for i := range e.voices {
		if !e.voices[i].active {
			return i
		}
	}
	// Steal quietest voice (carrier env)
	quiet := 0
	minEnv := e.voices[0].ops[0].env
	for i := 1; i < len(e.voices); i++ {
		if e.voices[i].ops[0].env < minEnv {
			minEnv = e.voices[i].ops[0].env
			quiet = i
		}
	}
	return quiet
}

func advanceOpEnv(op *operator, sampleRate float64) {
	switch op.envState {
	case envAttack:
		step := 1.0 / (op.ar * sampleRate)
		if step <= 0 {
			step = 1
		}
		op.env += step
		if op.env >= 1 {
			op.env = 1
			op.envState = envDecay
		}
	case envDecay:
		step := (1 - op.sl) / (op.dr * sampleRate)
		if step <= 0 {
			step = 1
		}
		op.env -= step
		if op.env <= op.sl {
			op.env = op.sl
			op.envState = envSustain
		}
	case envSustain:
	case envRelease:
		step := op.sl / (op.rr * sampleRate)
		if step <= 0 {
			step = 1
		}
		op.env -= step
		if op.env <= 0.0001 {
			op.env = 0
			op.envState = envOff
		}
	case envOff:
		op.env = 0
	}
}

var noiseLFSR uint32 = 0x7FFF

func waveformSample(phase float64, waveform int) float64 {
	switch waveform {
	case 1: // saw
		return 1.0 - 2.0*math.Mod(phase, twoPi)/twoPi
	case 2: // triangle
		return 2.0*math.Abs(2.0*math.Mod(phase, twoPi)/twoPi-1.0) - 1.0
	case 3: // square
		if math.Mod(phase, twoPi) < math.Pi {
			return 1.0
		}
		return -1.0
	case 4: // pulse 25%
		if math.Mod(phase, twoPi) < math.Pi/2 {
			return 1.0
		}
		return -1.0
	case 5: // pulse 12.5%
		if math.Mod(phase, twoPi) < math.Pi/4 {
			return 1.0
		}
		return -1.0
	case 6: // half-rectified sine
		s := math.Sin(phase)
		if s > 0 {
			return s
		}
		return 0
	case 7: // noise
		noiseLFSR = (noiseLFSR >> 1) ^ (-(noiseLFSR & 1) & 0xB400)
		return float64(noiseLFSR)/float64(0x7FFF)*2.0 - 1.0
	default: // 0 = sine
		return math.Sin(phase)
	}
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

func clampInt(v, lo, hi int) int {
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
