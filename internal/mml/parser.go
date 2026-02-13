package mml

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

var noteOffsets = map[byte]int{
	'c': 0, 'd': 2, 'e': 4, 'f': 5, 'g': 7, 'a': 9, 'b': 11,
}

type Parser struct{ cfg ParserConfig }

func NewParser(cfg ParserConfig) *Parser { return &Parser{cfg: cfg} }

func (p *Parser) Parse(input string) (*Score, error) {
	preprocessed := preprocessInput(input)
	parts := splitSectionsAsTracks(preprocessed.text)
	tmode, tunit, tfps := parseTMODE(preprocessed.definitions)
	opts := parserOptions{
		quantMax:  parseQuantMax(preprocessed.definitions),
		tempoMode: tmode,
		tempoUnit: tunit,
		tempoFPS:  tfps,
	}
	tracks := make([]Track, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		tr, _, err := p.parseTrack(part, opts, preprocessed.definitions)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, tr)
	}
	return &Score{
		Resolution:  p.cfg.Resolution,
		InitialBPM:  p.cfg.DefaultBPM,
		Tracks:      tracks,
		Definitions: preprocessed.definitions,
	}, nil
}

type parserOptions struct {
	quantMax  int
	tempoMode string
	tempoUnit int
	tempoFPS  int
}

