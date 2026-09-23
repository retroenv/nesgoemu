package sampler

import (
	"math"
	"strconv"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func BenchmarkAdd(b *testing.B) {
	for _, rate := range []int{44100, 48000} {
		b.Run(strconv.Itoa(rate), func(b *testing.B) {
			s := New(rate, func(float64) {})
			b.ResetTimer()
			for i := range b.N {
				s.Add(float64(i&31) / 32)
			}
		})
	}
}

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

func TestAddPreservesDCGain(t *testing.T) {
	samples := collect()
	s := New(44100, samples.add)
	for range 20_000 {
		s.Add(0.25)
	}
	for _, value := range samples.values[100:] {
		assert.LessOrEqual(t, math.Abs(value-0.25), 1e-12)
	}
}

func TestAddCarriesTheFractionalPeriod(t *testing.T) {
	samples := collect()
	s := New(44100, samples.add)

	// The exact NTSC ratio is 3125 CPU cycles per 77 output samples.
	for range 3125 {
		s.Add(1)
	}

	assert.Len(t, samples.values, 77)
}

func TestAddDoesNotDriftOverTime(t *testing.T) {
	const cycles = 4_410_000
	samples := collect()
	s := New(44100, samples.add)

	for range cycles {
		s.Add(0.25)
	}

	expected := int(float64(cycles) / s.cyclesPerSample)
	assert.LessOrEqual(t, math.Abs(float64(len(samples.values)-expected)), 1)
}

func TestFrequencyResponse(t *testing.T) {
	// Measure gain from a sine wave. Expected limits come from the filter contract.
	for _, rate := range []int{44100, 48000} {
		for _, frequency := range []float64{1000, 10000, 15000, 24000, 27965.199, 30000, 55930.398} {
			samples := collect()
			s := New(rate, samples.add)
			cycles := int(float64(rate/5) * s.cyclesPerSample)
			for index := range cycles {
				s.Add(math.Sin(2 * math.Pi * frequency * float64(index) / ntscCPURate))
			}
			var power float64
			steady := samples.values[256:]
			for _, sample := range steady {
				power += sample * sample
			}
			gain := math.Sqrt(2 * power / float64(len(steady)))
			if frequency <= 15000 {
				assert.LessOrEqual(t, math.Abs(gain-1), 0.003, "passband gain")
			} else {
				assert.LessOrEqual(t, gain, 0.0002, "stopband gain")
			}
		}
	}
}

func TestAddMatchesInterpolatedFilter(t *testing.T) {
	for _, rate := range []int{44100, 48000, 96000, 44101, 1789772} {
		t.Run(strconv.Itoa(rate), func(t *testing.T) {
			actual, expected := collect(), collect()
			optimized := New(rate, actual.add)
			reference := New(rate, expected.add)
			reference.exactPhases = nil
			// Cover each sample phase, ring wrap, silence, and signal transitions.
			for cycle := range 30_000 {
				level := 0.0
				if cycle >= 2000 && cycle < 20_000 {
					level = math.Sin(float64(cycle)*0.37) + float64(cycle%31)/31
				}
				optimized.Add(level)
				reference.Add(level)
				assert.Len(t, actual.values, len(expected.values), "sample time")
			}
			assert.Greater(t, len(actual.values), 0)
			for i, value := range actual.values {
				assert.LessOrEqual(t, math.Abs(value-expected.values[i]), 1e-12)
			}
		})
	}
}

func TestExactPhaseSchedule(t *testing.T) {
	for _, test := range []struct {
		rate, phases int
	}{
		{44100, 77},
		{48000, 352},
		{96000, 0},
		{44101, 0},
		{8000, 0},
	} {
		s := New(test.rate, func(float64) {})
		assert.Len(t, s.exactPhases, test.phases)
	}
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
