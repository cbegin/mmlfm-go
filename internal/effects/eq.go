package effects

import "math"

// EQ3Band implements a simple 3-band equalizer.
type EQ3Band struct {
	lowGain  float32
	midGain  float32
	highGain float32
	lpAlpha  float32
	hpAlpha  float32
	lpL, lpR float32 // lowpass state
	hpL, hpR float32 // highpass state
}

// NewEQ3Band creates a 3-band EQ.
// lowGain, midGain, highGain: gain for each band (1.0 = unity)
// lowFreq: crossover frequency between low and mid bands
// highFreq: crossover frequency between mid and high bands
func NewEQ3Band(sampleRate int, lowGain, midGain, highGain, lowFreq, highFreq float32) *EQ3Band {
	lpRC := 1.0 / (2.0 * math.Pi * float64(lowFreq))
	hpRC := 1.0 / (2.0 * math.Pi * float64(highFreq))
	dt := 1.0 / float64(sampleRate)
	return &EQ3Band{
		lowGain:  lowGain,
		midGain:  midGain,
		highGain: highGain,
		lpAlpha:  float32(dt / (lpRC + dt)),
		hpAlpha:  float32(dt / (hpRC + dt)),
	}
}

func (eq *EQ3Band) Process(l, r float32) (float32, float32) {
	// Low band (LP filter)
	eq.lpL += eq.lpAlpha * (l - eq.lpL)
	eq.lpR += eq.lpAlpha * (r - eq.lpR)
	lowL, lowR := eq.lpL, eq.lpR

	// High band (HP filter)
	eq.hpL += eq.hpAlpha * (l - eq.hpL)
	eq.hpR += eq.hpAlpha * (r - eq.hpR)
	highL := l - eq.hpL
	highR := r - eq.hpR

	// Mid band (everything between)
	midL := l - lowL - highL
	midR := r - lowR - highR

	return lowL*eq.lowGain + midL*eq.midGain + highL*eq.highGain,
		lowR*eq.lowGain + midR*eq.midGain + highR*eq.highGain
}

func (eq *EQ3Band) Reset() {
	eq.lpL, eq.lpR = 0, 0
	eq.hpL, eq.hpR = 0, 0
}
