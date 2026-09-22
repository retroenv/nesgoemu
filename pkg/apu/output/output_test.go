package output

import (
	"encoding/binary"
	"math"
	"sync"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestFillWritesSilenceWhenEmpty(t *testing.T) {
	stage := New(44100)
	buffer := make([]byte, 8)

	written := stage.Fill(buffer)

	assert.Equal(t, 0, written)
	assert.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0}, buffer)
}

func TestQueueStatistics(t *testing.T) {
	stage := New(44100)
	stage.Drain(make([]byte, 8))
	assert.Equal(t, uint64(0), stage.Stats().Silence)
	stage.Fill(make([]byte, 8))
	assert.Equal(t, uint64(4), stage.Stats().Silence)
	for range ringFrames + 3 {
		stage.Write(0)
	}
	assert.Equal(t, Stats{
		Produced: ringFrames + 3,
		Queued:   ringFrames,
		Dropped:  3,
		Silence:  4,
	}, stage.Stats())
	stage.Fill(make([]byte, 10))
	assert.Equal(t, ringFrames-5, stage.Stats().Queued)
}

func TestWriteQueuesSamples(t *testing.T) {
	stage := New(44100)

	for range 100 {
		stage.Write(0.5)
	}

	assert.Equal(t, 100, stage.Queued())
	buffer := make([]byte, 100*2)
	assert.Equal(t, 100, stage.Fill(buffer))
	assert.Equal(t, 0, stage.Queued())
}

func TestWriteRemovesDirectCurrent(t *testing.T) {
	stage := New(44100)
	for range ringFrames {
		stage.Write(1)
	}
	buffer := make([]byte, ringFrames*2)
	stage.Fill(buffer)

	assert.Greater(t, sampleAt(buffer, 0), 20000, "the first sample passes")
	assert.Less(t, math.Abs(float64(sampleAt(buffer, ringFrames-1))), 100, "the level decays to zero")
}

func TestQueueKeepsNewestSamples(t *testing.T) {
	stage := New(44100)
	const total = ringFrames + 1000
	for index := range total {
		stage.push(int16(index))
	}
	buffer := make([]byte, (ringFrames+1)*2)

	written := stage.Fill(buffer)

	assert.Equal(t, ringFrames, written, "the queue is bounded")
	assert.Equal(t, total-ringFrames, sampleAt(buffer, 0), "the oldest samples are dropped")
	assert.Equal(t, total-1, sampleAt(buffer, ringFrames-1))
	assert.Equal(t, 0, sampleAt(buffer, ringFrames), "dropped samples are not queued")
}

func TestConcurrentWriteAndFill(t *testing.T) {
	stage := New(44100)
	buffer := make([]byte, 512)

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		for range 100_000 {
			stage.Write(0.5)
		}
	}()
	go func() {
		defer waitGroup.Done()
		for range 1000 {
			stage.Fill(buffer)
		}
	}()
	waitGroup.Wait()

	assert.LessOrEqual(t, stage.Queued(), ringFrames)
}

func sampleAt(buffer []byte, frame int) int {
	return int(int16(binary.LittleEndian.Uint16(buffer[frame*2:])))
}
