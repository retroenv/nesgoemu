// Package lengthcounter provides the APU length counter unit.
// The counter controls how long a channel plays after it starts.
// https://www.nesdev.org/wiki/APU_Length_Counter
package lengthcounter

// lengthLoadTable contains the counter load values for the five high bits of a
// length register. The unit is half frames.
// https://www.nesdev.org/wiki/APU_Length_Counter
var lengthLoadTable = [32]byte{
	10, 254, 20, 2, 40, 4, 80, 6,
	160, 8, 60, 10, 14, 12, 26, 14,
	12, 16, 24, 18, 48, 20, 96, 22,
	192, 24, 72, 26, 16, 28, 32, 30,
}

// LengthCounter counts half frames and mutes its channel at zero.
type LengthCounter struct {
	counter byte
	enabled bool
	halt    bool

	load     bool
	previous byte
	reload   byte

	nextHalt bool
}

// New returns a new length counter.
func New() *LengthCounter {
	return &LengthCounter{}
}

// Active reports whether the counter is not zero.
func (l *LengthCounter) Active() bool {
	return l.counter > 0
}

// Clock decrements the counter by one half frame unless it is halted.
func (l *LengthCounter) Clock() {
	if l.halt || l.counter == 0 {
		return
	}
	l.counter--
}

// Commit applies register writes after the frame clock of the next CPU cycle.
// A simultaneous decrement cancels a reload of a nonzero counter.
// https://www.nesdev.org/wiki/APU_Length_Counter
func (l *LengthCounter) Commit() {
	if l.load && l.counter == l.previous {
		l.counter = l.reload
	}
	l.load = false
	l.halt = l.nextHalt
}

// Load schedules a counter load from bits 3 to 7 of the written value.
// A disabled channel ignores the load.
func (l *LengthCounter) Load(value byte) {
	if !l.enabled {
		return
	}
	l.load = true
	l.previous = l.counter
	l.reload = lengthLoadTable[value>>3]
}

// Reset disables the counter and keeps the halt register setting.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (l *LengthCounter) Reset() {
	l.counter = 0
	l.enabled = false
	l.load = false
	l.nextHalt = l.halt
}

// SetEnabled enables the channel. A disabled channel clears its counter.
func (l *LengthCounter) SetEnabled(enabled bool) {
	l.enabled = enabled
	if !enabled {
		l.counter = 0
		l.load = false
	}
}

// SetHalt schedules the halt input for the next Commit.
func (l *LengthCounter) SetHalt(halt bool) {
	l.nextHalt = halt
}
