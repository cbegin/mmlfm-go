package mml

import "testing"

func TestParseNoteByNumber(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("o5 l4 n60n64n67")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	notes := []int{}
	for _, e := range tr.Events {
		if e.Type == EventNote {
			notes = append(notes, e.Note)
		}
	}
	if len(notes) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(notes))
	}
	if notes[0] != 60 || notes[1] != 64 || notes[2] != 67 {
		t.Fatalf("expected MIDI notes 60,64,67, got %v", notes)
	}
}

func TestParseGuillemetOctave(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("o5 «c »c c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	notes := []int{}
	for _, e := range tr.Events {
		if e.Type == EventNote {
			notes = append(notes, e.Note)
		}
	}
	if len(notes) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(notes))
	}
	// With OctavePolarize -1: « adds 2*(-1)=-2 to octave, » subtracts -2 so +2.
	// o5 «c -> o3 c = 36; »c -> o5 c = 60; c = 60
	if notes[0] != 36 || notes[1] != 60 || notes[2] != 60 {
		t.Fatalf("expected «c=36 »c=60 c=60, got %v", notes)
	}
}

func TestParseBasicMelody(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("t120 o5 l4 cdefgab>c;")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(score.Tracks))
	}
	tr := score.Tracks[0]
	if len(tr.Events) < 8 {
		t.Fatalf("expected at least 8 events, got %d", len(tr.Events))
	}
	if tr.EndTick <= 0 {
		t.Fatalf("expected positive end tick")
	}
}

func TestParseLoopAlternate(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("o4 l8 [cdef|gab]2")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	noteCount := 0
	for _, e := range tr.Events {
		if e.Type == EventNote {
			noteCount++
		}
	}
	if noteCount != 6 {
		t.Fatalf("expected 6 note events, got %d", noteCount)
	}
}

func TestParseRepeatAllMarker(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("l8 cdef$gab")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	if tr.LoopTick <= 0 {
		t.Fatalf("expected loop tick to be set, got %d", tr.LoopTick)
	}
	if tr.LoopIndex <= 0 {
		t.Fatalf("expected loop index to be set, got %d", tr.LoopIndex)
	}
}

func TestParseTransposeAndQuantize(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("#QUANT100; o4 l4 k2 q50 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	ev := Event{}
	for _, cand := range tr.Events {
		if cand.Type == EventNote {
			ev = cand
			break
		}
	}
	if ev.Type != EventNote {
		t.Fatalf("expected note event")
	}
	if ev.Note != 48 {
		t.Fatalf("expected note 48 with fine detune storage, got %d", ev.Note)
	}
	if ev.Duration != 240 {
		t.Fatalf("expected gated duration 240, got %d", ev.Duration)
	}
	if tr.EndTick != 480 {
		t.Fatalf("expected timeline duration 480, got %d", tr.EndTick)
	}
}

func TestParseProgramPanAndMultitrack(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("o4 p-50 @3 c, o5 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(score.Tracks))
	}
	first := score.Tracks[0]
	second := score.Tracks[1]
	if len(first.Events) < 3 {
		t.Fatalf("expected control + note events on first track, got %d", len(first.Events))
	}
	var gotNote bool
	for _, ev := range first.Events {
		if ev.Type != EventNote {
			continue
		}
		gotNote = true
		if ev.Program != 3 {
			t.Fatalf("expected program=3, got %d", ev.Program)
		}
		if ev.Pan != -50 {
			t.Fatalf("expected pan=-50, got %d", ev.Pan)
		}
	}
	if !gotNote {
		t.Fatalf("expected note event on first track")
	}
	if len(second.Events) != 1 || second.Events[0].Type != EventNote {
		t.Fatalf("expected one note event on second track")
	}
}

func TestOctaveShiftClampedToParserRange(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("o0<<<<<<c, o9>>>>>>b")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(score.Tracks))
	}
	low := score.Tracks[0].Events[0]
	high := score.Tracks[1].Events[0]
	if low.Type != EventNote || high.Type != EventNote {
		t.Fatalf("expected notes on both tracks")
	}
	if low.Note != 0 {
		t.Fatalf("expected low note clamped to 0, got %d", low.Note)
	}
	if high.Note != 119 {
		t.Fatalf("expected high note at max octave B (119), got %d", high.Note)
	}
}

