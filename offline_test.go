package mmlfm

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	intmml "github.com/cbegin/mmlfm-go/internal/mml"
)

func TestGoldenWAVSnapshot(t *testing.T) {
	parser := intmml.NewParser(intmml.DefaultParserConfig())
	score, err := parser.Parse("t140 o5 l8 cdefgab>c<c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	cases := []struct {
		name   string
		file   string
		render func() []float32
	}{
		{
			name: "fm",
			file: "golden_fm_short_phrase.sha256",
			render: func() []float32 {
				return RenderSamples(score, 48000, 1.2)
			},
		},
		{
			name: "chiptune",
			file: "golden_chiptune_short_phrase.sha256",
			render: func() []float32 {
				return RenderSamplesChiptune(score, 48000, 1.2)
			},
		},
		{
			name: "nesapu",
			file: "golden_nesapu_short_phrase.sha256",
			render: func() []float32 {
				return RenderSamplesNESAPU(score, 48000, 1.2)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			samples := tc.render()
			wav := EncodeWAVFloat32LE(samples, 48000, 2)
			sum := sha256.Sum256(wav)
			got := hex.EncodeToString(sum[:])
			raw, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatalf("read golden hash: %v", err)
			}
			want := strings.TrimSpace(string(raw))
			if got != want {
				t.Fatalf("golden mismatch\nwant: %s\ngot:  %s", want, got)
			}
		})
	}
}