func (p *Parser) parseTrack(input string, opts parserOptions, defs map[string]string) (Track, float64, error) {
	expanded, err := expandLoops(input)
	if err != nil {
		return Track{}, 0, err
	}
	st := newState(p.cfg, opts, defs)
	events := make([]Event, 0, 256)
	i := 0
	loopTick, loopIndex := -1, -1
	for i < len(expanded) {
		ch := lower(expanded[i])
		if isSpace(ch) {
			i++
			continue
		}
		switch {
		case ch == 'n' && i+1 < len(expanded) && unicode.IsDigit(rune(expanded[i+1])):
			evt, stepDur, next, e := parseNoteByNumber(expanded, i, st)
			if e != nil {
				return Track{}, 0, e
			}
			events = append(events, evt)
			st.slurMode = SlurNone
			st.tick += stepDur
			i = next
		case isNote(ch):
			evt, stepDur, next, e := parseNote(expanded, i, st)
			if e != nil {
				return Track{}, 0, e
			}
			events = append(events, evt)
			st.slurMode = SlurNone
			st.tick += stepDur
			i = next
		case ch == 'r':
			dur, next, e := parseLengthWithTie(expanded, i+1, st)
			if e != nil {
				return Track{}, 0, e
			}
			events = append(events, Event{Type: EventRest, Tick: st.tick, Duration: dur})
			st.tick += dur
			i = next
		case ch == 'l':
			length, next, e := parseLengthToken(expanded, i+1, st)
			if e != nil {
				return Track{}, 0, e
			}
			st.defaultLen = length
			i = next
		case ch == 't':
			val, next, e := parseNumberDefault(expanded, i+1, int(st.bpm))
			if e != nil {
				return Track{}, 0, e
			}
			bpm := applyTMODETempo(val, opts)
			st.bpm = bpm
			events = append(events, Event{Type: EventTempo, Tick: st.tick, Value: int(math.Round(bpm))})
			i = next
		case ch == 'o':
			val, next, e := parseNumberDefault(expanded, i+1, st.octave)
			if e != nil {
				return Track{}, 0, e
			}
			if val < p.cfg.MinOctave || val > p.cfg.MaxOctave {
				return Track{}, 0, fmt.Errorf("octave out of range at %d", i)
			}
			st.octave = val
			i = next
		case strings.HasPrefix(expanded[i:], "«"):
			st.octave += 2 * p.cfg.OctavePolarize
			st.octave = clampInt(st.octave, p.cfg.MinOctave, p.cfg.MaxOctave)
			i += len("«")
		case strings.HasPrefix(expanded[i:], "»"):
			st.octave -= 2 * p.cfg.OctavePolarize
			st.octave = clampInt(st.octave, p.cfg.MinOctave, p.cfg.MaxOctave)
			i += len("»")
		case ch == '<':
			val, next, e := parseNumberDefault(expanded, i+1, 1)
			if e != nil {
				return Track{}, 0, e
			}
			st.octave += val * p.cfg.OctavePolarize
			st.octave = clampInt(st.octave, p.cfg.MinOctave, p.cfg.MaxOctave)
			i = next
		case ch == '>':
			val, next, e := parseNumberDefault(expanded, i+1, 1)
			if e != nil {
				return Track{}, 0, e
			}
			st.octave -= val * p.cfg.OctavePolarize
			st.octave = clampInt(st.octave, p.cfg.MinOctave, p.cfg.MaxOctave)
			i = next
		case ch == 'v':
			val, next, e := parseNumberDefault(expanded, i+1, st.volume)
			if e != nil {
				return Track{}, 0, e
			}
			st.volume = val
			events = append(events, Event{Type: EventVolume, Tick: st.tick, Value: val})
			i = next
		case ch == 'x':
			val, next, e := parseNumberDefault(expanded, i+1, st.expression)
			if e != nil {
				return Track{}, 0, e
			}
			st.expression = clampInt(val, 0, 128)
			events = append(events, Event{Type: EventExpression, Tick: st.tick, Value: st.expression})
			i = next
		case ch == 'q':
			val, next, e := parseNumberDefault(expanded, i+1, st.quantValue)
			if e != nil {
				return Track{}, 0, e
			}
			val = clampInt(val, 0, st.quantMax)
			st.quantValue = val
			st.gatePercent = (val * 100) / st.quantMax
			events = append(events, Event{Type: EventQuantize, Tick: st.tick, Value: val})
			i = next
		case ch == 'k':
			if i+1 < len(expanded) && lower(expanded[i+1]) == 't' {
				val, next, e := parseSignedNumberDefault(expanded, i+2, st.transpose)
				if e != nil {
					return Track{}, 0, e
				}
				st.transpose = val
				events = append(events, Event{Type: EventTranspose, Tick: st.tick, Value: val})
				i = next
				continue
			}
			val, next, e := parseSignedNumberDefault(expanded, i+1, st.detune)
			if e != nil {
				return Track{}, 0, e
			}
			st.detune = val
			events = append(events, Event{Type: EventDetune, Tick: st.tick, Value: val})
			i = next
		case ch == 'p':
			if i+1 < len(expanded) && lower(expanded[i+1]) == 'o' {
				val, next, e := parseSignedNumberDefault(expanded, i+2, 0)
				if e != nil {
					return Track{}, 0, e
				}
				events = append(events, Event{Type: EventControl, Tick: st.tick, Command: "po", Value: val})
				i = next
				continue
			}
			val, next, e := parseSignedNumberDefault(expanded, i+1, st.pan)
			if e != nil {
				return Track{}, 0, e
			}
			st.pan = normalizePanValue(val)
			events = append(events, Event{Type: EventPan, Tick: st.tick, Value: st.pan})
			i = next
		case ch == '%':
			if i+1 < len(expanded) && (lower(expanded[i+1]) == 'f' || lower(expanded[i+1]) == 't' || lower(expanded[i+1]) == 'e') {
				cmd := "%" + string(lower(expanded[i+1]))
				val, next, e := parseSignedNumberDefault(expanded, i+2, 0)
				if e != nil {
					return Track{}, 0, e
				}
				values := []int{val}
				for next < len(expanded) && expanded[next] == ',' {
					arg, n2, e2 := parseSignedNumberDefault(expanded, next+1, 0)
					if e2 != nil {
						return Track{}, 0, e2
					}
					values = append(values, arg)
					next = n2
				}
				events = append(events, Event{Type: EventControl, Tick: st.tick, Command: cmd, Value: val, Values: values})
				i = next
				continue
			}
			if i+1 < len(expanded) && (lower(expanded[i+1]) == 'v' || lower(expanded[i+1]) == 'x') {
				scaleName := lower(expanded[i+1])
				val, next, e := parseNumberDefault(expanded, i+2, 0)
				if e != nil {
					return Track{}, 0, e
				}
				if scaleName == 'v' {
					mode := val
					max := st.vScaleMax
					if next < len(expanded) && expanded[next] == ',' {
						mv, n2, e2 := parseNumberDefault(expanded, next+1, 0)
						if e2 != nil {
							return Track{}, 0, e2
						}
						// Spec: n2 = max value of v computed as 256 >> n2.
						if mv > 0 {
							max = 256 >> mv
						}
						next = n2
					}
					if max <= 0 {
						max = 16
					}
					st.vScaleMode = mode
					st.vScaleMax = max
					events = append(events, Event{
						Type:    EventControl,
						Tick:    st.tick,
						Command: "%v",
						Value:   mode,
						Values:  []int{mode, max},
					})
				} else {
					st.xScaleMode = val
					events = append(events, Event{
						Type:    EventControl,
						Tick:    st.tick,
						Command: "%x",
						Value:   val,
						Values:  []int{val},
					})
				}
				i = next
				continue
			}
			mod, next, e := parseNumberDefault(expanded, i+1, st.module)
			if e != nil {
				return Track{}, 0, e
			}
			st.module = mod
			st.channel = 0
			if next < len(expanded) && expanded[next] == ',' {
				chv, n2, e2 := parseNumberDefault(expanded, next+1, 0)
				if e2 != nil {
					return Track{}, 0, e2
				}
				st.channel = chv
				next = n2
			}
			events = append(events, Event{Type: EventModule, Tick: st.tick, Module: st.module, Channel: st.channel})
			i = next
		case ch == '&':
			if i+1 < len(expanded) && expanded[i+1] == '&' {
				st.slurMode = SlurWeak
				events = append(events, Event{Type: EventSlur, Tick: st.tick, Slur: SlurWeak})
				i += 2
				continue
			}
			st.slurMode = SlurNormal
			events = append(events, Event{Type: EventSlur, Tick: st.tick, Slur: SlurNormal})
			i++
		case ch == 's':
			// sustain/release command: s n1,n2 where n1=release rate, n2=pitch sweep.
			val, next, e := parseSignedNumberDefault(expanded, i+1, 0)
			if e != nil {
				return Track{}, 0, e
			}
			values := []int{val}
			if next < len(expanded) && expanded[next] == ',' {
				v2, n2, e2 := parseSignedNumberDefault(expanded, next+1, 0)
				if e2 != nil {
					return Track{}, 0, e2
				}
				values = append(values, v2)
				next = n2
			}
			events = append(events, Event{Type: EventControl, Tick: st.tick, Command: "s", Value: val, Values: values})
			i = next
		case ch == '(' || ch == ')':
			// volume shift shorthand
			shift, next, e := parseNumberDefault(expanded, i+1, 1)
			if e != nil {
				return Track{}, 0, e
			}
			up := ch == '('
			if st.revVolume {
				up = !up
			}
			if up {
				st.volume += shift
			} else {
				st.volume -= shift
			}
			st.volume = clampInt(st.volume, 0, 127)
			events = append(events, Event{Type: EventVolume, Tick: st.tick, Value: st.volume})
			i = next
		case ch == '@':
			if i+1 < len(expanded) && lower(expanded[i+1]) == 'v' {
				val, next, e := parseNumberDefault(expanded, i+2, st.fineVol)
				if e != nil {
					return Track{}, 0, e
				}
				values := []int{val}
				for next < len(expanded) && expanded[next] == ',' {
					arg, n2, e2 := parseNumberDefault(expanded, next+1, 0)
					if e2 != nil {
						return Track{}, 0, e2
					}
					values = append(values, arg)
					next = n2
				}
				st.fineVol = val
				events = append(events, Event{Type: EventFineVolume, Tick: st.tick, Value: val, Values: values})
				i = next
				continue
			}
			if i+1 < len(expanded) && lower(expanded[i+1]) == 'q' {
				off, next, e := parseNumberDefault(expanded, i+2, st.keyOffTick)
				if e != nil {
					return Track{}, 0, e
				}
				convertedOff := convertQuarter192ToTicks(off, st.resolution)
				if convertedOff <= 0 {
					convertedOff = -1
				}
				st.keyOffTick = convertedOff
				st.keyOnDelay = 0
				if next < len(expanded) && expanded[next] == ',' {
					delay, n2, e2 := parseNumberDefault(expanded, next+1, 0)
					if e2 != nil {
						return Track{}, 0, e2
					}
					st.keyOnDelay = convertQuarter192ToTicks(delay, st.resolution)
					next = n2
				}
				events = append(events, Event{Type: EventKeyOnDelay, Tick: st.tick, GateTick: st.keyOffTick, Delay: st.keyOnDelay})
				i = next
				continue
			}
			if startsWithWord(expanded, i, "@p") {
				val, next, e := parseSignedNumberDefault(expanded, i+2, st.pan)
				if e != nil {
					return Track{}, 0, e
				}
				st.pan = normalizePanValue(val)
				events = append(events, Event{Type: EventPan, Tick: st.tick, Value: st.pan})
				i = next
				continue
			}
			if startsWithWord(expanded, i, "@mask") {
				val, next, e := parseNumberDefault(expanded, i+5, 0)
				if e != nil {
					return Track{}, 0, e
				}
				events = append(events, Event{Type: EventControl, Tick: st.tick, Command: "@mask", Value: clampInt(val, 0, 63)})
				i = next
				continue
			}
			if i+1 < len(expanded) && isAlpha(lower(expanded[i+1])) {
				cmdStart := i + 1
				cmdEnd := cmdStart
				for cmdEnd < len(expanded) && isAlpha(lower(expanded[cmdEnd])) {
					cmdEnd++
				}
				cmd := strings.ToLower(expanded[cmdStart:cmdEnd])
				first := 0
				next := cmdEnd
				if cmdEnd < len(expanded) {
					if v, n2, e := parseSignedNumberDefault(expanded, cmdEnd, 0); e == nil {
						first = v
						next = n2
					}
				}
				// Parse optional comma arguments and preserve raw tail text for compatibility.
				tailStart := next
				for next < len(expanded) && (expanded[next] == ',' || expanded[next] == '+' || expanded[next] == '-' || (expanded[next] >= '0' && expanded[next] <= '9') || isSpace(expanded[next])) {
					next++
				}
				events = append(events, Event{
					Type:    EventControl,
					Tick:    st.tick,
					Command: "@" + cmd,
					Value:   first,
					Text:    strings.TrimSpace(expanded[tailStart:next]),
				})
				i = next
				continue
			}
			val, next, e := parseNumberDefault(expanded, i+1, st.program)
			if e != nil {
				return Track{}, 0, e
			}
			st.program = val
			args := []int{}
			for next < len(expanded) && expanded[next] == ',' {
				arg, n2, e2 := parseNumberDefault(expanded, next+1, 0)
				if e2 != nil {
					break
				}
				args = append(args, arg)
				next = n2
			}
			evt := Event{Type: EventProgram, Tick: st.tick, Value: val}
			if len(args) > 0 {
				evt.Values = args
			}
			events = append(events, evt)
			i = next
		case ch == '$':
			loopTick, loopIndex = st.tick, len(events)
			i++
		default:
			// parser-level fallback coverage for commands we do not fully
			// synthesize yet but still need to keep in the control stream.
			if startsWithWord(expanded, i, "kt") {
				val, next, e := parseSignedNumberDefault(expanded, i+2, st.transpose)
				if e != nil {
					return Track{}, 0, e
				}
				st.transpose = val
				events = append(events, Event{Type: EventTranspose, Tick: st.tick, Value: val})
				i = next
				continue
			}
			if startsWithWord(expanded, i, "po") || ch == '*' {
				cmd := string(ch)
				advance := i + 1
				if startsWithWord(expanded, i, "po") {
					cmd = "po"
					advance = i + 2
				}
				val, next, e := parseSignedNumberDefault(expanded, advance, 0)
				if e != nil {
					return Track{}, 0, e
				}
				events = append(events, Event{Type: EventControl, Tick: st.tick, Command: cmd, Value: val})
				i = next
				continue
			}
			if startsWithWord(expanded, i, "mp") || startsWithWord(expanded, i, "ma") || startsWithWord(expanded, i, "mf") {
				cmd := strings.ToLower(expanded[i : i+2])
				val, next, e := parseSignedNumberDefault(expanded, i+2, 0)
				if e != nil {
					return Track{}, 0, e
				}
				tailStart := next
				for next < len(expanded) && (expanded[next] == ',' || expanded[next] == '+' || expanded[next] == '-' || (expanded[next] >= '0' && expanded[next] <= '9') || isSpace(expanded[next])) {
					next++
				}
				events = append(events, Event{
					Type:    EventControl,
					Tick:    st.tick,
					Command: cmd,
					Value:   val,
					Text:    strings.TrimSpace(expanded[tailStart:next]),
				})
				i = next
				continue
			}
			if startsWithWord(expanded, i, "na") || startsWithWord(expanded, i, "np") || startsWithWord(expanded, i, "nt") || startsWithWord(expanded, i, "nf") ||
				startsWithWord(expanded, i, "_na") || startsWithWord(expanded, i, "_np") || startsWithWord(expanded, i, "_nt") || startsWithWord(expanded, i, "_nf") ||
				startsWithWord(expanded, i, "@@") || startsWithWord(expanded, i, "_@@") {
				cmd, next := parseWordToken(expanded, i)
				val, n2, _ := parseSignedNumberDefault(expanded, next, 0)
				step := 1
				values := []int{val}
				if n2 < len(expanded) && expanded[n2] == ',' {
					v2, n3, _ := parseNumberDefault(expanded, n2+1, 1)
					step = v2
					values = append(values, v2)
					n2 = n3
				}
				events = append(events, Event{Type: EventTableEnv, Tick: st.tick, Command: cmd, Value: val, Delay: step, Values: values})
				i = n2
				continue
			}
			i++
		}
	}
	return Track{
		Events:    events,
		EndTick:   st.tick,
		LoopTick:  loopTick,
		LoopIndex: loopIndex,
	}, st.bpm, nil
}

