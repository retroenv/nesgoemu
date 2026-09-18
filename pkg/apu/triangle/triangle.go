// Package triangle provides the APU triangle channel.
// The channel has no volume control. It outputs a 32-step triangle wave or
// holds its last value while a counter silences it.
// https://www.nesdev.org/wiki/APU_Triangle
package triangle

import "github.com/retroenv/nesgoemu/pkg/apu/lengthcounter"

// sequenceTable contains the output of the 32-step sequencer.
// https://www.nesdev.org/wiki/APU_Triangle
var sequenceTable = [32]byte{
	15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

// Triangle provides the triangle channel.
type Triangle struct {
	length *lengthcounter.LengthCounter

	timer       uint16
	counter     uint16
	sequence    byte
	linear      byte
	reloadValue byte
	control     bool
	reload      bool
}

// New returns a new triangle channel.
func New() *Triangle {
	return &Triangle{length: lengthcounter.New()}
}

// Clock advances the channel by one CPU cycle. The sequencer advances when the
// timer reaches zero, so a period of t advances it every t+1 CPU cycles. The
// timer keeps running while the channel is silenced, but the sequencer does not
// advance.
// https://www.nesdev.org/wiki/APU_Triangle
func (t *Triangle) Clock() {
	if t.counter == 0 {
		t.counter = t.timer
		if t.length.Active() && t.linear > 0 {
			t.sequence = (t.sequence + 1) % 32
		}
		return
	}
	t.counter--
}

// ClockHalfFrame advances the length counter.
func (t *Triangle) ClockHalfFrame() {
	t.length.Clock()
}

// ClockQuarterFrame advances the linear counter by one quarter frame.
// https://www.nesdev.org/wiki/APU_Triangle
func (t *Triangle) ClockQuarterFrame() {
	switch {
	case t.reload:
		t.linear = t.reloadValue
	case t.linear > 0:
		t.linear--
	}

	if !t.control {
		t.reload = false
	}
}

// LengthActive reports whether the length counter is not zero.
func (t *Triangle) LengthActive() bool {
	return t.length.Active()
}

// Output returns the current level between 0 and 15. The channel keeps
// outputting the current sequence value also while it is silenced.
func (t *Triangle) Output() byte {
	return sequenceTable[t.sequence]
}

// Reset returns the channel to its power-up state.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (t *Triangle) Reset() {
	t.length.Reset()

	t.timer = 0
	t.counter = 0
	t.sequence = 0
	t.linear = 0
	t.reloadValue = 0
	t.control = false
	t.reload = false
}

// SetEnabled enables or disables the channel. A disabled channel clears its
// length counter.
func (t *Triangle) SetEnabled(enabled bool) {
	t.length.SetEnabled(enabled)
}

// Write sets a channel register. The low two address bits select the register.
// A write to the timer high register loads the length counter and sets the
// linear counter reload flag.
// https://www.nesdev.org/wiki/APU_Triangle
func (t *Triangle) Write(address uint16, value byte) {
	switch address & 0x03 {
	case 0: // control flag and linear counter load
		t.control = value&0x80 != 0
		t.reloadValue = value & 0x7f
		t.length.SetHalt(t.control)

	case 1: // unused

	case 2: // timer low
		t.timer = t.timer&0x700 | uint16(value)

	default: // timer high and length counter load
		t.timer = t.timer&0x0ff | uint16(value&0x07)<<8
		t.length.Load(value)
		t.reload = true
	}
}
