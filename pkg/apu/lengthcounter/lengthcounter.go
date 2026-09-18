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

// Load sets the counter from bits 3 to 7 of the written value.
// A disabled channel keeps a counter of zero.
func (l *LengthCounter) Load(value byte) {
	if !l.enabled {
		return
	}
	l.counter = lengthLoadTable[value>>3]
}

// Reset returns the counter to its power-up state.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (l *LengthCounter) Reset() {
	l.counter = 0
	l.enabled = false
	l.halt = false
}

// SetEnabled enables the channel. A disabled channel clears its counter.
func (l *LengthCounter) SetEnabled(enabled bool) {
	l.enabled = enabled
	if !enabled {
		l.counter = 0
	}
}

// SetHalt sets the halt input of the counter.
func (l *LengthCounter) SetHalt(halt bool) {
	l.halt = halt
}
