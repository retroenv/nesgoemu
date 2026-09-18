// Package output provides the analog output stage of the APU: the filter chain
// of the NES and the sample queue that playback drains.
// The NES passes the DAC output of the channels through a high-pass and a
// low-pass filter chain before the audio connector.
// https://www.nesdev.org/wiki/APU_Mixer
package output

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/retroenv/nesgoemu/pkg/apu/filter"
)

const (
	// ringFrames is the sample queue capacity in frames, about 93 ms at 44100 Hz.
	// A full queue drops its oldest samples to keep playback latency bounded.
	ringFrames = 4096

	// Filter cutoff frequencies of the output stage in Hz.
	// https://www.nesdev.org/wiki/APU_Mixer
	highPassCutoff1 = 90
	highPassCutoff2 = 440
	lowPassCutoff   = 14000

	// fullScale is the highest value of a 16-bit signed sample.
	fullScale = math.MaxInt16
)

// Stage filters the mixed signal of the APU and holds mono 16-bit signed
// samples for playback. It is safe for concurrent use: the emulation goroutine
// calls Write, and the playback worker calls Fill.
type Stage struct {
	highPass1 *filter.HighPass
	highPass2 *filter.HighPass
	lowPass   *filter.LowPass

	ring []int16
	head int
	tail int
	size int

	mu sync.Mutex
}

// New returns a new output stage for the given sample rate.
func New(sampleRate int) *Stage {
	rate := float64(sampleRate)

	return &Stage{
		highPass1: filter.NewHighPass(highPassCutoff1, rate),
		highPass2: filter.NewHighPass(highPassCutoff2, rate),
		lowPass:   filter.NewLowPass(lowPassCutoff, rate),
		ring:      make([]int16, ringFrames),
	}
}

// Fill fills the destination with mono 16-bit signed little-endian samples.
// The function writes silence when the queue is empty.
// It returns the number of sample frames that came from the queue.
func (s *Stage) Fill(destination []byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(destination)
	frames := len(destination) / 2
	available := min(frames, s.size)

	for index := range available {
		binary.LittleEndian.PutUint16(destination[index*2:], uint16(s.ring[s.head]))
		s.head = (s.head + 1) % len(s.ring)
	}
	s.size -= available

	return available
}

// Queued returns the number of sample frames that wait for playback.
func (s *Stage) Queued() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.size
}

// Write adds one sample of the mixed signal level and removes its direct
// current component. The filter output is centered on zero and can be negative.
func (s *Stage) Write(level float64) {
	level = s.highPass1.Apply(level)
	level = s.highPass2.Apply(level)
	level = s.lowPass.Apply(level)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.push(int16(math.Round(min(max(level, -1), 1) * fullScale)))
}

// push appends one sample and drops the oldest sample when the queue is full.
func (s *Stage) push(sample int16) {
	if s.size == len(s.ring) {
		s.ring[s.tail] = sample
		s.tail = (s.tail + 1) % len(s.ring)
		s.head = s.tail
		return
	}

	s.ring[s.tail] = sample
	s.tail = (s.tail + 1) % len(s.ring)
	s.size++
}