type parseState struct {
	resolution  int
	tick        int
	octave      int
	defaultLen  int
	bpm         float64
	volume      int
	fineVol     int
	expression  int
	quantMax    int
	quantValue  int
	gatePercent int
	keyOffTick  int
	keyOnDelay  int
	slurMode    SlurMode
	transpose   int
	detune      int
	pan         int
	program     int
	module      int
	channel     int
	revVolume   bool
	keySig      map[byte]int
	vmode       string
	vScaleMode  int
	vScaleMax   int
	xScaleMode  int
}

func newState(cfg ParserConfig, opts parserOptions, defs map[string]string) parseState {
	quantMax := opts.quantMax
	if quantMax <= 0 {
		quantMax = 8
	}
	quantValue := (quantMax * 3) / 4
	if quantValue <= 0 {
		quantValue = quantMax
	}
	return parseState{
		resolution:  cfg.Resolution,
		octave:      cfg.DefaultOctave,
		defaultLen:  cfg.Resolution / cfg.DefaultLValue,
		bpm:         cfg.DefaultBPM,
		volume:      cfg.DefaultVolume,
		fineVol:     cfg.DefaultFineVol,
		expression:  128,
		quantMax:    quantMax,
		quantValue:  quantValue,
		gatePercent: (quantValue * 100) / quantMax,
		keyOffTick:  -1,
		revVolume:   isVolumeReversed(defs),
		keySig:      parseKeySignature(defs),
		vmode:       strings.ToLower(strings.TrimSpace(defs["VMODE"])),
		vScaleMode:  0,
		vScaleMax:   16,
		xScaleMode:  0,
	}
}

