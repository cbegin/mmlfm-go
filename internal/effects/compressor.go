package effects

import "math"

// Compressor implements basic dynamic range compression.
type Compressor struct {
	threshold float32
	ratio     float32
	attack    float32 // coefficient
	release   float32 // coefficient
	makeupDB  float32
	makeup    float32
	envL      float32
	envR      float32
}

// NewCompressor creates a compressor effect.
// thresholdDB: threshold in dB (e.g., -20)
// ratio: compression ratio (e.g., 4 for 4:1)
// attackMs: attack time in ms
// releaseMs: release time in ms
// makeupDB: makeup gain in dB
func NewCompressor(sampleRate int, thresholdDB, ratio, attackMs, releaseMs, makeupDB float32) *Compressor {
	sr := float64(sampleRate)
	return &Compressor{
		threshold: float32(math.Pow(10, float64(thresholdDB)/20)),
		ratio:     ratio,
		attack:    float32(1.0 - math.Exp(-1.0/(float64(attackMs)*sr/1000.0))),
		release:   float32(1.0 - math.Exp(-1.0/(float64(releaseMs)*sr/1000.0))),
		makeupDB:  makeupDB,
		makeup:    float32(math.Pow(10, float64(makeupDB)/20)),
	}
}

func (c *Compressor) Process(l, r float32) (float32, float32) {
	absL := float32(math.Abs(float64(l)))
	absR := float32(math.Abs(float64(r)))
	// Envelope follower
	if absL > c.envL {
		c.envL += c.attack * (absL - c.envL)
	} else {
		c.envL += c.release * (absL - c.envL)
	}
	if absR > c.envR {
		c.envR += c.attack * (absR - c.envR)
	} else {
		c.envR += c.release * (absR - c.envR)
	}
	// Gain reduction
	gainL := c.computeGain(c.envL)
	gainR := c.computeGain(c.envR)
	return l * gainL * c.makeup, r * gainR * c.makeup
}

func (c *Compressor) computeGain(env float32) float32 {
	if env <= c.threshold || c.threshold <= 0 {
		return 1.0
	}
	// How far above threshold in linear scale
	over := env / c.threshold
	// Apply ratio: reduce the excess
	compressed := float32(math.Pow(float64(over), float64(1.0/c.ratio-1)))
	return compressed
}

func (c *Compressor) Reset() {
	c.envL = 0
	c.envR = 0
}
