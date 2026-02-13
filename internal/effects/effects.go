package effects

// Effector processes stereo audio in-place.
type Effector interface {
	Process(l, r float32) (float32, float32)
	Reset()
}

// Chain applies a sequence of effects in order.
type Chain struct {
	effects []Effector
}

func NewChain(effects ...Effector) *Chain {
	return &Chain{effects: effects}
}

func (c *Chain) Process(l, r float32) (float32, float32) {
	for _, e := range c.effects {
		l, r = e.Process(l, r)
	}
	return l, r
}

func (c *Chain) Reset() {
	for _, e := range c.effects {
		e.Reset()
	}
}

func (c *Chain) Add(e Effector) {
	c.effects = append(c.effects, e)
}