func TestParseSectionedTracksWithGlobalPrelude(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("t111; o5 l4 c, o5 l4 e; o4 l4 g;")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(score.Tracks))
	}
	for i, tr := range score.Tracks {
		if len(tr.Events) < 2 {
			t.Fatalf("expected tempo + note events on track %d, got %d", i, len(tr.Events))
		}
		if tr.Events[0].Type != EventTempo || tr.Events[0].Value != 111 {
			t.Fatalf("expected leading tempo=111 on track %d", i)
		}
	}
}

func TestParseWithLineAndBlockComments(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("t120 /* intro */ o5 l4 c // first\n d;")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(score.Tracks))
	}
	tr := score.Tracks[0]
	noteCount := 0
	for _, ev := range tr.Events {
		if ev.Type == EventNote {
			noteCount++
		}
	}
	if noteCount != 2 {
		t.Fatalf("expected 2 notes after comment stripping, got %d", noteCount)
	}
}

func TestParseHashDirectivesAndMacroExpansion(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse(`#TITLE{demo};
#A= c4 d4;
#X= c8;
#B= X X;
t111;
@v48 A;
@v48 B;`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(score.Tracks))
	}
	for i, tr := range score.Tracks {
		if len(tr.Events) == 0 {
			t.Fatalf("expected non-empty events on track %d", i)
		}
		if tr.Events[0].Type != EventTempo || tr.Events[0].Value != 111 {
			t.Fatalf("expected tempo prelude on track %d", i)
		}
	}
}

func TestParseMacroRangeAndAppend(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("#A-C=c; #AB+=d; l8ABCD")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(score.Tracks))
	}
	tr := score.Tracks[0]
	noteCount := 0
	for _, ev := range tr.Events {
		if ev.Type == EventNote {
			noteCount++
		}
	}
	// A,B are "cd", C is "c", D is undefined.
	if noteCount != 6 {
		t.Fatalf("expected 6 expanded notes, got %d", noteCount)
	}
}

func TestParseMacroStaticAndDynamicModes(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	staticScore, err := p.Parse("#MACRO{static}; #A=cde; #B=Afg; B; #A=gfe; B;")
	if err != nil {
		t.Fatalf("parse failed(static): %v", err)
	}
	dynamicScore, err := p.Parse("#MACRO{dynamic}; #A=cde; #B=Afg; B; #A=gfe; B;")
	if err != nil {
		t.Fatalf("parse failed(dynamic): %v", err)
	}
	if len(staticScore.Tracks) != 2 || len(dynamicScore.Tracks) != 2 {
		t.Fatalf("expected 2 tracks for both static and dynamic runs")
	}
	staticSecond := firstNote(staticScore.Tracks[1])
	dynamicSecond := firstNote(dynamicScore.Tracks[1])
	if staticSecond != 60 {
		t.Fatalf("expected static second track to begin at C5(60), got %d", staticSecond)
	}
	if dynamicSecond != 67 {
		t.Fatalf("expected dynamic second track to begin at G5(67), got %d", dynamicSecond)
	}
}

func TestParseMacroInvocationWithTransposeArgument(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("#A=c; o5 A(2);")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(score.Tracks))
	}
	n := firstNote(score.Tracks[0])
	if n != 62 {
		t.Fatalf("expected transposed note D5(62), got %d", n)
	}
}

func TestParseRevAndEndDirectives(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("#REV; o4<c; #END; o4c;")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(score.Tracks))
	}
	n := firstNote(score.Tracks[0])
	if n != 60 {
		t.Fatalf("expected octave-reversed result o4>c => note 60, got %d", n)
	}
}

