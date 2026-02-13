package effects

import "math"

// Distortion implements waveshaping distortion with pre/post gain and LPF.
type Distortion struct {
	preGain  float32
	postGain float32
	lpfAlpha float32
	lpfL     float32
	lpfR     float32
}

// NewDistortion creates a distortion effect.
// preGain: input gain (higher = more distortion)
// postGain: output gain
// lpfCutoff: lowpass filter cutoff in Hz (0 = no filter)
func NewDistortion(sampleRate int, preGain, postGain, lpfCutoff float32) *Distortion {
	d := &Distortion{
		preGain:  preGain,
		postGain: postGain,
	}
	if lpfCutoff > 0 && lpfCutoff < float32(sampleRate)/2 {
		rc := 1.0 / (2.0 * math.Pi * float64(lpfCutoff))
		dt := 1.0 / float64(sampleRate)
		d.lpfAlpha = float32(dt / (rc + dt))
	}
	return d
}

func (d *Distortion) Process(l, r float32) (float32, float32) {
	l *= d.preGain
	r *= d.preGain
	// Soft clipping via tanh waveshaping
	l = float32(math.Tanh(float64(l)))
	r = float32(math.Tanh(float64(r)))
	l *= d.postGain
	r *= d.postGain
	if d.lpfAlpha > 0 {
		d.lpfL += d.lpfAlpha * (l - d.lpfL)
		d.lpfR += d.lpfAlpha * (r - d.lpfR)
		l = d.lpfL
		r = d.lpfR
	}
	return l, r
}

func (d *Distortion) Reset() {
	d.lpfL = 0
	d.lpfR = 0
}
