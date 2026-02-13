package mml

import (
	"os"
	"testing"
)

func TestConformance_DirectiveDefinitionsCaptured(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse(`#TITLE{demo};
#FPS120;
#TMODE{unit=100};
#VMODE{n88};
#SIGN{Fm};
#TABLE1{(0,8)4};
o5 cdef;`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if score.Definitions["TITLE"] != "demo" {
		t.Fatalf("missing TITLE definition")
	}
	if score.Definitions["FPS"] != "120" {
		t.Fatalf("missing FPS definition")
	}
	if score.Definitions["TMODE"] == "" || score.Definitions["VMODE"] == "" || score.Definitions["SIGN"] == "" {
		t.Fatalf("expected TMODE/VMODE/SIGN definitions captured")
	}
	if score.Definitions["TABLE1"] == "" {
		t.Fatalf("expected TABLE1 definition captured")
	}
}

func TestConformance_MacroTransposeInvocation(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse(`#A=cde;
o5 A(2);`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(score.Tracks))
	}
	notes := 0
	for _, ev := range score.Tracks[0].Events {
		if ev.Type == EventNote {
			notes++
		}
	}
	if notes != 3 {
		t.Fatalf("expected 3 expanded notes, got %d", notes)
	}
}

func TestConformance_AdvancedCommandParsing(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse(`%1,2 @q24,8 q80 x96 v12 kt2 k64
o5 l8 c&d&&e po12 *3 na1 np2 nt3 nf4 @@5;`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(score.Tracks))
	}
	var noteCount, controlCount int
	for _, ev := range score.Tracks[0].Events {
		if ev.Type == EventNote {
			noteCount++
		}
		if ev.Type == EventControl || ev.Type == EventTableEnv || ev.Type == EventModule || ev.Type == EventKeyOnDelay || ev.Type == EventTranspose || ev.Type == EventDetune {
			controlCount++
		}
	}
	if noteCount != 3 {
		t.Fatalf("expected 3 notes, got %d", noteCount)
	}
	if controlCount == 0 {
		t.Fatalf("expected control events for advanced commands")
	}
}

func TestConformance_PercentVScaleMax256Shift(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("%v0,4 o5 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	// %v0,4 => max = 256 >> 4 = 16.
	var found bool
	for _, ev := range score.Tracks[0].Events {
		if ev.Type == EventControl && ev.Command == "%v" {
			found = true
			if len(ev.Values) < 2 || ev.Values[1] != 16 {
				t.Fatalf("expected %%v max=16 (256>>4), got values %#v", ev.Values)
			}
		}
	}
	if !found {
		t.Fatalf("expected %%v control event")
	}
}

func TestConformance_PercentFTE_Parsing(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("%f1,2 %t3,4,5 %e6 o5 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	cmds := map[string][]int{}
	for _, ev := range score.Tracks[0].Events {
		if ev.Type == EventControl && (ev.Command == "%f" || ev.Command == "%t" || ev.Command == "%e") {
			cmds[ev.Command] = ev.Values
		}
	}
	if v, ok := cmds["%f"]; !ok || len(v) != 2 || v[0] != 1 || v[1] != 2 {
		t.Fatalf("expected %%f values [1,2], got %#v", cmds["%f"])
	}
	if v, ok := cmds["%t"]; !ok || len(v) != 3 || v[0] != 3 || v[1] != 4 || v[2] != 5 {
		t.Fatalf("expected %%t values [3,4,5], got %#v", cmds["%t"])
	}
	if v, ok := cmds["%e"]; !ok || len(v) != 1 || v[0] != 6 {
		t.Fatalf("expected %%e values [6], got %#v", cmds["%e"])
	}
}

func TestConformance_SCommandTwoArgs(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("s32,-128 o5 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var found bool
	for _, ev := range score.Tracks[0].Events {
		if ev.Type == EventControl && ev.Command == "s" {
			found = true
			if len(ev.Values) != 2 || ev.Values[0] != 32 || ev.Values[1] != -128 {
				t.Fatalf("expected s values [32,-128], got %#v", ev.Values)
			}
		}
	}
	if !found {
		t.Fatalf("expected s control event")
	}
}

func TestConformance_SignPlusAlias(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("#SIGN{F+m}; o4 l4 f")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	n := score.Tracks[0].Events[0]
	if n.Type != EventNote {
		t.Fatalf("expected note event")
	}
	// F+m = F#m => same as A major: F#, C#, G#
	if n.Note != 54 { // F#4
		t.Fatalf("expected F#4 (54) with F+m key sig, got %d", n.Note)
	}
}

func TestConformance_ParseRealWorldMMLTFixture(t *testing.T) {
	data, err := os.ReadFile("../../examples/mmlt.mml")
	if err != nil {
		t.Skipf("mmlt fixture not available: %v", err)
	}
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse(string(data))
	if err != nil {
		t.Fatalf("parse failed for mmlt fixture: %v", err)
	}
	if len(score.Tracks) == 0 {
		t.Fatalf("expected at least one playable track")
	}
	if score.Definitions["TITLE"] == "" {
		t.Fatalf("expected TITLE directive to be captured from mmlt")
	}
}
