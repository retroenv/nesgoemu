// Package framecounter provides the APU frame counter.
// The counter generates the low frequency clocks for the channel units and can
// assert the frame interrupt.
// https://www.nesdev.org/wiki/APU_Frame_Counter
package framecounter

// resetDelayCycles is the number of CPU cycles between a write to $4017 and the
// reset of the sequencer. The hardware uses three or four cycles, depending on
// the APU cycle of the write.
// https://www.nesdev.org/wiki/APU_Frame_Counter
const resetDelayCycles = 3

// step describes the actions of one step of the frame sequence.
type step struct {
	cycle   uint64 // CPU cycle of the step
	quarter bool   // clock the envelopes and the triangle linear counter
	half    bool   // clock the length counters and the sweep units
	irq     bool   // set the frame interrupt flag
}

// The source gives the sequence in APU cycles. The value is twice the APU cycle
// value, plus one for the extra CPU cycle of the clock signals.
// https://www.nesdev.org/wiki/APU_Frame_Counter
var (
	fourStepSequence = [...]step{
		{cycle: 7457, quarter: true},
		{cycle: 14913, quarter: true, half: true},
		{cycle: 22371, quarter: true},
		{cycle: 29828, irq: true},
		{cycle: 29829, quarter: true, half: true},
	}
	fiveStepSequence = [...]step{
		{cycle: 7457, quarter: true},
		{cycle: 14913, quarter: true, half: true},
		{cycle: 22371, quarter: true},
		{cycle: 37281, quarter: true, half: true},
	}
)

// FrameCounter generates the low frequency clocks of the APU.
type FrameCounter struct {
	step    int
	cycles  uint64
	delay   byte
	mode    bool
	inhibit bool
	irq     bool
}

// New returns a new frame counter.
func New() *FrameCounter {
	return &FrameCounter{}
}

// ClearIRQ clears the frame interrupt flag.
func (f *FrameCounter) ClearIRQ() {
	f.irq = false
}

// Clock advances the counter by one CPU cycle. It reports the frame units to
// clock after this cycle.
// https://www.nesdev.org/wiki/APU_Frame_Counter
func (f *FrameCounter) Clock() (quarter, half bool) {
	if f.delay > 0 {
		f.delay--
		if f.delay > 0 {
			return false, false
		}
		f.restart()
		if f.mode {
			// The 5-step mode clocks both units at the start of the sequence.
			return true, true
		}
		return false, false
	}

	f.cycles++
	current := f.current()
	if f.cycles != current.cycle {
		return false, false
	}

	if current.irq && !f.inhibit {
		f.irq = true
	}

	f.step++
	if f.step == f.sequenceLength() {
		f.restart()
	}

	return current.quarter, current.half
}

// IRQ reports whether the frame interrupt flag is set.
func (f *FrameCounter) IRQ() bool {
	return f.irq
}

// Reset returns the counter to its power-up state.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (f *FrameCounter) Reset() {
	f.step = 0
	f.cycles = 0
	f.delay = 0
	f.mode = false
	f.inhibit = false
	f.irq = false
}

// Write sets the mode and the interrupt inhibit flag, and it schedules a reset
// of the sequencer. The reset occurs after a short delay. A write with the
// inhibit flag set clears the frame interrupt flag.
// https://www.nesdev.org/wiki/APU_Frame_Counter
func (f *FrameCounter) Write(value byte) {
	f.mode = value&0x80 != 0
	f.inhibit = value&0x40 != 0
	if f.inhibit {
		f.irq = false
	}
	f.delay = resetDelayCycles
}

// current returns the step that the counter is at.
func (f *FrameCounter) current() step {
	if f.mode {
		return fiveStepSequence[f.step]
	}

	return fourStepSequence[f.step]
}

// sequenceLength returns the number of steps of the current mode.
func (f *FrameCounter) sequenceLength() int {
	if f.mode {
		return len(fiveStepSequence)
	}

	return len(fourStepSequence)
}

// restart returns the sequence to its first step.
func (f *FrameCounter) restart() {
	f.step = 0
	f.cycles = 0
}
