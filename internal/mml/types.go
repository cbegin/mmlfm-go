package mml

type EventType int

const (
	EventNote EventType = iota + 1
	EventRest
	EventTempo
	EventVolume
	EventFineVolume
	EventProgram
	EventPan
	EventExpression
	EventModule
	EventDetune
	EventTranspose
	EventQuantize
	EventKeyOnDelay
	EventSlur
	EventTableEnv
	EventControl
)

type SlurMode int

const (
	SlurNone SlurMode = iota
	SlurNormal
	SlurWeak
)

type Event struct {
	Type     EventType
	Tick     int
	Duration int
	Note     int
	Value    int
	Program  int
	Pan      int
	Module   int
	Channel  int
	Detune   int
	Expr     int
	GateTick int
	Delay    int
	Slur     SlurMode
	Command  string
	Text     string
	Values   []int
}

type Track struct {
	Events    []Event
	EndTick   int
	LoopTick  int
	LoopIndex int
}

type Score struct {
	Resolution  int
	InitialBPM  float64
	Tracks      []Track
	Definitions map[string]string
}

type ParserConfig struct {
	Resolution     int
	DefaultBPM     float64
	DefaultLValue  int
	DefaultOctave  int
	MinOctave      int
	MaxOctave      int
	DefaultVolume  int
	DefaultFineVol int
	OctavePolarize int
}

func DefaultParserConfig() ParserConfig {
	return ParserConfig{
		Resolution:     1920,
		DefaultBPM:     120,
		DefaultLValue:  4,
		DefaultOctave:  5,
		MinOctave:      0,
		MaxOctave:      9,
		DefaultVolume:  16,
		DefaultFineVol: 127,
		OctavePolarize: -1,
	}
}
