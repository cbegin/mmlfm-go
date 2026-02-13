package mmlfm

import (
	"encoding/binary"
	"math"

	intchip "github.com/cbegin/mmlfm-go/internal/chiptune"
	intfm "github.com/cbegin/mmlfm-go/internal/fm"
	intmml "github.com/cbegin/mmlfm-go/internal/mml"
	intnes "github.com/cbegin/mmlfm-go/internal/nesapu"
	intwt "github.com/cbegin/mmlfm-go/internal/wavetable"
	intseq "github.com/cbegin/mmlfm-go/internal/sequencer"
)

func RenderSamples(score *intmml.Score, sampleRate int, seconds float64) []float32 {
	engine := intfm.New(sampleRate, intfm.DefaultParams())
	seq := intseq.New(score, engine, sampleRate)
	frames := int(float64(sampleRate) * seconds)
	out := make([]float32, frames*2)
	seq.Process(out)
	return out
}

func RenderSamplesChiptune(score *intmml.Score, sampleRate int, seconds float64) []float32 {
	engine := intchip.New(sampleRate, intchip.DefaultParams())
	seq := intseq.New(score, engine, sampleRate)
	frames := int(float64(sampleRate) * seconds)
	out := make([]float32, frames*2)
	seq.Process(out)
	return out
}

func RenderSamplesNESAPU(score *intmml.Score, sampleRate int, seconds float64) []float32 {
	engine := intnes.New(sampleRate, intnes.DefaultParams())
	seq := intseq.New(score, engine, sampleRate)
	frames := int(float64(sampleRate) * seconds)
	out := make([]float32, frames*2)
	seq.Process(out)
	return out
}

func RenderSamplesWavetable(score *intmml.Score, sampleRate int, seconds float64) []float32 {
	engine := intwt.New(sampleRate, intwt.DefaultParams())
	seq := intseq.New(score, engine, sampleRate)
	frames := int(float64(sampleRate) * seconds)
	out := make([]float32, frames*2)
	seq.Process(out)
	return out
}

func EncodeWAVFloat32LE(samples []float32, sampleRate int, channels int) []byte {
	dataSize := len(samples) * 4
	byteRate := sampleRate * channels * 4
	blockAlign := channels * 4
	chunkSize := 36 + dataSize
	out := make([]byte, 44+dataSize)
	copy(out[0:], []byte("RIFF"))
	binary.LittleEndian.PutUint32(out[4:], uint32(chunkSize))
	copy(out[8:], []byte("WAVE"))
	copy(out[12:], []byte("fmt "))
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 3)
	binary.LittleEndian.PutUint16(out[22:], uint16(channels))
	binary.LittleEndian.PutUint32(out[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(out[28:], uint32(byteRate))
	binary.LittleEndian.PutUint16(out[32:], uint16(blockAlign))
	binary.LittleEndian.PutUint16(out[34:], 32)
	copy(out[36:], []byte("data"))
	binary.LittleEndian.PutUint32(out[40:], uint32(dataSize))
	for i, s := range samples {
		binary.LittleEndian.PutUint32(out[44+i*4:], math.Float32bits(s))
	}
	return out
}
