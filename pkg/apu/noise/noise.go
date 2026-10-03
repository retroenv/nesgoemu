// Package noise provides the APU noise channel.
// The channel generates pseudo random noise from a linear feedback shift
// register with 16 selectable periods.
// https://www.nesdev.org/wiki/APU_Noise
package noise

import (
	"github.com/retroenv/nesgoemu/pkg/apu/envelope"
	"github.com/retroenv/nesgoemu/pkg/apu/lengthcounter"
)

// ntscPeriodTable contains the timer periods in APU cycles. The source table
// gives CPU cycles, which are twice the APU cycles.
// https://www.nesdev.org/wiki/APU_Noise
var ntscPeriodTable = [16]uint16{
	2, 4, 8, 16, 32, 48, 64, 80, 101, 127, 190, 254, 381, 508, 1017, 2034,
}

// powerOnShiftRegister is the value of the shift register at power-up.
// https://www.nesdev.org/wiki/APU_Noise
const powerOnShiftRegister = 1

// Noise provides the noise channel.
type Noise struct {
	envelope *envelope.Envelope
	length   *lengthcounter.LengthCounter

	timer   uint16
	counter uint16
	shift   uint16
	mode    bool
}

// New returns a new noise channel.
func New() *Noise {
	channel := &Noise{
		envelope: envelope.New(),
		length:   lengthcounter.New(),
		shift:    powerOnShiftRegister,
	}
	channel.Reset()

	return channel
}

// Clock advances the channel by one APU cycle. The shift register is clocked
// when the timer reaches zero, so a period of p clocks it every p CPU cycles.
// https://www.nesdev.org/wiki/APU_Noise
func (n *Noise) Clock() {
	if n.counter == 0 {
		n.advance()
		n.counter = n.timer - 1
		return
	}
	n.counter--
}

// ClockQuarterFrame advances the envelope by one quarter frame.
func (n *Noise) ClockQuarterFrame() {
	n.envelope.Clock()
}

// ClockHalfFrame advances the length counter.
func (n *Noise) ClockHalfFrame() {
	n.length.Clock()
}

// CommitLengthWrites applies length register writes after the frame clock.
func (n *Noise) CommitLengthWrites() {
	n.length.Commit()
}

// LengthActive reports whether the length counter is not zero.
func (n *Noise) LengthActive() bool {
	return n.length.Active()
}

// Output returns the channel level between 0 and 15. The channel is muted while
// bit 0 of the shift register is set or the length counter is zero.
// https://www.nesdev.org/wiki/APU_Noise
func (n *Noise) Output() byte {
	if n.shift&1 != 0 || !n.length.Active() {
		return 0
	}

	return n.envelope.Output()
}

// Reset clears the channel counters and period. It keeps the volume and mode.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (n *Noise) Reset() {
	n.envelope.Reset()
	n.length.Reset()

	// Reset clears the period register, which selects the first table entry.
	n.timer = ntscPeriodTable[0]
	n.counter = 0
}

// SetEnabled enables or disables the channel. A disabled channel clears its
// length counter.
func (n *Noise) SetEnabled(enabled bool) {
	n.length.SetEnabled(enabled)
}

// Write sets a channel register. The low two address bits select the register.
// https://www.nesdev.org/wiki/APU_Noise
func (n *Noise) Write(address uint16, value byte) {
	switch address & 0x03 {
	case 0: // length counter halt and envelope
		n.envelope.Write(value)
		n.length.SetHalt(n.envelope.Loop())

	case 1: // unused

	case 2: // mode and period
		n.mode = value&0x80 != 0
		n.timer = ntscPeriodTable[value&0x0f]

	default: // length counter load and envelope restart
		n.length.Load(value)
		n.envelope.Start()
	}
}

// advance clocks the shift register. The feedback is the exclusive or of bit 0
// and bit 1, or bit 6 in short mode. The feedback moves into bit 14.
// https://www.nesdev.org/wiki/APU_Noise
func (n *Noise) advance() {
	tap := n.shift >> 1 & 1
	if n.mode {
		tap = n.shift >> 6 & 1
	}
	feedback := n.shift&1 ^ tap

	n.shift = n.shift>>1 | feedback<<14
}