func parseNoteByNumber(s string, at int, st parseState) (Event, int, int, error) {
	nn, next, err := parseNumberDefault(s, at+1, 60)
	if err != nil {
		return Event{}, 0, at, err
	}
	nn += st.transpose
	nn += st.detune / 64
	if nn < 0 {
		nn = 0
	}
	if nn > 127 {
		nn = 127
	}
	dur, next, err := parseLengthWithTie(s, next, st)
	if err != nil {
		return Event{}, 0, at, err
	}
	vel := scaledVelocity(st.volume, st.expression, st.fineVol, st.vScaleMode, st.vScaleMax, st.xScaleMode, st.vmode)
	noteDur := parseGateDuration(dur, st.gatePercent)
	if st.keyOffTick > 0 {
		noteDur = noteDur - st.keyOffTick - st.keyOnDelay
		if noteDur < 0 {
			noteDur = 0
		}
	}
	return Event{
		Type:     EventNote,
		Tick:     st.tick,
		Duration: noteDur,
		Note:     nn,
		Value:    vel,
		Program:  st.program,
		Pan:      st.pan,
		Module:   st.module,
		Channel:  st.channel,
		Detune:   st.detune,
		Expr:     st.expression,
		GateTick: st.keyOffTick,
		Delay:    st.keyOnDelay,
		Slur:     st.slurMode,
	}, dur, next, nil
}

func parseNote(s string, at int, st parseState) (Event, int, int, error) {
	base := noteOffsets[lower(s[at])]
	i, shift := at+1, 0
	explicitAccidental := false
	for i < len(s) {
		switch lower(s[i]) {
		case '#', '+':
			shift++
			explicitAccidental = true
			i++
		case '-', 'b':
			shift--
			explicitAccidental = true
			i++
		default:
			goto done
		}
	}
done:
	if !explicitAccidental {
		shift += st.keySig[lower(s[at])]
	}
	dur, next, err := parseLengthWithTie(s, i, st)
	if err != nil {
		return Event{}, 0, at, err
	}
	nn := st.octave*12 + base + shift
	nn += st.transpose
	nn += st.detune / 64
	if nn < 0 {
		nn = 0
	}
	if nn > 127 {
		nn = 127
	}
	vel := scaledVelocity(st.volume, st.expression, st.fineVol, st.vScaleMode, st.vScaleMax, st.xScaleMode, st.vmode)
	noteDur := parseGateDuration(dur, st.gatePercent)
	if st.keyOffTick > 0 {
		noteDur = noteDur - st.keyOffTick - st.keyOnDelay
		if noteDur < 0 {
			noteDur = 0
		}
	}
	return Event{
		Type:     EventNote,
		Tick:     st.tick,
		Duration: noteDur,
		Note:     nn,
		Value:    vel,
		Program:  st.program,
		Pan:      st.pan,
		Module:   st.module,
		Channel:  st.channel,
		Detune:   st.detune,
		Expr:     st.expression,
		GateTick: st.keyOffTick,
		Delay:    st.keyOnDelay,
		Slur:     st.slurMode,
	}, dur, next, nil
}

func parseLengthWithTie(s string, at int, st parseState) (int, int, error) {
	dur, i, err := parseLengthToken(s, at, st)
	if err != nil {
		return 0, at, err
	}
	for i < len(s) && lower(s[i]) == '^' {
		extra, next, e := parseLengthToken(s, i+1, st)
		if e != nil {
			return 0, at, e
		}
		dur += extra
		i = next
	}
	return dur, i, nil
}

func parseLengthToken(s string, at int, st parseState) (int, int, error) {
	val, i, err := parseNumberOptional(s, at)
	if err != nil {
		return 0, at, err
	}
	base := st.defaultLen
	if val > 0 {
		base = st.resolution / val
	}
	dots := 0
	for i < len(s) && s[i] == '.' {
		dots++
		i++
	}
	dur, term := base, base
	for k := 0; k < dots; k++ {
		term >>= 1
		dur += term
	}
	return dur, i, nil
}

