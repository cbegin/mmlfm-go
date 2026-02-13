package effects

import "math"

// Chorus implements a modulated delay for chorus/flanger effects.
type Chorus struct {
	bufL, bufR []float32
	pos        int
	size       int
	depth      float32 // modulation depth in samples
	rate       float64 // modulation rate in radians per sample
	phase      float64
	feedback   float32
	wet        float32
}

// NewChorus creates a chorus/flanger effect.
// delayMs: base delay time in ms (typically 5-30ms)
// feedback: feedback amount 0..1
// depthMs: modulation depth in ms
// rateHz: modulation rate in Hz (typically 0.1-5Hz)
// wet: wet/dry mix 0..1
func NewChorus(sampleRate int, delayMs, feedback, depthMs, rateHz, wet float32) *Chorus {
	baseSamples := int(float64(delayMs) * float64(sampleRate) / 1000.0)
	depthSamples := float64(depthMs) * float64(sampleRate) / 1000.0
	size := baseSamples + int(depthSamples) + 2
	if size < 4 {
		size = 4
	}
	return &Chorus{
		bufL:     make([]float32, size),
		bufR:     make([]float32, size),
		size:     size,
		depth:    float32(depthSamples),
		rate:     2.0 * math.Pi * float64(rateHz) / float64(sampleRate),
		feedback: clamp(feedback, 0, 0.9),
		wet:      clamp(wet, 0, 1),
	}
}

func (c *Chorus) Process(l, r float32) (float32, float32) {
	mod := float32(math.Sin(c.phase)) * c.depth
	c.phase += c.rate
	if c.phase > 2*math.Pi {
		c.phase -= 2 * math.Pi
	}
	// Write input + feedback into buffer
	c.bufL[c.pos] = l
	c.bufR[c.pos] = r

	// Read with fractional delay
	delay := float32(c.size/2) + mod
	readPos := float32(c.pos) - delay
	for readPos < 0 {
		readPos += float32(c.size)
	}
	idx := int(readPos)
	frac := readPos - float32(idx)
	idx2 := idx + 1
	if idx2 >= c.size {
		idx2 = 0
	}
	delL := c.bufL[idx]*(1-frac) + c.bufL[idx2]*frac
	delR := c.bufR[idx]*(1-frac) + c.bufR[idx2]*frac

	c.bufL[c.pos] += delL * c.feedback
	c.bufR[c.pos] += delR * c.feedback

	c.pos++
	if c.pos >= c.size {
		c.pos = 0
	}
	return l*(1-c.wet) + delL*c.wet, r*(1-c.wet) + delR*c.wet
}

func (c *Chorus) Reset() {
	for i := range c.bufL {
		c.bufL[i] = 0
		c.bufR[i] = 0
	}
	c.pos = 0
	c.phase = 0
}
