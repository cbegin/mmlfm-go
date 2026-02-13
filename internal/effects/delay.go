package effects

// Delay implements a simple stereo delay with feedback and cross-channel mixing.
type Delay struct {
	bufL, bufR []float32
	pos        int
	feedback   float32
	cross      float32
	wet        float32
}

// NewDelay creates a delay effect.
// delayMs: delay time in milliseconds
// feedback: feedback amount 0..1
// cross: cross-channel feedback 0..1
// wet: wet/dry mix 0..1
func NewDelay(sampleRate int, delayMs float64, feedback, cross, wet float32) *Delay {
	samples := int(delayMs * float64(sampleRate) / 1000.0)
	if samples < 1 {
		samples = 1
	}
	return &Delay{
		bufL:     make([]float32, samples),
		bufR:     make([]float32, samples),
		feedback: clamp(feedback, 0, 0.95),
		cross:    clamp(cross, 0, 1),
		wet:      clamp(wet, 0, 1),
	}
}

func (d *Delay) Process(l, r float32) (float32, float32) {
	delL := d.bufL[d.pos]
	delR := d.bufR[d.pos]
	fbL := delL*d.feedback*(1-d.cross) + delR*d.feedback*d.cross
	fbR := delR*d.feedback*(1-d.cross) + delL*d.feedback*d.cross
	d.bufL[d.pos] = l + fbL
	d.bufR[d.pos] = r + fbR
	d.pos++
	if d.pos >= len(d.bufL) {
		d.pos = 0
	}
	return l*(1-d.wet) + delL*d.wet, r*(1-d.wet) + delR*d.wet
}

func (d *Delay) Reset() {
	for i := range d.bufL {
		d.bufL[i] = 0
		d.bufR[i] = 0
	}
	d.pos = 0
}

func clamp(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