func parseNumberDefault(s string, at int, def int) (int, int, error) {
	v, i, err := parseNumberOptional(s, at)
	if err != nil {
		return 0, at, err
	}
	if v == -1 {
		return def, i, nil
	}
	return v, i, nil
}

func parseSignedNumberDefault(s string, at int, def int) (int, int, error) {
	if at >= len(s) {
		return def, at, nil
	}
	sign := 1
	i := at
	if s[i] == '+' {
		i++
	} else if s[i] == '-' {
		sign = -1
		i++
	}
	v, next, err := parseNumberOptional(s, i)
	if err != nil {
		return 0, at, err
	}
	if v == -1 {
		return def, next, nil
	}
	return sign * v, next, nil
}

func parseNumberOptional(s string, at int) (int, int, error) {
	i, start := at, at
	for i < len(s) && unicode.IsDigit(rune(s[i])) {
		i++
	}
	if start == i {
		return -1, i, nil
	}
	n, err := strconv.Atoi(s[start:i])
	if err != nil {
		return 0, at, err
	}
	return n, i, nil
}

func parseGateDuration(dur int, gatePercent int) int {
	if gatePercent <= 0 {
		return 0
	}
	gated := (dur * gatePercent) / 100
	if gated <= 0 && dur > 0 {
		return 1
	}
	return gated
}

func convertQuarter192ToTicks(v int, resolution int) int {
	if v < 0 {
		return v
	}
	if resolution <= 0 {
		resolution = 1920
	}
	return (v * resolution) / 192
}

func normalizePanValue(v int) int {
	// Spec center is @p=0 in -64..64. Keep backward compatibility by accepting
	// old signed p values and coarse p0..p8 values.
	if v >= 0 && v <= 8 {
		return (v - 4) * 16
	}
	if v > 64 {
		return 64
	}
	if v < -64 {
		return -64
	}
	return v
}

func isVolumeReversed(defs map[string]string) bool {
	if defs == nil {
		return false
	}
	rev, ok := defs["REV"]
	if !ok {
		return false
	}
	rev = strings.ToLower(strings.TrimSpace(rev))
	return rev == "" || strings.Contains(rev, "volume")
}

func scaledVelocity(volume int, expression int, fineVol int, vScaleMode int, vScaleMax int, xScaleMode int, vmode string) int {
	volMax := vScaleMax
	if volMax <= 0 {
		volMax = 16
	}
	vol := clampInt(volume, 0, 127)
	expr := clampInt(expression, 0, 128)
	fine := clampInt(fineVol, 0, 128)
	volNorm := float64(vol) / float64(volMax)
	if volNorm < 0 {
		volNorm = 0
	}
	if volNorm > 1 {
		volNorm = 1
	}
	exprNorm := float64(expr) / 128.0
	switch {
	case vScaleMode == 1, strings.Contains(vmode, "n88") && vScaleMode == 0:
		volNorm = dbScale(volNorm, 96)
	case vScaleMode == 2, strings.Contains(vmode, "mdx") && vScaleMode == 0:
		volNorm = dbScale(volNorm, 64)
	case vScaleMode == 3, strings.Contains(vmode, "mck") && vScaleMode == 0:
		volNorm = dbScale(volNorm, 48)
	case vScaleMode == 4, strings.Contains(vmode, "tss") && vScaleMode == 0:
		volNorm = dbScale(volNorm, 32)
	}
	switch xScaleMode {
	case 1:
		exprNorm = math.Sqrt(exprNorm)
	case 2:
		exprNorm = exprNorm * exprNorm
	case 3:
		exprNorm = dbScale(exprNorm, 48)
	case 4:
		exprNorm = dbScale(exprNorm, 32)
	}
	vel := volNorm * exprNorm * (float64(fine) / 128.0) * 127.0
	return clampInt(int(math.Round(vel)), 0, 127)
}

func dbScale(norm float64, dbRange float64) float64 {
	if norm <= 0 {
		return 0
	}
	if norm >= 1 {
		return 1
	}
	return math.Pow(10, -dbRange*(1-norm)/20)
}

func parseKeySignature(defs map[string]string) map[byte]int {
	out := map[byte]int{'c': 0, 'd': 0, 'e': 0, 'f': 0, 'g': 0, 'a': 0, 'b': 0}
	if defs == nil {
		return out
	}
	raw := strings.TrimSpace(defs["SIGN"])
	if raw == "" {
		return out
	}
	lowerRaw := strings.ToLower(raw)
	if strings.Contains(lowerRaw, ",") {
		for _, tok := range strings.Split(lowerRaw, ",") {
			tok = strings.TrimSpace(tok)
			if len(tok) < 2 {
				continue
			}
			n := tok[0]
			if _, ok := out[n]; !ok {
				continue
			}
			switch tok[len(tok)-1] {
			case '+', '#':
				out[n] = 1
			case '-', 'b':
				out[n] = -1
			default:
				out[n] = 0
			}
		}
		return out
	}
	key := strings.ReplaceAll(strings.ReplaceAll(lowerRaw, "+", "#"), " ", "")
	switch key {
	case "c", "am":
		return out
	case "g", "em":
		out['f'] = 1
	case "d", "bm":
		out['f'], out['c'] = 1, 1
	case "a", "f#m":
		out['f'], out['c'], out['g'] = 1, 1, 1
	case "e", "c#m":
		out['f'], out['c'], out['g'], out['d'] = 1, 1, 1, 1
	case "b", "g#m":
		out['f'], out['c'], out['g'], out['d'], out['a'] = 1, 1, 1, 1, 1
	case "f#", "d#m":
		out['f'], out['c'], out['g'], out['d'], out['a'], out['e'] = 1, 1, 1, 1, 1, 1
	case "c#", "a#m":
		out['f'], out['c'], out['g'], out['d'], out['a'], out['e'], out['b'] = 1, 1, 1, 1, 1, 1, 1
	case "f", "dm":
		out['b'] = -1
	case "bb", "gm":
		out['b'], out['e'] = -1, -1
	case "eb", "cm":
		out['b'], out['e'], out['a'] = -1, -1, -1
	case "ab", "fm":
		out['b'], out['e'], out['a'], out['d'] = -1, -1, -1, -1
	case "db", "bbm":
		out['b'], out['e'], out['a'], out['d'], out['g'] = -1, -1, -1, -1, -1
	case "gb", "ebm":
		out['b'], out['e'], out['a'], out['d'], out['g'], out['c'] = -1, -1, -1, -1, -1, -1
	case "cb", "abm":
		out['b'], out['e'], out['a'], out['d'], out['g'], out['c'], out['f'] = -1, -1, -1, -1, -1, -1, -1
	}
	return out
}