func firstNote(tr Track) int {
	for _, ev := range tr.Events {
		if ev.Type == EventNote {
			return ev.Note
		}
	}
	return -1
}

func TestParseTMODEUnitTempo(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("#TMODE{unit=100}; t13755 o5 c;")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(score.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(score.Tracks))
	}
	tr := score.Tracks[0]
	if len(tr.Events) == 0 || tr.Events[0].Type != EventTempo {
		t.Fatalf("expected leading tempo event")
	}
	if tr.Events[0].Value < 137 || tr.Events[0].Value > 138 {
		t.Fatalf("expected TMODE unit tempo around 137-138 BPM, got %d", tr.Events[0].Value)
	}
}

func TestParseAtQUses192ndNoteUnits(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("l4 @q48,12 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	n := tr.Events[len(tr.Events)-1]
	if n.Type != EventNote {
		t.Fatalf("expected note event")
	}
	// resolution=1920 => @q48 is a quarter note = 480 ticks.
	if n.GateTick != 480 {
		t.Fatalf("expected gate tick 480, got %d", n.GateTick)
	}
	if n.Delay != 120 {
		t.Fatalf("expected delay 120 ticks, got %d", n.Delay)
	}
	// @q subtracts from q-gated duration. Default q6/8 => gated=360.
	// noteDur = 360 - 480 - 120 = clamped to 0.
	if n.Duration != 0 {
		t.Fatalf("expected duration 0 (subtracted past zero), got %d", n.Duration)
	}
}

func TestParseAtQSubtractsFromGatedDuration(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	// q8 = full gate, @q24 = 240 ticks at resolution 1920.
	// l4 = 480 ticks, q8 gated = 480, noteDur = 480 - 240 = 240.
	score, err := p.Parse("q8 @q24 l4 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	n := tr.Events[len(tr.Events)-1]
	if n.Type != EventNote {
		t.Fatalf("expected note event")
	}
	if n.Duration != 240 {
		t.Fatalf("expected duration 240 (480-240), got %d", n.Duration)
	}
}

func TestParseAtQZeroDisablesAbsoluteGateOverride(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("l4 q8 @q0 c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	n := tr.Events[len(tr.Events)-1]
	if n.Type != EventNote {
		t.Fatalf("expected note event")
	}
	if n.GateTick != -1 {
		t.Fatalf("expected @q0 to disable absolute gate override, got %d", n.GateTick)
	}
	if n.Duration <= 0 {
		t.Fatalf("expected positive duration from q gate, got %d", n.Duration)
	}
}

func TestParseVolumeShiftAndRevVolume(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("v10(2)c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if score.Tracks[0].Events[0].Type != EventVolume || score.Tracks[0].Events[0].Value != 10 {
		t.Fatalf("expected initial v10 event")
	}
	if score.Tracks[0].Events[1].Type != EventVolume || score.Tracks[0].Events[1].Value != 12 {
		t.Fatalf("expected '(' to increase volume to 12")
	}
	score2, err := p.Parse("#REV{volume}; v10(2)c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if score2.Tracks[0].Events[1].Value != 8 {
		t.Fatalf("expected reversed volume shift to decrease to 8, got %d", score2.Tracks[0].Events[1].Value)
	}
}

func TestParseSignAppliesImplicitAccidentals(t *testing.T) {
	p := NewParser(DefaultParserConfig())
	score, err := p.Parse("#SIGN{G}; o4 l4 f f+")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	tr := score.Tracks[0]
	if len(tr.Events) < 2 {
		t.Fatalf("expected 2 notes")
	}
	if tr.Events[0].Note != 54 { // F#4
		t.Fatalf("expected implicit F#(54), got %d", tr.Events[0].Note)
	}
	if tr.Events[1].Note != 54 {
		t.Fatalf("expected explicit F#(54), got %d", tr.Events[1].Note)
	}
}
