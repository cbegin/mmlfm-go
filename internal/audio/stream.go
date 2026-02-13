package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	ebitaudio "github.com/hajimehoshi/ebiten/v2/audio"
)

type SampleSource interface {
	Process(dst []float32)
}

// FinishingSource is a SampleSource that can signal when playback has ended.
// When Finished returns true, the stream will return io.EOF on the next Read.
type FinishingSource interface {
	SampleSource
	Finished() bool
}

type StreamReader struct {
	mu     sync.Mutex
	source SampleSource
	buf    []float32
}

func NewStreamReader(source SampleSource) *StreamReader {
	return &StreamReader{source: source}
}

func (r *StreamReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	frames := len(p) / 8
	if frames == 0 {
		return 0, nil
	}
	need := frames * 2
	if cap(r.buf) < need {
		r.buf = make([]float32, need)
	}
	r.buf = r.buf[:need]
	r.source.Process(r.buf)
	for i := 0; i < need; i++ {
		u := math.Float32bits(r.buf[i])
		binary.LittleEndian.PutUint32(p[i*4:], u)
	}
	n := frames * 8
	if fs, ok := r.source.(FinishingSource); ok && fs.Finished() {
		return n, io.EOF
	}
	return n, nil
}

func (r *StreamReader) Close() error { return nil }

type Player struct {
	player *ebitaudio.Player
	reader io.ReadCloser
}

var (
	audioContextOnce sync.Once
	audioContext     *ebitaudio.Context
	audioContextErr  error
	audioSampleRate  int
)

func sharedAudioContext(sampleRate int) (*ebitaudio.Context, error) {
	audioContextOnce.Do(func() {
		audioSampleRate = sampleRate
		audioContext = ebitaudio.NewContext(sampleRate)
	})
	if audioContextErr != nil {
		return nil, audioContextErr
	}
	if audioSampleRate != sampleRate {
		return nil, fmt.Errorf("audio context already initialized at %d Hz (requested %d Hz)", audioSampleRate, sampleRate)
	}
	return audioContext, nil
}

func NewPlayer(sampleRate int, source SampleSource) (*Player, error) {
	ctx, err := sharedAudioContext(sampleRate)
	if err != nil {
		return nil, err
	}
	reader := NewStreamReader(source)
	pl, err := ctx.NewPlayerF32(reader)
	if err != nil {
		return nil, err
	}
	return &Player{
		player: pl,
		reader: reader,
	}, nil
}

func (p *Player) Play()  { p.player.Play() }
func (p *Player) Pause() { p.player.Pause() }
func (p *Player) IsPlaying() bool {
	return p.player.IsPlaying()
}

// Position returns the current playback position (what the listener actually hears).
func (p *Player) Position() time.Duration {
	return p.player.Position()
}

func (p *Player) Stop() error {
	p.player.Pause()
	p.player.Close()
	return p.reader.Close()
}
