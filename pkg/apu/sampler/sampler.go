// Package sampler converts the mixed APU signal into host sample frames.
// The APU signal advances one step per CPU cycle, so the sampler averages the
// signal over each output sample period. The average removes the aliasing that
// reading a single cycle would add.
// https://www.nesdev.org/wiki/Cycle_reference_chart
package sampler

// ntscCPURate is the NTSC CPU clock rate in Hz: six times the 315/88 MHz
// colorburst, divided by 12.
// https://www.nesdev.org/wiki/Cycle_reference_chart
const ntscCPURate = 315.0 / 88.0 * 6 / 12 * 1_000_000

// Sampler converts the mixed signal of the APU to the output sample rate and
// calls a sink for every sample. Only the emulation goroutine calls Add.
type Sampler struct {
	cyclesPerSample float64
	accumulator     float64
	sum             float64
	window          int
	sink            func(level float64)
}

// New returns a new sampler that calls the sink once per output sample period.
// The sink receives the average signal level of that period.
func New(sampleRate int, sink func(level float64)) *Sampler {
	return &Sampler{
		cyclesPerSample: ntscCPURate / float64(sampleRate),
		sink:            sink,
	}
}

// Add adds the mixed signal level of one CPU cycle.
func (s *Sampler) Add(level float64) {
	s.sum += level
	s.window++
	s.accumulator++
	if s.accumulator < s.cyclesPerSample {
		return
	}

	s.accumulator -= s.cyclesPerSample
	s.sink(s.sum / float64(s.window))
	s.sum = 0
	s.window = 0
}
