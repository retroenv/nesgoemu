// Package filter provides the first-order filters of the APU output stage.
// The DAC output of the NES passes a high-pass and a low-pass filter chain
// before it reaches the audio connector.
// https://www.nesdev.org/wiki/APU_Mixer
package filter

import "math"

// HighPass is a first-order high-pass filter.
type HighPass struct {
	coefficient float64
	lastInput   float64
	lastOutput  float64
}

// LowPass is a first-order low-pass filter.
type LowPass struct {
	coefficient float64
	lastOutput  float64
}

// NewHighPass returns a high-pass filter for the given cutoff frequency in Hz.
func NewHighPass(cutoff, sampleRate float64) *HighPass {
	return &HighPass{coefficient: 1 / (1 + 2*math.Pi*cutoff/sampleRate)}
}

// NewLowPass returns a low-pass filter for the given cutoff frequency in Hz.
func NewLowPass(cutoff, sampleRate float64) *LowPass {
	return &LowPass{coefficient: 1 - math.Exp(-2*math.Pi*cutoff/sampleRate)}
}

// Apply filters one sample.
func (f *HighPass) Apply(sample float64) float64 {
	output := f.coefficient * (f.lastOutput + sample - f.lastInput)
	f.lastInput = sample
	f.lastOutput = output

	return output
}

// Apply filters one sample.
func (f *LowPass) Apply(sample float64) float64 {
	f.lastOutput += f.coefficient * (sample - f.lastOutput)

	return f.lastOutput
}