type preprocessedInput struct {
	text        string
	definitions map[string]string
}

func preprocessInput(src string) preprocessedInput {
	noComments := stripComments(src)
	state := preprocessorState{
		macros:      make(map[string]string),
		definitions: make(map[string]string),
	}
	return preprocessedInput{
		text:        preprocessStream(noComments, &state),
		definitions: state.definitions,
	}
}

func stripComments(src string) string {
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); i++ {
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			i += 2
			for i < len(src) {
				if i+1 < len(src) && src[i] == '*' && src[i+1] == '/' {
					i++
					break
				}
				i++
			}
			continue
		}
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			i += 2
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if i < len(src) && src[i] == '\n' {
				out.WriteByte('\n')
			}
			continue
		}
		out.WriteByte(src[i])
	}
	return out.String()
}

type preprocessorState struct {
	macros       map[string]string
	definitions  map[string]string
	macroDynamic bool
	revOctave    bool
	revVolume    bool
}

func preprocessStream(src string, st *preprocessorState) string {
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); {
		if src[i] == '#' {
			advance, stopAll := parseDirective(src, i, st)
			if stopAll {
				break
			}
			i = advance
			continue
		}
		if isMacroName(src[i]) {
			name := string(src[i])
			if _, ok := st.macros[name]; ok {
				shift, next := parseOptionalSignedParen(src, i+1)
				out.WriteString(expandMacroByName(name, shift, st, 0))
				i = next
				continue
			}
		}
		if st.revOctave {
			if src[i] == '<' {
				out.WriteByte('>')
				i++
				continue
			}
			if src[i] == '>' {
				out.WriteByte('<')
				i++
				continue
			}
		}
		out.WriteByte(src[i])
		i++
	}
	return out.String()
}

func parseDirective(src string, at int, st *preprocessorState) (int, bool) {
	end := at + 1
	for end < len(src) && src[end] != ';' {
		end++
	}
	stmtEnd := end
	if end < len(src) && src[end] == ';' {
		stmtEnd = end + 1
	}
	body := strings.TrimSpace(src[at+1 : min(end, len(src))])
	if body == "" {
		return stmtEnd, false
	}
	upperBody := strings.ToUpper(body)
	if upperBody == "END" {
		st.definitions["END"] = "1"
		return len(src), true
	}
	if strings.HasPrefix(upperBody, "MACRO{") {
		mode := parseBraceValue(body[len("MACRO"):])
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "dynamic":
			st.macroDynamic = true
		case "static":
			st.macroDynamic = false
		}
		st.definitions["MACRO_MODE"] = strings.ToLower(strings.TrimSpace(mode))
		return stmtEnd, false
	}
	if strings.HasPrefix(upperBody, "REV") {
		// #REV; reverses both octave and volume.
		opts := strings.ToLower(strings.TrimSpace(parseBraceValue(body[len("REV"):])))
		if opts == "" || strings.Contains(opts, "octave") {
			st.revOctave = true
		}
		if opts == "" || strings.Contains(opts, "volume") {
			st.revVolume = true
		}
		st.definitions["REV"] = opts
		return stmtEnd, false
	}
	if key, val, ok := parseKnownDirective(body); ok {
		st.definitions[key] = val
		return stmtEnd, false
	}
	if applyMacroDefinition(body, st) {
		return stmtEnd, false
	}
	return stmtEnd, false
}

func parseKnownDirective(body string) (string, string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(body))
	switch {
	case strings.HasPrefix(upper, "TITLE{"):
		return "TITLE", parseBraceValue(body[len("TITLE"):]), true
	case strings.HasPrefix(upper, "SIGN{"):
		return "SIGN", parseBraceValue(body[len("SIGN"):]), true
	case strings.HasPrefix(upper, "VMODE{"):
		return "VMODE", parseBraceValue(body[len("VMODE"):]), true
	case strings.HasPrefix(upper, "TMODE{"):
		return "TMODE", parseBraceValue(body[len("TMODE"):]), true
	case strings.HasPrefix(upper, "FPS"):
		return "FPS", strings.TrimSpace(body[len("FPS"):]), true
	case strings.HasPrefix(upper, "QUANT"):
		return "QUANT", strings.TrimSpace(body[len("QUANT"):]), true
	case strings.HasPrefix(upper, "TABLE"):
		return extractDirectiveName(upper), body, true
	case strings.HasPrefix(upper, "WAV"):
		return extractDirectiveName(upper), body, true
	case strings.HasPrefix(upper, "OPL@"),
		strings.HasPrefix(upper, "OPM@"),
		strings.HasPrefix(upper, "OPN@"),
		strings.HasPrefix(upper, "OPX@"),
		strings.HasPrefix(upper, "MA@"),
		strings.HasPrefix(upper, "@"),
		strings.HasPrefix(upper, "FM{"),
		strings.HasPrefix(upper, "EFFECT"),
		strings.HasPrefix(upper, "SAMPLER"),
		strings.HasPrefix(upper, "PCMWAVE"),
		strings.HasPrefix(upper, "PCMVOICE"):
		return extractDirectiveName(upper), body, true
	default:
		return "", "", false
	}
}

func extractDirectiveName(s string) string {
	if s == "" {
		return "DIRECTIVE"
	}
	end := 0
	for end < len(s) {
		ch := s[end]
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '@' || ch == '_' {
			end++
			continue
		}
		break
	}
	if end == 0 {
		return "DIRECTIVE"
	}
	return s[:end]
}

