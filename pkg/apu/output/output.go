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
// samples for playback. One sample producer calls Write. The playback worker
// can call Fill concurrently with that producer.
type Stage struct {
	highPass1 *filter.HighPass
	highPass2 *filter.HighPass
	lowPass   *filter.LowPass

	ring []int16
	head int
	tail int
	size int

	produced uint64
	dropped  uint64
	silence  uint64

	mu sync.Mutex
}

// Stats reports sample frames produced, queued, dropped, or replaced by silence.
type Stats struct {
	// Produced is the total number of generated frames.
	Produced uint64
	// Queued is the number of frames that wait for playback.
	Queued int
	// Dropped is the number of frames removed because the queue was full.
	Dropped uint64
	// Silence is the number of playback frames supplied from an empty queue.
	Silence uint64
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
	return s.fill(destination, true)
}

// Drain removes queued frames for a recording. An empty queue does not count
// as missing playback because the caller can run more emulation steps.
func (s *Stage) Drain(destination []byte) int {
	return s.fill(destination, false)
}

// Queued returns the number of sample frames that wait for playback.
func (s *Stage) Queued() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.size
}

// Stats returns a consistent snapshot of the queue counters.
func (s *Stage) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{
		Produced: s.produced,
		Queued:   s.size,
		Dropped:  s.dropped,
		Silence:  s.silence,
	}
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

func (s *Stage) fill(destination []byte, playback bool) int {
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
	if playback {
		s.silence += uint64(frames - available)
	}
	return available
}

// push appends one sample and drops the oldest sample when the queue is full.
func (s *Stage) push(sample int16) {
	s.produced++
	if s.size == len(s.ring) {
		s.dropped++
		s.ring[s.tail] = sample
		s.tail = (s.tail + 1) % len(s.ring)
		s.head = s.tail
		return
	}

	s.ring[s.tail] = sample
	s.tail = (s.tail + 1) % len(s.ring)
	s.size++
}
