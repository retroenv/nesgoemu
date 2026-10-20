package nes

import (
	"sync"

	"github.com/retroenv/nesgoemu/pkg/apu"
)

// APU and input register addresses used by the side-effect-free trace reader.
const (
	apuRegisterStart = 0x4000
	apuRegisterEnd   = 0x401F
	apuStatusAddress = 0x4015
	joypad1Address   = 0x4016
	joypad2Address   = 0x4017
)

// defaultTraceCapacity is the default number of stored trace lines.
const defaultTraceCapacity = 10000

// TraceRing is a bounded ring buffer of nestest-format trace lines. It drops
// the oldest line when it is full. Capture is off until SetEnabled turns it on.
type TraceRing struct {
	mu       sync.Mutex
	capacity int
	lines    []string
	next     int
	count    int
	total    uint64
	enabled  bool
}

// NewTraceRing creates a ring with the given capacity. A non-positive capacity
// selects the default of 10000 lines.
func NewTraceRing(capacity int) *TraceRing {
	if capacity <= 0 {
		capacity = defaultTraceCapacity
	}
	return &TraceRing{
		capacity: capacity,
		lines:    make([]string, capacity),
	}
}

// Enabled reports whether the ring captures lines.
func (r *TraceRing) Enabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.enabled
}

// SetEnabled turns capture on or off.
func (r *TraceRing) SetEnabled(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.enabled = enabled
}

// Capacity returns the maximum number of stored lines.
func (r *TraceRing) Capacity() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.capacity
}

// Len returns the number of stored lines.
func (r *TraceRing) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.count
}

// Clear removes every stored line. The total count of added lines stays.
func (r *TraceRing) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lines = make([]string, r.capacity)
	r.next = 0
	r.count = 0
}

// Total returns the number of lines that were ever added. It grows after the
// ring is full, so a caller can use it to find the lines that a run added.
func (r *TraceRing) Total() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.total
}

// LinesSince returns the stored lines that were added after the given total
// count. It returns fewer lines when the ring already dropped some of them.
func (r *TraceRing) LinesSince(total uint64) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.total <= total {
		return nil
	}
	wanted := int(min(r.total-total, uint64(r.count)))
	lines := make([]string, 0, wanted)
	start := 0
	if r.count == r.capacity {
		start = r.next
	}
	for i := r.count - wanted; i < r.count; i++ {
		lines = append(lines, r.lines[(start+i)%r.capacity])
	}
	return lines
}

// Lines returns the stored lines in chronological order.
func (r *TraceRing) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	lines := make([]string, 0, r.count)
	start := 0
	if r.count == r.capacity {
		start = r.next
	}
	for i := range r.count {
		lines = append(lines, r.lines[(start+i)%r.capacity])
	}
	return lines
}

// StepTraced executes one step and captures a trace line when tracing is on.
// It is the traced variant of StepSystem for the debugging tools.
func (sys *System) StepTraced() (StepResult, error) {
	if !sys.tracingEnabled() {
		return sys.StepSystem()
	}

	state := sys.captureTraceState()
	step, err := sys.StepSystem()
	if step.InstructionExecuted {
		sys.printTraceStep(state)
	}
	return step, err
}

// add appends a line and drops the oldest line when full.
func (r *TraceRing) add(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.enabled {
		return
	}
	r.lines[r.next] = line
	r.next = (r.next + 1) % r.capacity
	if r.count < r.capacity {
		r.count++
	}
	r.total++
}

// tracingEnabled reports whether a step captures a trace line.
func (sys *System) tracingEnabled() bool {
	if !sys.opts.tracing {
		return false
	}
	if ring := sys.opts.traceRing; ring != nil {
		return ring.Enabled()
	}
	return true
}

// captureTraceState copies the registers used by the trace line.
func (sys *System) captureTraceState() cpuState {
	return cpuState{
		A:      sys.A,
		X:      sys.X,
		Y:      sys.Y,
		SP:     sys.SP,
		Flags:  sys.GetFlags(),
		Cycles: sys.Cycles(),
	}
}

// tracePeek reads a byte for the trace formatter without side effects. APU
// registers return 0xFF, the $4015 status is read without clearing, and the
// controller registers report false because a read shifts them.
func (sys *System) tracePeek(address uint16) (byte, bool) {
	switch {
	case address == apuStatusAddress:
		device, ok := sys.Bus.APU.(*apu.APU)
		if !ok {
			return 0, false
		}
		return device.PeekStatus(), true

	case address == joypad1Address || address == joypad2Address:
		return 0, false

	case address >= apuRegisterStart && address <= apuRegisterEnd:
		return 0xFF, true

	default:
		return sys.Peek(address)
	}
}
