# MMLRef Spec Matrix

This matrix tracks [mmlref.md](mmlref.md) conformance by feature, owner, and test coverage.

## Owners

- `internal/mml/parser.go` + `internal/mml/types.go`: preprocessor/parser/event model
- `internal/sequencer/sequencer.go`: tick semantics, control application, runtime envelope/LFO
- `internal/fm/engine.go`, `internal/nesapu/engine.go`, `internal/chiptune/engine.go`: synth behavior
- `internal/wavetable/engine.go`: wavetable synthesis
- `internal/effects/`: delay, reverb, chorus, distortion, EQ, compressor
- `cmd/play_mml/main.go`: CLI behavior

## Matrix

### System commands

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `#TITLE{...}` | Parsed (stored, not displayed) | Parser preprocessor | — |
| `#SIGN{...}` key signature accidentals | Implemented | Parser note parsing | `TestParseSignAppliesImplicitAccidentals` |
| `#REV{octave}` | Implemented | Parser preprocessor | `TestParseRevAndEndDirectives` |
| `#REV{volume}` | Implemented | Parser parse state | `TestParseVolumeShiftAndRevVolume` |
| `#MACRO{static\|dynamic}`, `#[A-Z]=`, append/range macros | Implemented | Parser preprocessor | `TestParseMacroStaticAndDynamicModes`, `TestParseMacroRangeAndAppend` |
| `#VMODE{n88\|mdx\|mck\|tss}` | Implemented | Parser | `TestConformance_AdvancedCommandParsing` |
| `#TMODE{unit\|fps}` | Implemented | Parser tempo conversion | `TestParseTMODEUnitTempo` |
| `#QUANT` | Implemented | Parser options | `TestParseTransposeAndQuantize` |
| `#FPS` | Implemented | Parser + Sequencer | Sets default `@fps` for table envelope frame rate |
| `#END` | Implemented | Parser preprocessor | `TestParseRevAndEndDirectives` |

### Definitions

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `#TABLE` numeric/formula expansion (`(a)n`, `(a,b)n`, `[x,y]n`, stretch/magnify/offset) | Implemented | Sequencer table parser | `TestParseTableDefinitionsInterpolation` |
| `#WAVB` hex wavetable | Implemented | Parser + Player + Wavetable engine | `LoadWAVBFromDefs()` wired in player |
| `#WAV` formula wavetable | Parsed only | Parser stores; not wired | — |
| `#WAVCOLOR` / `#WAVC` | Parsed only | Parser stores; not wired | — |

### FM patch formats

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `#OPM@` OPM patch loading | Implemented | FM engine | `LoadOPMPatchFromDefs()`, AR/DR/RR/TL/MUL conversion |
| `#OPM@` patch modulation suffix (`mp`/`ma`/`mf` after `}`) | Implemented | Sequencer `parsePatchMods()` | Applied on `EventProgram`; used by `examples/tr.mml` |
| `#@` generic FM patch | Parsed only | Parser stores; not loaded at runtime | — |
| `#OPL@` OPL patch | Parsed only | Parser stores; not loaded at runtime | — |
| `#OPN@` OPN patch | Parsed only | Parser stores; not loaded at runtime | — |
| `#OPX@` OPX patch | Parsed only | Parser stores; not loaded at runtime | — |
| `#MA@` MA3 patch | Parsed only | Parser stores; not loaded at runtime | — |

### Sequence controls

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `t` tempo | Implemented | Parser + Sequencer | `TestParseBasicMelody` |
| `$` repeat-all marker | Implemented | Parser + Sequencer | `TestParseRepeatAllMarker` |
| `[...\|...]n` loop with alternates | Implemented | Parser + Sequencer | `TestParseLoopAlternate` |
| `@mask` event ignore mask (v/p/q/table/LFO groups) | Implemented | Parser + Sequencer | `TestSequencerMaskCanIgnorePan` |
| `//` and `/* */` comments | Implemented | Parser | — |
| `![...!]n` legacy loop | Not implemented | — | — |

