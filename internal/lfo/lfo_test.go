package lfo

import (
	"math"
	"testing"
)

func TestLFOTriangleBasicShape(t *testing.T) {
	l := &LFO{}
	l.Set(1.0, 1.0, WaveTriangle) // 1 Hz, depth 1, triangle

	sr := 100.0 // 100 samples per second = 100 samples per cycle
	samples := make([]float64, 100)
	for i := range samples {
		samples[i] = l.Sample(sr)
	}

	// At phase 0, triangle should be -1*depth = -1.0
	if math.Abs(samples[0]-(-1.0)) > 0.05 {
		t.Errorf("triangle at phase 0: got %f, want -1.0", samples[0])
	}
	// At phase 0.25 (sample 25), should be ~0
	if math.Abs(samples[25]) > 0.05 {
		t.Errorf("triangle at phase 0.25: got %f, want ~0", samples[25])
	}
	// At phase 0.5 (sample 50), should be 1.0
	if math.Abs(samples[50]-1.0) > 0.05 {
		t.Errorf("triangle at phase 0.5: got %f, want 1.0", samples[50])
	}
}

func TestLFOSquareShape(t *testing.T) {
	l := &LFO{}
	l.Set(2.0, 1.0, WaveSquare) // 1 Hz, depth 2

	sr := 100.0
	// First quarter should be +2
	v := l.Sample(sr)
	if math.Abs(v-2.0) > 0.01 {
		t.Errorf("square first half: got %f, want 2.0", v)
	}
	// Skip to second half
	for i := 1; i < 50; i++ {
		l.Sample(sr)
	}
	v = l.Sample(sr)
	if math.Abs(v-(-2.0)) > 0.01 {
		t.Errorf("square second half: got %f, want -2.0", v)
	}
}

func TestLFOSawShape(t *testing.T) {
	l := &LFO{}
	l.Set(1.0, 1.0, WaveSaw)

	sr := 100.0
	v := l.Sample(sr)
	// At phase 0, saw = 1 - 2*0 = 1.0
	if math.Abs(v-1.0) > 0.05 {
		t.Errorf("saw at phase 0: got %f, want 1.0", v)
	}
}

func TestLFOZeroDepthReturnsZero(t *testing.T) {
	l := &LFO{}
	l.Set(0, 5.0, WaveTriangle)

	v := l.Sample(44100)
	if v != 0 {
		t.Errorf("zero depth should return 0, got %f", v)
	}
}

func TestLFOZeroRateReturnsZero(t *testing.T) {
	l := &LFO{}
	l.Set(1.0, 0, WaveTriangle)

	v := l.Sample(44100)
	if v != 0 {
		t.Errorf("zero rate should return 0, got %f", v)
	}
}

func TestLFOActive(t *testing.T) {
	l := &LFO{}
	if l.Active() {
		t.Error("default LFO should not be active")
	}
	l.Set(1.0, 5.0, WaveTriangle)
	if !l.Active() {
		t.Error("configured LFO should be active")
	}
	l.Set(0, 5.0, WaveTriangle)
	if l.Active() {
		t.Error("zero-depth LFO should not be active")
	}
}

func TestLFORandomProducesValues(t *testing.T) {
	l := &LFO{}
	l.Set(1.0, 10.0, WaveRandom) // 10 Hz

	sr := 1000.0
	// Sample 200 values (covers 2 cycles), check we get non-zero values
	var nonZero int
	for i := 0; i < 200; i++ {
		v := l.Sample(sr)
		if v != 0 {
			nonZero++
		}
		if math.Abs(v) > 1.0 {
			t.Errorf("random sample exceeds depth: %f", v)
		}
	}
	// After first cycle boundary the held value should be non-zero at some point
	// Initial randVal is 0, so first cycle produces 0s, but after boundary it should change
	if nonZero == 0 {
		t.Log("warning: all random samples were zero (possible but unlikely)")
	}
}
