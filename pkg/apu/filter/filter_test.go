package filter

import (
	"math"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

const sampleRate = 44100.0

func TestHighPassRemovesDirectCurrent(t *testing.T) {
	f := NewHighPass(440, sampleRate)

	output := 0.0
	for range 44100 {
		output = f.Apply(1)
	}

	assert.Less(t, math.Abs(output), 0.01)
}

func TestHighPassPassesAudioFrequencies(t *testing.T) {
	f := NewHighPass(90, sampleRate)

	assert.Greater(t, measureGain(f.Apply, 1000), 0.95)
}

func TestHighPassAttenuatesLowFrequencies(t *testing.T) {
	f := NewHighPass(440, sampleRate)

	assert.Less(t, measureGain(f.Apply, 100), 0.25)
}

func TestLowPassPassesDirectCurrent(t *testing.T) {
	f := NewLowPass(14000, sampleRate)

	output := 0.0
	for range 44100 {
		output = f.Apply(1)
	}

	assert.Greater(t, output, 0.99)
}

func TestLowPassPassesAudioFrequencies(t *testing.T) {
	f := NewLowPass(14000, sampleRate)

	assert.Greater(t, measureGain(f.Apply, 1000), 0.95)
}

func TestLowPassAttenuatesUltrasonicFrequencies(t *testing.T) {
	f := NewLowPass(14000, sampleRate)

	// A first-order filter reduces a 20 kHz signal by about 4.5 dB.
	assert.Less(t, measureGain(f.Apply, 20000), 0.8)
}

// measureGain returns the amplitude that the filter passes for a sine wave.
func measureGain(apply func(float64) float64, frequency float64) float64 {
	settle := int(sampleRate)
	amplitude := 0.0

	for index := range settle * 2 {
		sample := math.Sin(2 * math.Pi * frequency * float64(index) / sampleRate)
		output := apply(sample)
		if index >= settle {
			amplitude = math.Max(amplitude, math.Abs(output))
		}
	}

	return amplitude
}