### Pitch commands

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `a`-`g` notes with accidentals | Implemented | Parser + Sequencer | `TestParseBasicMelody` |
| `r` rest | Implemented | Parser + Sequencer | `TestParseBasicMelody` |
| `o` octave | Implemented | Parser + Sequencer | `TestParseBasicMelody` |
| `<` / `>` octave shift | Implemented | Parser + Sequencer | `TestParseBasicMelody` |
| `«` / `»` guillemet octave brackets | Implemented | Parser | `TestParseGuillemetOctave` |
| `k` detune | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `kt` transpose | Implemented | Parser + Sequencer | `TestParseTransposeAndQuantize` |
| `n` MIDI note command | Implemented | Parser | `TestParseNoteByNumber` |
| `*` pitch slide | Implemented | Sequencer + all engines | Portamento to next note |
| `po` portamento | Implemented | Sequencer + all engines | `po N` = N ms glide |

### Length commands

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `l` default length | Implemented | Parser | `TestParseBasicMelody` |
| `^` tie | Implemented | Parser | `TestParseBasicMelody` |
| `q` gate | Implemented | Parser | `TestParseTransposeAndQuantize` |
| `@q n1,n2` in 1/192 units | Implemented | Parser + Sequencer | `TestParseAtQUses192ndNoteUnits` |
| `&` slur (no key-off) | Implemented | Parser + Sequencer | `TestSequencerSlurCancelsPreviousNoteOff` |
| `&&` weak slur (envelope reset) | Implemented | Parser + Sequencer | `TestSequencerSlurCancelsPreviousNoteOff` |

### Volume and pan commands

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `v` volume | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `(` / `)` volume shift | Implemented | Parser | `TestParseVolumeShiftAndRevVolume` |
| `x` expression | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `%v` volume scaling mode | Implemented (core modes) | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `%x` expression scaling mode | Implemented (core modes) | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `@v` fine volume / effect send | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `p` pan (coarse 0-8) | Implemented | Parser + Sequencer + Engines | `TestParseProgramPanAndMultitrack` |
| `@p` pan (fine -64..64) | Implemented | Parser + Sequencer + Engines | pan engine tests |

### Voice commands

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `%` module select | Implemented | Parser + Sequencer | Multi-engine routing via `MultiEngine` |
| `@` program/tone select | Implemented | Parser + Sequencer | `@n,args` full arg parsing; `EventProgram.Values` |
| `@f` filter cutoff + envelope (10-arg) | Implemented | Sequencer | `@f co,res,ar,dr,sr,rr,co2,co3,sc,rc` ADSR stepping |
| `%f` filter type (LP/BP/HP) | Implemented | Sequencer + all engines | `%f0`=LP, `%f1`=BP, `%f2`=HP |
| `s` sustain/release shaping | Partial | Sequencer | 1st arg approximate via amp bias; 2nd arg (pitch sweep after key-off) not synthesized |
| `@ph` phase on key-on | Implemented | Sequencer + all engines | `@ph -1` = random, `@ph 0` = reset |
| FM multi-operator (`@al`, `@fb`, 1-4 ops) | Implemented | Sequencer + FM engine | `@al` sets operator count + algorithm; `@fb` sets feedback level |
| `i` operator select | Not implemented | — | Parsed but not forwarded |
| `@rr`, `@tl`, `@ml`, `@dt`, `@fx` | Not implemented | — | Parsed as generic @ commands but not forwarded to FM engine operators |
| `@se` SSG envelope | Not implemented | — | — |
| `@er` envelope reset | Not implemented | — | — |

### LFO / modulation

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `@lfo` rate/waveform | Implemented | Sequencer + all engines | `TestConformance_AdvancedCommandParsing` |
| `mp` pitch modulation (depth, end, delay, change) | Implemented | Sequencer + all engines | Per-frame pitch LFO via `SetPitchLFO` |
| `ma` amplitude modulation | Implemented | Sequencer + all engines | Per-frame amp LFO via `SetAmpLFO` |
| `mf` filter modulation | Implemented | Sequencer + all engines | Per-frame filter LFO via `SetFilterLFO` |

