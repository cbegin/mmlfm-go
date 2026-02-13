package effects

// Reverb implements a Schroeder-style reverb with multiple comb filters
// and two allpass filters.
type Reverb struct {
	combs    [4]combFilter
	allpass  [2]allpassFilter
	wet      float32
}

type combFilter struct {
	buf  []float32
	pos  int
	fb   float32
}

type allpassFilter struct {
	buf  []float32
	pos  int
	fb   float32
}

// NewReverb creates a reverb effect.
// roomSize: 0..1 controls delay lengths
// feedback: 0..1 controls decay time
// wet: wet/dry mix 0..1
func NewReverb(sampleRate int, roomSize, feedback, wet float32) *Reverb {
	base := int(float32(sampleRate) * roomSize * 0.05)
	if base < 10 {
		base = 10
	}
	fb := clamp(feedback, 0, 0.95)
	r := &Reverb{wet: clamp(wet, 0, 1)}
	// Comb filter delay lengths (prime-ish ratios to avoid resonances)
	combLens := [4]int{base, base * 1117 / 1000, base * 1271 / 1000, base * 1437 / 1000}
	for i := range r.combs {
		r.combs[i] = combFilter{
			buf: make([]float32, combLens[i]),
			fb:  fb,
		}
	}
	// Allpass filter delay lengths
	apLens := [2]int{base * 347 / 1000, base * 213 / 1000}
	for i := range r.allpass {
		r.allpass[i] = allpassFilter{
			buf: make([]float32, maxInt(apLens[i], 1)),
			fb:  0.5,
		}
	}
	return r
}

func (r *Reverb) Process(l, r2 float32) (float32, float32) {
	mono := (l + r2) * 0.5
	var out float32
	for i := range r.combs {
		out += r.combs[i].process(mono)
	}
	out *= 0.25
	for i := range r.allpass {
		out = r.allpass[i].process(out)
	}
	return l*(1-r.wet) + out*r.wet, r2*(1-r.wet) + out*r.wet
}

func (r *Reverb) Reset() {
	for i := range r.combs {
		for j := range r.combs[i].buf {
			r.combs[i].buf[j] = 0
		}
		r.combs[i].pos = 0
	}
	for i := range r.allpass {
		for j := range r.allpass[i].buf {
			r.allpass[i].buf[j] = 0
		}
		r.allpass[i].pos = 0
	}
}

func (c *combFilter) process(in float32) float32 {
	out := c.buf[c.pos]
	c.buf[c.pos] = in + out*c.fb
	c.pos++
	if c.pos >= len(c.buf) {
		c.pos = 0
	}
	return out
}

func (a *allpassFilter) process(in float32) float32 {
	bufOut := a.buf[a.pos]
	out := -in + bufOut
	a.buf[a.pos] = in + bufOut*a.fb
	a.pos++
	if a.pos >= len(a.buf) {
		a.pos = 0
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
