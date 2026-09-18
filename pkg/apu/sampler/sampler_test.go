package sampler

import (
	"math"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestNewUsesNTSCRateRatio(t *testing.T) {
	s := New(44100, func(float64) {})

	assert.Equal(t, 40.584415584, round(s.cyclesPerSample, 9))
}

func TestAddEmitsOneSamplePerPeriod(t *testing.T) {
	const cycles = 441_000
	samples := collect()
	s := New(44100, samples.add)

	for range cycles {
		s.Add(0.5)
	}

	assert.Len(t, samples.values, int(float64(cycles)/s.cyclesPerSample))
}

func TestAddAveragesOverTheSamplePeriod(t *testing.T) {
	samples := collect()
	s := New(44100, samples.add)
	s.cyclesPerSample = 4

	s.Add(0)
	s.Add(0.5)
	s.Add(1)
	s.Add(0.5)

	assert.Equal(t, []float64{0.5}, samples.values)
}

func TestAddCarriesThePartialPeriod(t *testing.T) {
	samples := collect()
	s := New(44100, samples.add)
	s.cyclesPerSample = 3

	for range 8 {
		s.Add(1)
	}

	assert.Equal(t, []float64{1, 1}, samples.values)
}

func TestAddDoesNotDriftOverTime(t *testing.T) {
	const cycles = 4_410_000
	samples := collect()
	s := New(44100, samples.add)

	for range cycles {
		s.Add(0.25)
	}

	expected := int(float64(cycles) / s.cyclesPerSample)
	assert.LessOrEqual(t, math.Abs(float64(len(samples.values)-expected)), 1, "10 seconds at 44100 Hz")
}

// sampleSink collects the samples that a sampler produces.
type sampleSink struct {
	values []float64
}

func collect() *sampleSink {
	return &sampleSink{}
}

func (s *sampleSink) add(level float64) {
	s.values = append(s.values, level)
}

func round(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}
