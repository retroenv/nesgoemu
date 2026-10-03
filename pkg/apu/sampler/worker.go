package sampler

// Each batch holds about 4.6 ms of CPU-cycle samples. Three buffers bound the
// queue and let the producer fill one buffer while the worker reads another.
const workerBatchSize = 8192

// StartWorker moves sample filtering to a worker goroutine. The caller must
// use the same goroutine for StartWorker, Add, and StopWorker. The sink must
// permit calls from the worker until StopWorker returns.
func (s *Sampler) StartWorker() {
	if s.worker != nil {
		return
	}
	w := &worker{
		done:    make(chan struct{}),
		free:    make(chan []float64, 3),
		jobs:    make(chan []float64, 2),
		pending: make([]float64, workerBatchSize),
	}
	for range 2 {
		w.free <- make([]float64, workerBatchSize)
	}
	s.worker = w
	go w.run(s)
}

// StopWorker processes all pending samples and waits for the worker to stop.
// Subsequent calls to Add process samples on the caller's goroutine.
func (s *Sampler) StopWorker() {
	w := s.worker
	if w == nil {
		return
	}
	if w.used > 0 {
		w.jobs <- w.pending[:w.used]
	}
	close(w.jobs)
	<-w.done
	s.worker = nil
}

type worker struct {
	done chan struct{}
	free chan []float64
	jobs chan []float64

	pending []float64
	used    int
}

func (w *worker) add(level float64) {
	w.pending[w.used] = level
	w.used++
	if w.used == len(w.pending) {
		w.jobs <- w.pending
		w.pending = <-w.free
		w.used = 0
	}
}

func (w *worker) run(s *Sampler) {
	defer close(w.done)
	// Keep the filter counters separate from the producer's worker pointer.
	// Otherwise, writes to the counters invalidate the producer's cache line.
	local := *s
	for batch := range w.jobs {
		for _, level := range batch {
			local.add(level)
		}
		w.free <- batch
	}
	s.accumulator = local.accumulator
	s.position = local.position
}