### Table envelopes

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `@@` timbre table | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `na` amplitude envelope | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `np` pan envelope | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `nt` pitch envelope | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `nf` filter envelope | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `_@@`, `_na`, `_np`, `_nt`, `_nf` release envelopes | Implemented | Parser + Sequencer | `TestConformance_AdvancedCommandParsing` |
| `@fps` frame rate | Implemented | Parser + Sequencer | Overrides default FPS for table stepping |

### Triggers

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `%t` event trigger | Implemented | Sequencer + Player | Emitted via `Watch()` as `EventTrigger` |
| `%e` dispatch event | Implemented | Sequencer + Player | One-shot trigger |

### Bus pipe controls

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `@o` output pipe | Not implemented | — | — |
| `@i` input pipe | Not implemented | — | — |
| `@r` ring modulation pipe | Not implemented | — | — |

### FM channel connection

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `#FM{...}` | Parsed only | Parser stores; not applied (requires bus pipe support) | — |

### Effects

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `#EFFECT` slot wiring | Implemented | Player `buildEffectChain()` | — |
| `delay` | Implemented | effects package | time/fb/cross/wet |
| `reverb` | Implemented | effects package | room/fb/wet |
| `chorus` | Implemented | effects package | delay/fb/depth/rate/wet |
| `dist` / `distortion` | Implemented | effects package | pre/post/lpf/slope |
| `eq` (3-band EQ) | Implemented | effects package | lg/mg/hg/lf/hf |
| `comp` / `compressor` | Implemented | effects package | threshold/ratio/attack/release/makeup |
| `ws` (wave shaper) | Not implemented | — | — |
| `autopan` | Not implemented | — | — |
| `stereo` enhancer | Not implemented | — | — |
| `ds` downsampler | Not implemented | — | — |
| `speaker` simulator | Not implemented | — | — |
| `lf`/`hf`/`bf`/`nf`/`pf`/`af` filters | Not implemented | — | — |
| `lb`/`hb` boosters | Not implemented | — | — |
| `nlf`/`nhf` envelope filters | Not implemented | — | — |
| `vowel` filter | Not implemented | — | — |

### Wavetable engine

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `%4` wavetable module routing | Implemented | Player + Sequencer | `SynthModeWavetable` engine selection |
| `ParseWAVB()` / `SetWavetable()` API | Implemented | Player + Wavetable engine | `LoadWAVBFromDefs()` loads `#WAVB` definitions at play time |

### PCM / Sampler

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| `#SAMPLER` | Parsed only | Parser stores; no sampler engine | — |
| `#PCMWAVE` | Parsed only | Parser stores; no PCM engine | — |
| `#PCMVOICE` | Parsed only | Parser stores; no PCM engine | — |
| `%7` PCM module | Not implemented | — | — |
| `%10` Sampler module | Not implemented | — | — |

### Other

| Feature | Status | Owner | Test coverage |
| --- | --- | --- | --- |
| FM equal-power pan law | Implemented | FM engine | `TestPanExtremesBiasChannels` |
| NES/chiptune stereo pan support | Implemented | NES/chiptune engines | `TestEngineSupportsStereoPan` |
| FM carrier waveforms (8 types) | Implemented | FM engine | sine/saw/triangle/square/pulse25/pulse12.5/half-rect/noise |
| Engine-specific deterministic goldens | Implemented | Offline renderer/tests | `TestGoldenWAVSnapshot` subtests |
| Multi-engine per-track routing | Implemented | Player + Sequencer | `MultiEngine`, `scoreUsedModules` |
| CLI flags (`-engine`, `-sample-rate`, `-file`, `-mml`, `-volume`, `-loop`, `-loops`) | Implemented | `cmd/play_mml` | `-engine fm\|chiptune\|nesapu\|wavetable` |
| Playback control: `Wait()`, `Watch()` | Implemented | Player | Prefer over sleeps and manual `Stop()` |

## Remaining strictness notes

- FM patch definitions (`#@`, `#OPL@`, etc. except `#OPM@`) are parsed and stored but not loaded. `#OPM@` is applied at runtime.
- Per-operator commands (`@rr`, `@tl`, etc.) are parsed but not yet forwarded from sequencer to engine.
- `#WAV` formula wavetable parsing exists but is not connected end-to-end.
- Parser still accepts legacy permissive constructs used by fixture content while enforcing key conformance semantics listed above.
