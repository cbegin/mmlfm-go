package effects

import (
	"math"
	"testing"
)

func TestDelayProducesOutput(t *testing.T) {
	d := NewDelay(44100, 100, 0.5, 0, 0.5)
	// Feed a pulse and check delayed output appears
	d.Process(1.0, 1.0)
	for i := 0; i < 4409; i++ { // ~100ms at 44100Hz
		d.Process(0, 0)
	}
	l, r := d.Process(0, 0)
	if math.Abs(float64(l)) < 0.01 || math.Abs(float64(r)) < 0.01 {
		t.Errorf("expected delayed output, got l=%f r=%f", l, r)
	}
}

func TestReverbProducesOutput(t *testing.T) {
	r := NewReverb(44100, 0.5, 0.7, 0.5)
	// Feed impulse
	r.Process(1.0, 1.0)
	// After some samples, reverb tail should be present
	var maxOut float32
	for i := 0; i < 10000; i++ {
		l, _ := r.Process(0, 0)
		if l > maxOut {
			maxOut = l
		}
	}
	if maxOut < 0.001 {
		t.Error("expected reverb tail")
	}
}

func TestDistortionClips(t *testing.T) {
	d := NewDistortion(44100, 10, 0.5, 0)
	l, r := d.Process(0.5, 0.5)
	// With high pregain, tanh should compress the signal
	if math.Abs(float64(l)) > 1.0 || math.Abs(float64(r)) > 1.0 {
		t.Error("distortion output should be bounded")
	}
	if math.Abs(float64(l)) < 0.01 {
		t.Error("expected non-zero distortion output")
	}
}

func TestChainAppliesEffectsInOrder(t *testing.T) {
	c := NewChain(
		NewDistortion(44100, 2, 1, 0),
		NewDelay(44100, 10, 0, 0, 0.5),
	)
	l, r := c.Process(0.5, 0.5)
	if l == 0 || r == 0 {
		t.Error("chain should produce output")
	}
}

func TestEQ3BandUnityGain(t *testing.T) {
	eq := NewEQ3Band(44100, 1.0, 1.0, 1.0, 300, 3000)
	// With unity gains, output should approximate input after warmup
	for i := 0; i < 1000; i++ {
		eq.Process(0.5, 0.5)
	}
	l, r := eq.Process(0.5, 0.5)
	if math.Abs(float64(l)-0.5) > 0.1 || math.Abs(float64(r)-0.5) > 0.1 {
		t.Errorf("expected ~0.5 with unity gains, got l=%f r=%f", l, r)
	}
}

func TestCompressorReducesLoud(t *testing.T) {
	c := NewCompressor(44100, -10, 4, 1, 50, 0)
	// Feed loud signal repeatedly to let envelope settle
	var out float32
	for i := 0; i < 1000; i++ {
		out, _ = c.Process(1.0, 1.0)
	}
	if out >= 1.0 {
		t.Errorf("compressor should reduce loud signals, got %f", out)
	}
}
