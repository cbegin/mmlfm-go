package effects

import (
	"math"
	"sync/atomic"
)

// EQ5Band implements a 5-band equalizer with runtime-adjustable gains.
// Bands are split at 200Hz, 800Hz, 2.5kHz, and 8kHz.
// Gains are stored as uint32 (bit-cast float32) for lock-free reads from the audio thread.
type EQ5Band struct {
	gains  [5]atomic.Uint32 // float32 bit patterns; 1.0 = unity
	alphas [4]float32       // crossover filter coefficients
	lpL    [4]float32       // lowpass state per crossover, left
	lpR    [4]float32       // lowpass state per crossover, right
}

var defaultCrossovers = [4]float64{200, 800, 2500, 8000}

// NewEQ5Band creates a 5-band EQ with all gains at unity.
func NewEQ5Band(sampleRate int) *EQ5Band {
	eq := &EQ5Band{}
	dt := 1.0 / float64(sampleRate)
	for i, freq := range defaultCrossovers {
		rc := 1.0 / (2.0 * math.Pi * freq)
		eq.alphas[i] = float32(dt / (rc + dt))
	}
	for i := range eq.gains {
		eq.gains[i].Store(math.Float32bits(1.0))
	}
	return eq
}

// SetGain sets the gain for band (0-4). 1.0 = unity, 0.0 = silence, 2.0 = +6dB.
func (eq *EQ5Band) SetGain(band int, gain float32) {
	if band >= 0 && band < 5 {
		eq.gains[band].Store(math.Float32bits(gain))
	}
}

// Gain returns the current gain for band (0-4).
func (eq *EQ5Band) Gain(band int) float32 {
	if band >= 0 && band < 5 {
		return math.Float32frombits(eq.gains[band].Load())
	}
	return 1.0
}

func (eq *EQ5Band) Process(l, r float32) (float32, float32) {
	// Split into 5 bands using 4 cascaded crossover filters.
	// Band 0: below crossover[0]
	// Band 1: crossover[0] .. crossover[1]
	// Band 2: crossover[1] .. crossover[2]
	// Band 3: crossover[2] .. crossover[3]
	// Band 4: above crossover[3]
	var bandL, bandR [5]float32
	remL, remR := l, r
	for i := 0; i < 4; i++ {
		eq.lpL[i] += eq.alphas[i] * (remL - eq.lpL[i])
		eq.lpR[i] += eq.alphas[i] * (remR - eq.lpR[i])
		bandL[i] = eq.lpL[i]
		bandR[i] = eq.lpR[i]
		remL -= bandL[i]
		remR -= bandR[i]
	}
	bandL[4] = remL
	bandR[4] = remR

	var outL, outR float32
	for i := 0; i < 5; i++ {
		g := math.Float32frombits(eq.gains[i].Load())
		outL += bandL[i] * g
		outR += bandR[i] * g
	}
	return outL, outR
}

func (eq *EQ5Band) Reset() {
	for i := range eq.lpL {
		eq.lpL[i] = 0
		eq.lpR[i] = 0
	}
}
