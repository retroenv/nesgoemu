package memory

// WriteEvent records one CPU bus write before it changes bus or device state.
// CPUCycles and Frame are the current counters when the callback runs.
type WriteEvent struct {
	CPUCycles uint64
	Frame     uint64

	Address uint16
	Value   byte
}

// ObserveWrites replaces the optional CPU bus write observer.
// The emulation goroutine calls it before it applies each write.
// A nil observer disables it. The observer must not change system state.
func (m *Memory) ObserveWrites(observer func(WriteEvent)) {
	m.writeObserver = observer
}
