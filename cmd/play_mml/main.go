package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/cbegin/mmlfm-go"
)

const defaultMML = "e g b d f a" // spaces prevent "b" from being parsed as flat accidental

func main() {
	var (
		sampleRate = flag.Int("sample-rate", 48000, "output sample rate")
		engineName = flag.String("engine", "fm", "synth engine: fm|chiptune|nesapu|wavetable")
		loop       = flag.Bool("loop", false, "loop playback; use with -loops to count then stop")
		loops      = flag.Int("loops", 3, "when -loop, stop after N loops (0 = loop forever)")
		mmlPath    = flag.String("file", "", "path to an MML file")
		mmlInline  = flag.String("mml", "", "inline MML string")
		volume     = flag.Float64("volume", 1.0, "master volume scalar")
		octave     = flag.Int("octave", 0, "master octave shift (-4..+4)")
	)
	flag.Parse()

	mmlText, err := resolveMMLInput(*mmlPath, *mmlInline)
	if err != nil {
		log.Fatal(err)
	}

	mode, err := parseSynthMode(*engineName)
	if err != nil {
		log.Fatal(err)
	}
	pl, err := mmlfm.NewPlayer(*sampleRate, mmlfm.WithSynthMode(mode), mmlfm.WithLoopPlayback(*loop))
	if err != nil {
		log.Fatal(err)
	}
	pl.SetMasterVolume(*volume)
	pl.SetTranspose(*octave)
	ch := pl.Watch()
	if err := pl.PlayMML(mmlText); err != nil {
		log.Fatal(err)
	}
	loopCount := 0
	for event := range ch {
		switch event.Kind {
		case mmlfm.EventPlaybackEnded:
			fmt.Println("playback completed")
			goto done
		case mmlfm.EventLoopCompleted:
			loopCount++
			fmt.Printf("loop %d completed\n", loopCount)
			if *loop && *loops > 0 && loopCount >= *loops {
				pl.Stop()
			}
		case mmlfm.EventTrigger:
			fmt.Printf("trigger %d (on=%d off=%d)\n", event.TriggerID, event.NoteOnType, event.NoteOffType)
		}
	}
done:
	pl.Wait()
}

func resolveMMLInput(path string, inline string) (string, error) {
	if strings.TrimSpace(inline) != "" {
		return inline, nil
	}
	if strings.TrimSpace(path) != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return defaultMML, nil
}

func parseSynthMode(name string) (mmlfm.SynthMode, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "fm":
		return mmlfm.SynthModeFM, nil
	case "chiptune":
		return mmlfm.SynthModeChiptune, nil
	case "nesapu":
		return mmlfm.SynthModeNESAPU, nil
	case "wavetable":
		return mmlfm.SynthModeWavetable, nil
	default:
		return "", fmt.Errorf("invalid -engine %q (expected fm|chiptune|nesapu|wavetable)", name)
	}
}
