package sampler

import (
	"math"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func BenchmarkWorker(b *testing.B) {
	s := New(44100, func(float64) {})
	s.StartWorker()
	b.ResetTimer()
	for i := range b.N {
		s.Add(float64(i&31) / 32)
	}
	s.StopWorker()
}

func TestWorkerPreservesSamples(t *testing.T) {
	actual, expected := collect(), collect()
	async, reference := New(44100, actual.add), New(44100, expected.add)
	// Include an empty run, full batches, a partial batch, and worker restarts.
	for _, cycles := range []int{0, workerBatchSize, workerBatchSize*9 + 137, 1} {
		async.StartWorker()
		async.StartWorker()
		for i := range cycles {
			level := math.Sin(float64(i) * 0.13)
			async.Add(level)
			reference.Add(level)
		}
		async.StopWorker()
		async.StopWorker()
		assert.Equal(t, expected.values, actual.values)
		// Continue with synchronous processing after the worker stops.
		for range 100 {
			async.Add(0.25)
			reference.Add(0.25)
		}
		assert.Equal(t, expected.values, actual.values)
	}
}
