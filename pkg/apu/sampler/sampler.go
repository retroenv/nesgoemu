// Package sampler converts the mixed APU signal into host sample frames.
// A low-pass filter removes ultrasonic components before sample rate conversion.
// https://www.nesdev.org/wiki/Cycle_reference_chart
package sampler

import (
	"math"
	"sync"
)

// ntscCPURate is the NTSC CPU clock rate in Hz: six times the 315/88 MHz
// colorburst, divided by 12.
// https://www.nesdev.org/wiki/Cycle_reference_chart
const (
	ntscClockNumerator   = 19_687_500
	ntscClockDenominator = 11
	ntscCPURate          = float64(ntscClockNumerator) / ntscClockDenominator
	filterPhases         = 32
	filterHalfWidth      = 24
	maxExactPhases       = 512
	maxExactWeights      = 1 << 20 // Limit each combined filter table to 8 MiB.
)

var (
	kernelMutex      sync.Mutex
	kernelCache      = make(map[int][][]float64)
	exactKernelCache = make(map[int][][]float64)
)

// Sampler converts the mixed signal of the APU to the output sample rate and
// calls a sink for every sample. Only the emulation goroutine calls Add.
type Sampler struct {
	accumulator     uint64
	cyclesPerSample float64
	increment       uint64

	coefficients [][]float64 // Shared, immutable fractional-delay filters.
	exactPhases  [][]float64 // Combined filters for a short, repeating sample schedule.
	phaseStep    uint64      // Greatest common divisor of the clock numerator and sample increment.

	history  []float64
	position int // Index of the most recent input sample.

	sink   func(level float64)
	worker *worker
}

// New returns a new sampler that calls the sink once per output sample period.
// The filter delays the signal by approximately 24 output samples. It preserves
// frequencies through 15 kHz at 44.1 and 48 kHz output rates.
// The sample rate must be positive and must not exceed the NTSC CPU rate.
func New(sampleRate int, sink func(level float64)) *Sampler {
	if sampleRate <= 0 || sampleRate > ntscClockNumerator/ntscClockDenominator {
		panic("sample rate must be between 1 Hz and the NTSC CPU rate")
	}
	coefficients := filterCoefficients(sampleRate)
	exactPhases, phaseStep := exactCoefficients(sampleRate, coefficients)
	return &Sampler{
		cyclesPerSample: ntscCPURate / float64(sampleRate),
		increment:       uint64(sampleRate) * ntscClockDenominator,

		coefficients: coefficients,
		exactPhases:  exactPhases,
		phaseStep:    phaseStep,

		history: make([]float64, len(coefficients[0])),

		sink: sink,
	}
}

// Add adds the mixed signal level of one CPU cycle.
func (s *Sampler) Add(level float64) {
	if s.worker != nil {
		s.worker.add(level)
		return
	}
	s.add(level)
}

func (s *Sampler) add(level float64) {
	s.position--
	if s.position < 0 {
		s.position = len(s.history) - 1
	}
	s.history[s.position] = level
	s.accumulator += s.increment
	if s.accumulator < ntscClockNumerator {
		return
	}

	s.accumulator -= ntscClockNumerator
	s.sink(s.filtered())
}

// filtered evaluates the filter at the exact output sample time.
func (s *Sampler) filtered() float64 {
	if s.exactPhases != nil {
		weights := s.exactPhases[s.accumulator/s.phaseStep]
		tail := len(s.history) - s.position
		return dot(s.history[s.position:], weights[:tail]) + dot(s.history[:s.position], weights[tail:])
	}
	phase := float64(s.accumulator) * filterPhases / float64(s.increment)
	index := int(phase)
	fraction := phase - float64(index)
	first, second := s.coefficients[index], s.coefficients[index+1]
	var left, right float64
	coefficient := 0
	for _, sample := range s.history[s.position:] {
		left += sample * first[coefficient]
		right += sample * second[coefficient]
		coefficient++
	}
	for _, sample := range s.history[:s.position] {
		left += sample * first[coefficient]
		right += sample * second[coefficient]
		coefficient++
	}
	return left + (right-left)*fraction
}

// dot processes four samples per iteration to reduce loop overhead in debug builds.
func dot(samples, weights []float64) float64 {
	var sum float64
	index := 0
	for ; index+3 < len(samples); index += 4 {
		sum += samples[index]*weights[index] + samples[index+1]*weights[index+1] +
			samples[index+2]*weights[index+2] + samples[index+3]*weights[index+3]
	}
	for ; index < len(samples); index++ {
		sum += samples[index] * weights[index]
	}
	return sum
}

// exactCoefficients combines adjacent filters once for each reachable sample time.
// Rates with long schedules use interpolation during playback to limit memory use.
func exactCoefficients(sampleRate int, coefficients [][]float64) ([][]float64, uint64) {
	increment := uint64(sampleRate) * ntscClockDenominator
	step, divisor := increment, uint64(ntscClockNumerator)
	for divisor != 0 {
		step, divisor = divisor, step%divisor
	}
	count := increment / step
	if count > maxExactPhases || count > uint64(maxExactWeights/len(coefficients[0])) {
		return nil, 0
	}
	kernelMutex.Lock()
	defer kernelMutex.Unlock()
	if weights, ok := exactKernelCache[sampleRate]; ok {
		return weights, step
	}
	weights := make([][]float64, count)
	for phase := range weights {
		position := float64(uint64(phase)*step) * filterPhases / float64(increment)
		index := int(position)
		fraction := position - float64(index)
		first, second := coefficients[index], coefficients[index+1]
		combined := make([]float64, len(first))
		for i := range combined {
			combined[i] = first[i] + (second[i]-first[i])*fraction
		}
		weights[phase] = combined
	}
	exactKernelCache[sampleRate] = weights
	return weights, step
}

// filterCoefficients constructs a Blackman-windowed sinc filter. The cutoff is
// below the output Nyquist frequency so the transition band cannot alias into
// the passband. Each phase has unit DC gain.
func filterCoefficients(sampleRate int) [][]float64 {
	kernelMutex.Lock()
	defer kernelMutex.Unlock()
	if coefficients, ok := kernelCache[sampleRate]; ok {
		return coefficients
	}

	half := int(math.Ceil(filterHalfWidth * ntscCPURate / float64(sampleRate)))
	cutoff := 0.42 * float64(sampleRate) / ntscCPURate
	coefficients := make([][]float64, filterPhases+1)
	for phase := range coefficients {
		weights := make([]float64, 2*half+1)
		var total float64
		for index := range weights {
			x := float64(index-half) - float64(phase)/filterPhases
			angle := math.Pi * float64(index) / float64(half)
			window := 0.42 - 0.5*math.Cos(angle) + 0.08*math.Cos(2*angle)
			weight := 2 * cutoff
			if x != 0 {
				weight = math.Sin(2*math.Pi*cutoff*x) / (math.Pi * x)
			}
			weights[index] = weight * window
			total += weights[index]
		}
		for index := range weights {
			weights[index] /= total
		}
		coefficients[phase] = weights
	}
	kernelCache[sampleRate] = coefficients
	return coefficients
}