func parseQuantMax(defs map[string]string) int {
	if defs == nil {
		return 8
	}
	raw, ok := defs["QUANT"]
	if !ok {
		return 8
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return 8
	}
	return v
}

func parseTMODE(defs map[string]string) (mode string, unit int, fps int) {
	if defs == nil {
		return "", 100, 60
	}
	raw, ok := defs["TMODE"]
	if !ok {
		return "", 100, 60
	}
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(raw, "unit="):
		v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(raw, "unit=")))
		if err != nil || v <= 0 {
			return "", 100, 60
		}
		return "unit", v, 60
	case strings.HasPrefix(raw, "fps="):
		v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(raw, "fps=")))
		if err != nil || v <= 0 {
			return "", 100, 60
		}
		return "fps", 100, v
	default:
		return "", 100, 60
	}
}

func applyTMODETempo(rawTempo int, opts parserOptions) float64 {
	if rawTempo <= 0 {
		return 120
	}
	switch opts.tempoMode {
	case "unit":
		unit := opts.tempoUnit
		if unit <= 0 {
			unit = 100
		}
		return float64(rawTempo) / float64(unit)
	case "fps":
		fps := opts.tempoFPS
		if fps <= 0 {
			fps = 60
		}
		// TMODE fps: t = frames per beat, bpm = fps*60/t.
		return (float64(fps) * 60.0) / float64(rawTempo)
	default:
		return float64(rawTempo)
	}
}

func parseBraceValue(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' {
		return ""
	}
	close := strings.IndexByte(s, '}')
	if close <= 0 {
		return ""
	}
	return s[1:close]
}

func applyMacroDefinition(stmt string, st *preprocessorState) bool {
	opIdx := strings.Index(stmt, "+=")
	opLen := 2
	appendMode := true
	if opIdx < 0 {
		opIdx = strings.IndexByte(stmt, '=')
		opLen = 1
		appendMode = false
	}
	if opIdx <= 0 {
		return false
	}
	targetSpec := strings.TrimSpace(stmt[:opIdx])
	value := strings.TrimSpace(stmt[opIdx+opLen:])
	targets := parseMacroTargets(targetSpec)
	if len(targets) == 0 {
		return false
	}
	expandedValue := value
	if !st.macroDynamic {
		expandedValue = expandMacroText(value, st, 0)
	}
	for _, target := range targets {
		if appendMode {
			st.macros[target] += expandedValue
		} else {
			st.macros[target] = expandedValue
		}
	}
	return true
}

func parseMacroTargets(spec string) []string {
	noSpace := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, spec)
	out := make([]string, 0, len(noSpace))
	seen := make(map[string]struct{}, len(noSpace))
	for i := 0; i < len(noSpace); {
		if i+2 < len(noSpace) && isMacroName(noSpace[i]) && noSpace[i+1] == '-' && isMacroName(noSpace[i+2]) {
			from := int(noSpace[i])
			to := int(noSpace[i+2])
			step := 1
			if from > to {
				step = -1
			}
			for c := from; ; c += step {
				key := string(byte(c))
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					out = append(out, key)
				}
				if c == to {
					break
				}
			}
			i += 3
			continue
		}
		if isMacroName(noSpace[i]) {
			key := string(noSpace[i])
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				out = append(out, key)
			}
		}
		i++
	}
	return out
}

func expandMacroByName(name string, shift int, st *preprocessorState, depth int) string {
	if depth > 32 {
		return name
	}
	body, ok := st.macros[name]
	if !ok {
		return name
	}
	if st.macroDynamic {
		body = expandMacroText(body, st, depth+1)
	}
	if shift != 0 {
		body = transposeNotes(body, shift)
	}
	if st.revOctave {
		body = swapOctaveMarkers(body)
	}
	return body
}

func expandMacroText(src string, st *preprocessorState, depth int) string {
	if depth > 32 {
		return src
	}
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if isMacroName(ch) {
			if _, ok := st.macros[string(ch)]; ok {
				shift, next := parseOptionalSignedParen(src, i+1)
				out.WriteString(expandMacroByName(string(ch), shift, st, depth+1))
				i = next - 1
				continue
			}
		}
		out.WriteByte(ch)
	}
	return out.String()
}

func parseOptionalSignedParen(src string, at int) (int, int) {
	if at >= len(src) || src[at] != '(' {
		return 0, at
	}
	i := at + 1
	sign := 1
	if i < len(src) && src[i] == '+' {
		i++
	} else if i < len(src) && src[i] == '-' {
		sign = -1
		i++
	}
	startDigits := i
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
	}
	if startDigits == i || i >= len(src) || src[i] != ')' {
		return 0, at
	}
	v, err := strconv.Atoi(src[startDigits:i])
	if err != nil {
		return 0, at
	}
	return sign * v, i + 1
}

func transposeNotes(src string, semitone int) string {
	var out strings.Builder
	out.Grow(len(src) + 16)
	currentOctave := 5
	for i := 0; i < len(src); {
		ch := src[i]
		lo := lower(ch)
		if lo == 'o' {
			out.WriteByte(ch)
			val, next, err := parseNumberOptional(src, i+1)
			if err == nil && val >= 0 {
				currentOctave = val
				out.WriteString(src[i+1 : next])
				i = next
				continue
			}
			i++
			continue
		}
		if lo == '<' || lo == '>' {
			delta, next, err := parseNumberDefault(src, i+1, 1)
			if err != nil {
				delta = 1
				next = i + 1
			}
			if lo == '<' {
				currentOctave += delta
			} else {
				currentOctave -= delta
			}
			out.WriteByte(ch)
			if next > i+1 {
				out.WriteString(src[i+1 : next])
			}
			i = next
			continue
		}
		if !isNote(lo) {
			out.WriteByte(ch)
			i++
			continue
		}
		base := noteOffsets[lo]
		j, shift := i+1, 0
		for j < len(src) {
			switch lower(src[j]) {
			case '#', '+':
				shift++
				j++
			case '-', 'b':
				shift--
				j++
			default:
				goto transposeDone
			}
		}
	transposeDone:
		abs := currentOctave*12 + base + shift + semitone
		newOct := abs / 12
		newNote := abs % 12
		if newNote < 0 {
			newNote += 12
			newOct--
		}
		out.WriteString(noteNameForSemitone(newNote))
		currentOctave = newOct
		i = j
	}
	return out.String()
}

