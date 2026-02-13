package lfo

import "math"

// Waveform constants matching sequencer LFO waveforms.
const (
	WaveSaw      = 0
	WaveSquare   = 1
	WaveTriangle = 2
	WaveRandom   = 3
)

// LFO is a low-frequency oscillator that produces per-sample modulation.
// It is designed to be shared across all voices in an engine (global LFO).
type LFO struct {
	depth    float64 // modulation depth (units depend on context: semitones, gain factor, cutoff)
	rateHz   float64 // oscillation rate in Hz
	waveform int     // 0=saw, 1=square, 2=triangle, 3=random
	phase    float64 // current phase [0, 1)
	randVal  float64 // held random value for sample-and-hold
}

// Set configures the LFO parameters.
func (l *LFO) Set(depth, rateHz float64, waveform int) {
	l.depth = depth
	l.rateHz = rateHz
	if waveform < 0 || waveform > 3 {
		waveform = WaveTriangle
	}
	l.waveform = waveform
}

// Sample advances the LFO by one sample and returns a value in [-depth, +depth].
// Returns 0 if depth or rate is zero.
func (l *LFO) Sample(sampleRate float64) float64 {
	if l.depth == 0 || l.rateHz == 0 || sampleRate == 0 {
		return 0
	}

	// Compute waveform value from current phase
	var waveVal float64
	switch l.waveform {
	case WaveSaw:
		waveVal = 1.0 - 2.0*l.phase
	case WaveSquare:
		if l.phase < 0.5 {
			waveVal = 1.0
		} else {
			waveVal = -1.0
		}
	case WaveRandom:
		waveVal = l.randVal
	default: // WaveTriangle
		if l.phase < 0.5 {
			waveVal = 4.0*l.phase - 1.0
		} else {
			waveVal = 3.0 - 4.0*l.phase
		}
	}

	// Advance phase
	oldPhase := l.phase
	l.phase += l.rateHz / sampleRate
	for l.phase >= 1.0 {
		l.phase -= 1.0
	}

	// For random waveform, update held value at each cycle boundary
	if l.waveform == WaveRandom && l.phase < oldPhase {
		// Simple deterministic-ish random using a sine-based hash
		l.randVal = math.Sin(l.phase*12345.6789+l.randVal*67890.1234) * 2.0
		l.randVal -= math.Floor(l.randVal)     // fractional part [0,1)
		l.randVal = l.randVal*2.0 - 1.0        // map to [-1, 1)
	}

	return waveVal * l.depth
}

// Active returns true if the LFO has non-zero depth and rate.
func (l *LFO) Active() bool {
	return l.depth != 0 && l.rateHz != 0
}

// Reset zeros the LFO phase.
func (l *LFO) Reset() {
	l.phase = 0
	l.randVal = 0
}
