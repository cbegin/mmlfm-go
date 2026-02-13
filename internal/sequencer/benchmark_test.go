package sequencer

import (
	"testing"

	"github.com/cbegin/mmlfm-go/internal/fm"
	"github.com/cbegin/mmlfm-go/internal/mml"
)

func BenchmarkSequencerProcess(b *testing.B) {
	parser := mml.NewParser(mml.DefaultParserConfig())
	score, err := parser.Parse("t150 o5 l16 cdefgab>c<cdefgab")
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	buf := make([]float32, 2048*2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine := fm.New(48000, fm.DefaultParams())
		seq := New(score, engine, 48000)
		seq.Process(buf)
	}
}
