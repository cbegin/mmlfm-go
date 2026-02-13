package fm

import (
	"math"
	"testing"
)

func TestEngineGeneratesSignal(t *testing.T) {
	e := New(48000, DefaultParams())
	id := e.NoteOn(60, 100, 0, 0)
	if id < 0 {
		t.Fatalf("invalid voice id")
	}

	var nonZero bool
	for i := 0; i < 5000; i++ {
		l, r := e.RenderFrame()
		if l != 0 || r != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Fatalf("expected non-zero output")
	}
	e.NoteOff(id)
}

func TestPanExtremesBiasChannels(t *testing.T) {
	e := New(48000, DefaultParams())
	e.NoteOn(60, 127, -64, 0)
	var leftEnergy, rightEnergy float64
	for i := 0; i < 4096; i++ {
		l, r := e.RenderFrame()
		if l < 0 {
			leftEnergy -= float64(l)
		} else {
			leftEnergy += float64(l)
		}
		if r < 0 {
			rightEnergy -= float64(r)
		} else {
			rightEnergy += float64(r)
		}
	}
	if leftEnergy <= rightEnergy {
		t.Fatalf("expected left-biased signal, left=%f right=%f", leftEnergy, rightEnergy)
	}
}

func TestMultiOperatorAlgorithms(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opCount int
		alg     int
	}{
		{"1-op", 1, 0},
		{"2-op serial", 2, 0},
		{"2-op parallel", 2, 1},
		{"3-op cascade", 3, 0},
		{"3-op all-parallel", 3, 3},
		{"4-op cascade", 4, 0},
		{"4-op all-parallel", 4, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := New(48000, DefaultParams())
			e.SetOperatorCount(tc.opCount)
			e.SetAlgorithm(tc.alg)
			id := e.NoteOn(60, 100, 0, 0)
			var maxAbs float64
			for i := 0; i < 2000; i++ {
				l, r := e.RenderFrame()
				if a := math.Abs(float64(l)); a > maxAbs {
					maxAbs = a
				}
				if a := math.Abs(float64(r)); a > maxAbs {
					maxAbs = a
				}
			}
			if maxAbs < 0.001 {
				t.Errorf("expected non-zero output for %s", tc.name)
			}
			e.NoteOff(id)
		})
	}
}

func TestWaveformTypes(t *testing.T) {
	for wf := 0; wf < 8; wf++ {
		t.Run("waveform_"+string(rune('0'+wf)), func(t *testing.T) {
			e := New(48000, DefaultParams())
			e.NoteOn(60, 100, 0, wf) // program % 8 = waveform
			var maxAbs float64
			for i := 0; i < 1000; i++ {
				l, _ := e.RenderFrame()
				if a := math.Abs(float64(l)); a > maxAbs {
					maxAbs = a
				}
			}
			if maxAbs < 0.001 {
				t.Errorf("waveform %d produced no output", wf)
			}
		})
	}
}

func TestFilterTypes(t *testing.T) {
	for _, ft := range []int{0, 1, 2} {
		e := New(48000, DefaultParams())
		e.SetFilterType(ft)
		e.NoteOn(60, 100, 0, 0)
		var maxAbs float64
		for i := 0; i < 2000; i++ {
			l, _ := e.RenderFrame()
			if a := math.Abs(float64(l)); a > maxAbs {
				maxAbs = a
			}
		}
		if maxAbs < 0.001 {
			t.Errorf("filter type %d produced no output", ft)
		}
	}
}

func TestFeedbackProducesDifferentOutput(t *testing.T) {
	// Without feedback
	e1 := New(48000, DefaultParams())
	e1.SetFeedback(0)
	e1.NoteOn(60, 100, 0, 0)
	var sum1 float64
	for i := 0; i < 1000; i++ {
		l, _ := e1.RenderFrame()
		sum1 += float64(l)
	}

	// With feedback
	e2 := New(48000, DefaultParams())
	e2.SetFeedback(0.7)
	e2.NoteOn(60, 100, 0, 0)
	var sum2 float64
	for i := 0; i < 1000; i++ {
		l, _ := e2.RenderFrame()
		sum2 += float64(l)
	}

	if sum1 == sum2 {
		t.Error("feedback should change the output")
	}
}
