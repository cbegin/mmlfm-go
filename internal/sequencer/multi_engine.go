package sequencer

import (
	"sync"
)

// MultiEngine routes note and control events to multiple VoiceEngines by module number.
// It implements VoiceEngine and mixes the output of all engines.
type MultiEngine struct {
	mu          sync.Mutex
	engines     map[int]VoiceEngine
	defaultMod  int
	currentMod  int
	sampleRate  int
}

// NewMultiEngine creates a MultiEngine. defaultMod is used when no module is specified.
func NewMultiEngine(defaultMod int, sampleRate int) *MultiEngine {
	return &MultiEngine{
		engines:    make(map[int]VoiceEngine),
		defaultMod: defaultMod,
		currentMod: defaultMod,
		sampleRate: sampleRate,
	}
}

// AddEngine registers an engine for the given module number.
func (m *MultiEngine) AddEngine(module int, engine VoiceEngine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engines[module] = engine
}

// SetCurrentModule sets the module for subsequent control calls (SetFilterType, LFO, etc.).
// Called by the sequencer before processing events for a track.
func (m *MultiEngine) SetCurrentModule(module int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentMod = module
}

func (m *MultiEngine) engine(module int) VoiceEngine {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.engines[module]; ok {
		return e
	}
	if e, ok := m.engines[m.defaultMod]; ok {
		return e
	}
	for _, e := range m.engines {
		return e
	}
	return nil
}

func (m *MultiEngine) currentEngine() VoiceEngine {
	return m.engine(m.currentMod)
}

// AllEngines returns all registered engines (for OPM loading etc.).
func (m *MultiEngine) AllEngines() []VoiceEngine {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]VoiceEngine, 0, len(m.engines))
	for _, e := range m.engines {
		out = append(out, e)
	}
	return out
}

// encodeVoiceID packs module and local voice ID into a single int.
func encodeVoiceID(module int, localID int) int {
	return (module << 16) | (localID & 0xFFFF)
}

func decodeVoiceID(id int) (module int, localID int) {
	return (id >> 16) & 0xFF, id & 0xFFFF
}

func (m *MultiEngine) NoteOn(note int, velocity int, pan int, program int) int {
	module := (program >> 8) & 0xFF
	if module == 0 {
		module = m.currentMod
	}
	e := m.engine(module)
	if e == nil {
		return -1
	}
	localID := e.NoteOn(note, velocity, pan, program)
	return encodeVoiceID(module, localID)
}

func (m *MultiEngine) NoteOff(id int) {
	module, localID := decodeVoiceID(id)
	e := m.engine(module)
	if e != nil {
		e.NoteOff(localID)
	}
}

func (m *MultiEngine) RenderFrame() (float32, float32) {
	var l, r float32
	for _, e := range m.AllEngines() {
		el, er := e.RenderFrame()
		l += el
		r += er
	}
	return l, r
}

func (m *MultiEngine) SetMasterGain(gain float64) {
	for _, e := range m.AllEngines() {
		e.SetMasterGain(gain)
	}
}

func (m *MultiEngine) SetFilterType(filterType int) {
	if e := m.currentEngine(); e != nil {
		e.SetFilterType(filterType)
	}
}

func (m *MultiEngine) SetNoteOnPhase(phase int) {
	if e := m.currentEngine(); e != nil {
		e.SetNoteOnPhase(phase)
	}
}

func (m *MultiEngine) SetPortamento(fromNote int, frames int) {
	if e := m.currentEngine(); e != nil {
		e.SetPortamento(fromNote, frames)
	}
}

func (m *MultiEngine) SetPitchLFO(depth float64, rateHz float64, waveform int) {
	if e := m.currentEngine(); e != nil {
		e.SetPitchLFO(depth, rateHz, waveform)
	}
}

func (m *MultiEngine) SetAmpLFO(depth float64, rateHz float64, waveform int) {
	if e := m.currentEngine(); e != nil {
		e.SetAmpLFO(depth, rateHz, waveform)
	}
}

func (m *MultiEngine) SetFilterLFO(depth float64, rateHz float64, waveform int) {
	if e := m.currentEngine(); e != nil {
		e.SetFilterLFO(depth, rateHz, waveform)
	}
}

func (m *MultiEngine) ActiveVoiceCount() int {
	n := 0
	for _, e := range m.AllEngines() {
		n += e.ActiveVoiceCount()
	}
	return n
}
