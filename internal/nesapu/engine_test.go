package nesapu

import "testing"

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

func TestEngineSupportsStereoPan(t *testing.T) {
	e := New(48000, DefaultParams())
	e.NoteOn(60, 127, 64, 0)
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
	if rightEnergy <= leftEnergy {
		t.Fatalf("expected right-biased signal, left=%f right=%f", leftEnergy, rightEnergy)
	}
}