func noteNameForSemitone(n int) string {
	switch n {
	case 0:
		return "c"
	case 1:
		return "c+"
	case 2:
		return "d"
	case 3:
		return "d+"
	case 4:
		return "e"
	case 5:
		return "f"
	case 6:
		return "f+"
	case 7:
		return "g"
	case 8:
		return "g+"
	case 9:
		return "a"
	case 10:
		return "a+"
	default:
		return "b"
	}
}

func swapOctaveMarkers(src string) string {
	if src == "" {
		return src
	}
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '<':
			out.WriteByte('>')
		case '>':
			out.WriteByte('<')
		default:
			out.WriteByte(src[i])
		}
	}
	return out.String()
}

func isMacroName(b byte) bool { return b >= 'A' && b <= 'Z' }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func splitSectionsAsTracks(src string) []string {
	sections := splitTopLevel(src, ';')
	nonEmptySections := make([]string, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		nonEmptySections = append(nonEmptySections, section)
	}
	if len(nonEmptySections) == 0 {
		return nil
	}

	globalPrelude := ""
	startSection := 0
	if len(nonEmptySections) > 1 && !containsPlayableEvents(nonEmptySections[0]) {
		globalPrelude = nonEmptySections[0]
		startSection = 1
	}

	parts := make([]string, 0, len(nonEmptySections)*2)
	for _, section := range nonEmptySections[startSection:] {
		for _, part := range splitTopLevel(section, ',') {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if globalPrelude != "" {
				parts = append(parts, globalPrelude+" "+part)
			} else {
				parts = append(parts, part)
			}
		}
	}
	if len(parts) == 0 && globalPrelude != "" {
		parts = append(parts, globalPrelude)
	}
	return parts
}

func splitTopLevel(src string, sep byte) []string {
	depth := 0
	start := 0
	parts := make([]string, 0, 4)
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		default:
			if src[i] == sep && depth == 0 {
				if sep == ',' && isArgumentComma(src, i) {
					continue
				}
				parts = append(parts, src[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, src[start:])
	return parts
}

func isArgumentComma(src string, at int) bool {
	if at < 0 || at >= len(src) || src[at] != ',' {
		return false
	}
	for i := at + 1; i < len(src); i++ {
		ch := src[i]
		if isSpace(ch) {
			continue
		}
		return (ch >= '0' && ch <= '9') || ch == '+' || ch == '-'
	}
	return false
}

func containsPlayableEvents(src string) bool {
	for i := 0; i < len(src); i++ {
		ch := lower(src[i])
		if isNote(ch) || ch == 'r' {
			return true
		}
	}
	return false
}

func startsWithWord(src string, at int, word string) bool {
	if at < 0 || at+len(word) > len(src) {
		return false
	}
	for i := 0; i < len(word); i++ {
		if lower(src[at+i]) != lower(word[i]) {
			return false
		}
	}
	return true
}

func parseWordToken(src string, at int) (string, int) {
	i := at
	for i < len(src) {
		ch := src[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '@' || ch == '_' {
			i++
			continue
		}
		break
	}
	return src[at:i], i
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

func isSpace(b byte) bool { return b == ' ' || b == '\n' || b == '\r' || b == '\t' }
func isNote(b byte) bool  { _, ok := noteOffsets[b]; return ok }

func expandLoops(src string) (string, error) {
	out, i, err := parseExpanded(src, 0, 0)
	if err != nil {
		return "", err
	}
	if i != len(src) {
		return "", fmt.Errorf("unexpected parser position: %d", i)
	}
	return out, nil
}

func parseExpanded(src string, at, depth int) (string, int, error) {
	var out strings.Builder
	for at < len(src) {
		ch := src[at]
		if ch == ']' {
			if depth == 0 {
				return "", at, fmt.Errorf("unmatched ']' at %d", at)
			}
			return out.String(), at, nil
		}
		if ch != '[' {
			out.WriteByte(ch)
			at++
			continue
		}
		body, next, err := parseLoopBody(src, at+1, depth+1)
		if err != nil {
			return "", at, err
		}
		out.WriteString(body)
		at = next
	}
	if depth > 0 {
		return "", at, fmt.Errorf("unclosed '['")
	}
	return out.String(), at, nil
}

func parseLoopBody(src string, at, depth int) (string, int, error) {
	var pre, post strings.Builder
	breakHit := false
	for at < len(src) {
		ch := src[at]
		if ch == '[' {
			body, next, err := parseLoopBody(src, at+1, depth+1)
			if err != nil {
				return "", at, err
			}
			if breakHit {
				post.WriteString(body)
			} else {
				pre.WriteString(body)
			}
			at = next
			continue
		}
		if ch == '|' && depth == 1 {
			breakHit = true
			at++
			continue
		}
		if ch == ']' {
			repeat, next, err := parseNumberDefault(src, at+1, 2)
			if err != nil {
				return "", at, err
			}
			if repeat < 1 {
				repeat = 1
			}
			preS, postS := pre.String(), post.String()
			var out strings.Builder
			if breakHit {
				for i := 0; i < repeat-1; i++ {
					out.WriteString(preS)
				}
				out.WriteString(postS)
			} else {
				for i := 0; i < repeat; i++ {
					out.WriteString(preS)
				}
			}
			return out.String(), next, nil
		}
		if breakHit {
			post.WriteByte(ch)
		} else {
			pre.WriteByte(ch)
		}
		at++
	}
	return "", at, fmt.Errorf("unclosed loop block")
}
