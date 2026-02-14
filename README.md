# mmlfm-go

[![Go Reference](https://pkg.go.dev/badge/github.com/cbegin/mmlfm-go.svg)](https://pkg.go.dev/github.com/cbegin/mmlfm-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/cbegin/mmlfm-go)](https://goreportcard.com/report/github.com/cbegin/mmlfm-go)

MML (Music Macro Language) playback library in Go with realtime audio output via Ebitengine. Implements a compatible MML subset with four synthesis engines, an effects pipeline, and comprehensive MML support.

## Features

- **Realtime playback** — Low-latency audio through Ebitengine/oto
- **Four synth engines** — FM, chiptune, NES APU, and wavetable
- **Effects pipeline** — Delay, reverb, chorus, distortion, EQ, compressor via `#EFFECT`
- **Rich MML support** — Notes, rests, tempo, octave, loops, macros, pan, volume, programs, table envelopes, LFO modulation, portamento, pitch slide, filter types
- **Offline rendering** — Render to `[]float32` or export WAV (all 4 engines)
- **Event-driven API** — Watch loop completion, playback end, and `%t`/`%e` trigger events
- **Deterministic** — Golden-file snapshot tests for engine regression

## Installation

```bash
go get github.com/cbegin/mmlfm-go
```

## Quick Start

```go
package main

import (
	"log"

	"github.com/cbegin/mmlfm-go"
)

func main() {
	pl, err := mmlfm.NewPlayer(48000, mmlfm.WithSynthMode(mmlfm.SynthModeFM), mmlfm.WithLoopPlayback(false))
	if err != nil {
		log.Fatal(err)
	}
	if err := pl.PlayMML("t140 o5 l8 cdefgab>c"); err != nil {
		log.Fatal(err)
	}
	pl.Wait() // blocks until playback ends
}
```

## Public API

| Method / Function                                                                                                | Description                                       |
| ---------------------------------------------------------------------------------------------------------------- | ------------------------------------------------- |
| `NewPlayer(sampleRate int, opts ...PlayerOption) (*Player, error)`                                               | Create a playback engine                          |
| `WithSynthMode(mode SynthMode) PlayerOption`                                                                     | Choose FM, chiptune, NES APU, or wavetable engine |
| `WithLoopPlayback(enabled bool) PlayerOption`                                                                    | Loop score until `Stop()` (default: true)         |
| `(*Player).PlayMML(mml string) error`                                                                            | Start playing MML                                 |
| `(*Player).Pause()` / `(*Player).Resume()`                                                                       | Pause and resume                                  |
| `(*Player).Stop() error`                                                                                         | Stop playback                                     |
| `(*Player).Wait()`                                                                                               | Block until playback ends                         |
| `(*Player).Watch() <-chan PlaybackEvent`                                                                         | Receive loop/end/trigger events                   |
| `(*Player).SetMasterVolume(v float64)`                                                                           | Linear amplitude (1.0 = unity)                    |
| `(*Player).SetMasterVolumeDB(db float64)`                                                                        | dB scaling (e.g. -6 ≈ half amplitude)             |
| `Compile(mmlText string) (*Score, error)`                                                                        | Parse MML to Score (for offline render)           |
| `RenderSamples(...)` / `RenderSamplesChiptune(...)` / `RenderSamplesNESAPU(...)` / `RenderSamplesWavetable(...)` | Offline render to samples                         |
| `EncodeWAVFloat32LE(...)`                                                                                        | Export WAV bytes                                  |

## Engine Modes

| Mode      | Constant             | Description                                        |
| --------- | -------------------- | -------------------------------------------------- |
| FM        | `SynthModeFM`        | Multi-operator FM synthesis (1–4 ops, 8 waveforms) |
| Chiptune  | `SynthModeChiptune`  | Retro chip-style (pulse / triangle / noise)        |
| NES APU   | `SynthModeNESAPU`    | Hardware-style 4-slot: 2 pulse, triangle, noise    |
| Wavetable | `SynthModeWavetable` | Custom waveforms via `#WAVB` or `SetWavetable()`   |

## Volume Control

Volume can be changed at runtime:

```go
pl.SetMasterVolume(0.7)   // linear scalar
pl.SetMasterVolumeDB(-6)  // approximately half amplitude
```

Negative values clamp to `0`. Updates are safe while audio is playing.

## Playback Control

Prefer `Wait()` and `Watch()` over sleeps and manual `Stop()`:

- **Wait** — blocks until playback ends (use when looping is disabled)
- **Watch** — receive events as they occur (loop completions, playback end, `%t`/`%e` triggers)

Disable looping to play once:

```go
pl, _ := mmlfm.NewPlayer(48000, mmlfm.WithLoopPlayback(false))
pl.PlayMML(mml)
pl.Wait() // blocks until playback ends
```

## Playback Events

Listen for loop, end, and trigger events via `Watch()`:

```go
ch := pl.Watch()
pl.PlayMML(mml)
for event := range ch {
	switch event.Kind {
	case mmlfm.EventLoopCompleted:
		// one loop finished
	case mmlfm.EventPlaybackEnded:
		return // non-looping playback finished
	case mmlfm.EventTrigger:
		// %t or %e fired: event.TriggerID, event.NoteOnType, event.NoteOffType
	}
}
```

Call `Watch()` before `Play()` or `PlayMML()`.

## Apps

### play_mml (CLI)

The `play_mml` CLI plays MML from a file or inline string:

```bash
# Default (inline melody, FM engine)
go run ./cmd/play_mml

# From file
go run ./cmd/play_mml -file ./song.mml

# Different engines (fm, chiptune, nesapu, wavetable)
go run ./cmd/play_mml -engine nesapu -mml "t140 o5 l8 cdefgab>c"
go run ./cmd/play_mml -engine wavetable -file examples/gr.mml

# Looped playback, stop after 3 loops
go run ./cmd/play_mml -loop -loops 3 -file examples/tr.mml

# Adjust volume
go run ./cmd/play_mml -volume 0.8 -file examples/gr.mml
```

#### CLI Flags

| Flag           | Default | Description                                    |
| -------------- | ------- | ---------------------------------------------- |
| `-file`        | (none)  | Path to MML file                               |
| `-mml`         | (none)  | Inline MML string                              |
| `-engine`      | fm      | `fm`, `chiptune`, `nesapu`, or `wavetable`     |
| `-sample-rate` | 48000   | Output sample rate                             |
| `-volume`      | 1.0     | Master volume scalar                           |
| `-loop`        | false   | Loop playback                                  |
| `-loops`       | 3       | When `-loop`, stop after N loops (0 = forever) |

### play_mml_ui (GUI)

An Ebitengine-based visual MML player with an integrated editor, waveform display, and spectrum analyzer:

```bash
# Launch with empty editor
go run ./cmd/play_mml_ui

# Load an MML file
go run ./cmd/play_mml_ui examples/tr.mml

# Or use make
make run-ui
make run-ui FILE=examples/tr.mml
```

The UI supports engine switching, play/pause/stop controls, and real-time audio visualization.

### play_mml_ui (Web / WASM)

The GUI player can also run in a web browser via WebAssembly. The web build bundles the example MML files as static assets and serves them alongside the player.

```bash
# Build the WASM player and assets into bin/web/
make build-web

# Build and serve locally on http://localhost:8080
make serve-web
```

The web version uses the same UI as the desktop GUI. A file navigator lists the bundled example files, which are fetched over HTTP when selected. The output structure:

```
bin/web/
  index.html
  wasm_exec.js
  play_mml_ui.wasm
  examples/
    files.json          # auto-generated manifest
    dq.mml, gr.mml, …  # example MML files
```

## MML Support

Implemented commands include:

- **Notes/rests** — `a`–`g`, accidentals, `r`
- **Length/tempo** — `l`, `t`, dotted notes `.`
- **Octave** — `o`, `<`, `>`
- **Volume/pan** — `v`, `@v`, `p`, `@p`, `%v`, `%x` scaling
- **Control** — gate `q`, `@q` (192nd units), transpose `k`, tie `^`, portamento `po`, pitch slide `*`, slur `&`/`&&`, volume shift `(`/`)`
- **Loops** — `[ ... ]`, break `|`, repeat-all `$`
- **Programs** — `@n`, `@mask` event ignore mask
- **Multi-track** — comma-separated tracks; `;` for sectioned tracks
- **Macros** — `#A=...;`, `#AB=...;`, `#A-D=...;`, `#MACRO{static|dynamic}`, invoke `A`, `A(n)`
- **Directives** — `#END;`, `#REV{octave|volume};`, `#SIGN`, `#TMODE`, `#QUANT`, `#TABLE`, `#VMODE`, `#WAVB`, `#EFFECT`
- **Table envelopes** — `na/np/nt/nf`, release-prefixed forms, `@@`
- **LFO** — `@lfo`, `mp`, `ma`, `mf` (pitch/amp/filter modulation; saw/square/triangle/random waveforms)
- **Filter** — `%f` (LP/BP/HP), `@f` filter envelope (10-arg)
- **FM** — `@al`, `@fb` (multi-op, 8 carrier waveforms)
- **Engines** — `%0` NES APU, `%1` chiptune, `%4` wavetable, `%6` FM
- **Effects** — `#EFFECT` slots: delay, reverb, chorus, distortion, EQ, compressor
- **Events** — `%t`, `%e` triggers (emitted via `Watch()`)
- **Voice** — random phase `@ph -1`

See [docs/mmlref.md](docs/mmlref.md) for the full SiON MML reference and [docs/mmlref_spec_matrix.md](docs/mmlref_spec_matrix.md) for the conformance matrix and test coverage.

## Offline Rendering

Render to samples or WAV without realtime playback. All four engines are supported:

```go
score, _ := mmlfm.Compile("t140 o5 l8 cdefgab>c")
samples := mmlfm.RenderSamples(score, 48000, 5.0)           // FM
// Or: RenderSamplesChiptune, RenderSamplesNESAPU, RenderSamplesWavetable
wav := mmlfm.EncodeWAVFloat32LE(samples, 48000, 2)
os.WriteFile("output.wav", wav, 0644)
```

## Project Layout

```
├── cmd/play_mml/        # CLI player
├── cmd/play_mml_ui/     # GUI player (Ebitengine, desktop + WASM)
├── internal/
│   ├── mml/             # MML parser
│   ├── sequencer/       # Tick/sample scheduler
│   ├── fm/              # FM synth engine
│   ├── chiptune/        # Chip-style engine
│   ├── nesapu/          # NES APU-inspired engine
│   ├── wavetable/       # Wavetable synthesis engine
│   ├── effects/         # Delay, reverb, chorus, distortion, EQ, compressor
│   ├── lfo/             # LFO modulation (shared by all engines)
│   └── audio/           # Ebitengine audio adapter
├── web/                 # WASM bootstrap (index.html)
├── examples/            # Sample MML files
├── testdata/            # Golden hashes for snapshot tests
└── docs/                # MML spec, conformance matrix, enhancement plan
```

## Development

```bash
# Build everything (CLI, GUI, web)
make build

# Build individual targets
make build-cli          # CLI only
make build-ui           # Desktop GUI only
make build-web          # WASM web player only

# Run tests
make test

# Run with verbose output
make test-verbose

# Check formatting and vet
make fmt vet

# Lint (requires golangci-lint)
make lint

# Serve the web player locally
make serve-web
```

## Requirements

- Go 1.24+
- Ebitengine v2 for realtime audio (display/input not required)

## License

See [LICENSE](LICENSE) in the repository root.

## Acknowledgments

- SiON (https://github.com/keim/SiON) — MML synthesis reference
- SiON mmlref.html — MML specification (converted to [mmlref.md](docs/mmlref.md))
- Example MML files: see header comments in each file (e.g. LinearDrive, YURAYSAN)
