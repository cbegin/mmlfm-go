package sequencer

import (
	"testing"

	"github.com/cbegin/mmlfm-go/internal/fm"
	"github.com/cbegin/mmlfm-go/internal/mml"
)

type countingEngine struct {
	noteOnCount int
	noteOffs    []int
	nextID      int
	pans        []int
}

func (e *countingEngine) NoteOn(note int, velocity int, pan int, program int) int {
	e.noteOnCount++
	e.pans = append(e.pans, pan)
	id := e.nextID
	e.nextID++
	return id
}
func (e *countingEngine) NoteOff(id int)                 { e.noteOffs = append(e.noteOffs, id) }
func (e *countingEngine) RenderFrame() (float32, float32) { return 0, 0 }
func (e *countingEngine) SetMasterGain(gain float64)      {}
func (e *countingEngine) ActiveVoiceCount() int           { return 0 }
func (e *countingEngine) SetFilterType(int)               {}
func (e *countingEngine) SetNoteOnPhase(int)              {}
func (e *countingEngine) SetPortamento(from int, frames int)          {}
func (e *countingEngine) SetPitchLFO(float64, float64, int)           {}
func (e *countingEngine) SetAmpLFO(float64, float64, int)             {}
func (e *countingEngine) SetFilterLFO(float64, float64, int)          {}

func TestSequencerProcessesFrames(t *testing.T) {
	parser := mml.NewParser(mml.DefaultParserConfig())
	score, err := parser.Parse("t120 o5 l8 cdefgab>c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	engine := fm.New(48000, fm.DefaultParams())
	seq := New(score, engine, 48000)

	buf := make([]float32, 48000/4*2)
	seq.Process(buf)

	var energy float64
	for _, s := range buf {
		if s < 0 {
			energy -= float64(s)
		} else {
			energy += float64(s)
		}
	}
	if energy == 0 {
		t.Fatalf("expected non-zero audio energy")
	}
}

func TestSequencerLoopsWholeScoreWhenEnabled(t *testing.T) {
	parser := mml.NewParser(mml.DefaultParserConfig())
	score, err := parser.Parse("t120 o5 l4 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	engine := &countingEngine{}
	seq := NewWithOptions(score, engine, 48000, Options{LoopWholeScore: true})
	// 2 seconds at 120 BPM with l4 notes should retrigger multiple times.
	buf := make([]float32, 48000*2*2)
	seq.Process(buf)
	if engine.noteOnCount < 2 {
		t.Fatalf("expected loop retriggers, got %d note-ons", engine.noteOnCount)
	}
}

func TestSequencerSlurClosesPreviousVoiceAtBoundary(t *testing.T) {
	parser := mml.NewParser(mml.DefaultParserConfig())
	score, err := parser.Parse("t120 o5 l8 q8 c&d")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	engine := &countingEngine{}
	seq := NewWithOptions(score, engine, 48000, Options{LoopWholeScore: false})
	buf := make([]float32, 48000*2*2)
	seq.Process(buf)
	if engine.noteOnCount < 2 {
		t.Fatalf("expected two note-ons, got %d", engine.noteOnCount)
	}
	if len(engine.noteOffs) < 2 {
		t.Fatalf("expected boundary close + final release note-offs, got %d", len(engine.noteOffs))
	}
}

func TestSequencerMaskCanIgnorePan(t *testing.T) {
	parser := mml.NewParser(mml.DefaultParserConfig())
	score, err := parser.Parse("@mask2 p8 c p0 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	engine := &countingEngine{}
	seq := NewWithOptions(score, engine, 48000, Options{LoopWholeScore: false})
	buf := make([]float32, 48000*2)
	seq.Process(buf)
	if len(engine.pans) < 2 {
		t.Fatalf("expected 2 note-ons")
	}
	if engine.pans[0] != 0 || engine.pans[1] != 0 {
		t.Fatalf("expected pan updates masked, got pans %#v", engine.pans)
	}
}

func TestTableLoopPointSemantics(t *testing.T) {
	defs := map[string]string{
		"TABLE0": "#TABLE0{1,2|3,4}",
	}
	tables := parseTableDefinitions(defs)
	td := tables[0]
	if len(td.values) != 4 {
		t.Fatalf("expected 4 values, got %d: %#v", len(td.values), td.values)
	}
	if td.loopStart != 2 {
		t.Fatalf("expected loopStart=2, got %d", td.loopStart)
	}
	if td.values[0] != 1 || td.values[1] != 2 || td.values[2] != 3 || td.values[3] != 4 {
		t.Fatalf("unexpected values: %#v", td.values)
	}
}

func TestTableTrailingOps(t *testing.T) {
	defs := map[string]string{
		"TABLE0": "#TABLE0{1,2,3}2*3+1",
	}
	tables := parseTableDefinitions(defs)
	td := tables[0]
	// stretch=2: each entry repeated twice => [1,1,2,2,3,3]
	// *3: => [3,3,6,6,9,9]
	// +1: => [4,4,7,7,10,10]
	if len(td.values) != 6 {
		t.Fatalf("expected 6 values, got %d: %#v", len(td.values), td.values)
	}
	expected := []int{4, 4, 7, 7, 10, 10}
	for i, v := range expected {
		if td.values[i] != v {
			t.Fatalf("value[%d]: expected %d, got %d (all: %#v)", i, v, td.values[i], td.values)
		}
	}
}

func TestSequencerFPSRuntimeApplication(t *testing.T) {
	parser := mml.NewParser(mml.DefaultParserConfig())
	score, err := parser.Parse("@fps60 o5 l4 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	engine := &countingEngine{}
	seq := NewWithOptions(score, engine, 48000, Options{})
	// Just verify it processes without error.
	buf := make([]float32, 48000*2)
	seq.Process(buf)
	if engine.noteOnCount < 1 {
		t.Fatalf("expected at least 1 note-on, got %d", engine.noteOnCount)
	}
}

func TestParseTableDefinitionsInterpolation(t *testing.T) {
	defs := map[string]string{
		"TABLE1": "#TABLE1{(0,8)4}",
		"TABLE2": "#TABLE2{[1,2]3}",
	}
	tables := parseTableDefinitions(defs)
	got1 := tables[1].values
	if len(got1) != 4 || got1[0] != 0 || got1[3] != 6 {
		t.Fatalf("unexpected TABLE1 values: %#v", got1)
	}
	got2 := tables[2].values
	if len(got2) != 6 {
		t.Fatalf("unexpected TABLE2 values: %#v", got2)
	}
}
