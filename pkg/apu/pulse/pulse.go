// Package pulse provides the APU pulse channels.
// The NES has two pulse channels with the same behavior. They differ only in
// the change amount that the sweep unit subtracts when it negates the period.
// https://www.nesdev.org/wiki/APU_Pulse
package pulse

import (
	"github.com/retroenv/nesgoemu/pkg/apu/envelope"
	"github.com/retroenv/nesgoemu/pkg/apu/lengthcounter"
	"github.com/retroenv/nesgoemu/pkg/apu/sweep"
)

// dutySequences contains the output of the four duty cycles. The sequencer
// reads the table from the first entry downward, which produces the duty cycle
// of the channel.
// https://www.nesdev.org/wiki/APU_Pulse
var dutySequences = [4][8]byte{
	{0, 0, 0, 0, 0, 0, 0, 1}, // 12.5%
	{0, 0, 0, 0, 0, 0, 1, 1}, // 25%
	{0, 0, 0, 0, 1, 1, 1, 1}, // 50%
	{1, 1, 1, 1, 1, 1, 0, 0}, // 25% negated
}

// Pulse provides one pulse channel.
type Pulse struct {
	envelope *envelope.Envelope
	length   *lengthcounter.LengthCounter
	sweep    *sweep.Sweep

	duty     byte
	sequence byte
	timer    uint16
	counter  uint16
}

// New returns a new pulse channel. The negation mode selects the sweep
// behavior of pulse 1 or pulse 2.
func New(mode sweep.NegateMode) *Pulse {
	return &Pulse{
		envelope: envelope.New(),
		length:   lengthcounter.New(),
		sweep:    sweep.New(mode),
	}
}

// Clock advances the channel by one APU cycle. The sequencer advances when the
// timer reaches zero, so a period of t advances it every t+1 APU cycles.
// https://www.nesdev.org/wiki/APU_Pulse
func (p *Pulse) Clock() {
	if p.counter == 0 {
		p.counter = p.timer
		p.sequence = (p.sequence + 7) % 8
		return
	}
	p.counter--
}

// ClockHalfFrame advances the length counter and applies the sweep unit.
func (p *Pulse) ClockHalfFrame() {
	p.length.Clock()
	p.timer = p.sweep.Clock(p.timer)
}

// ClockQuarterFrame advances the envelope by one quarter frame.
func (p *Pulse) ClockQuarterFrame() {
	p.envelope.Clock()
}

// LengthActive reports whether the length counter is not zero.
func (p *Pulse) LengthActive() bool {
	return p.length.Active()
}

// Output returns the channel level between 0 and 15. The channel is muted while
// the length counter is zero or the sweep unit mutes it.
// https://www.nesdev.org/wiki/APU_Pulse
func (p *Pulse) Output() byte {
	if !p.length.Active() || p.sweep.Muted(p.timer) {
		return 0
	}

	return dutySequences[p.duty][p.sequence] * p.envelope.Output()
}

// Reset returns the channel to its power-up state.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (p *Pulse) Reset() {
	p.envelope.Reset()
	p.length.Reset()
	p.sweep.Reset()

	p.duty = 0
	p.sequence = 0
	p.timer = 0
	p.counter = 0
}

// SetEnabled enables or disables the channel. A disabled channel clears its
// length counter.
func (p *Pulse) SetEnabled(enabled bool) {
	p.length.SetEnabled(enabled)
}

// Write sets a channel register. The low two address bits select the register.
// A write to the timer high register restarts the sequencer and the envelope,
// and it loads the length counter.
// https://www.nesdev.org/wiki/APU_Pulse
func (p *Pulse) Write(address uint16, value byte) {
	switch address & 0x03 {
	case 0: // duty, length counter halt and envelope
		p.duty = value >> 6
		p.envelope.Write(value)
		p.length.SetHalt(p.envelope.Loop())

	case 1: // sweep
		p.sweep.Write(value)

	case 2: // timer low
		p.timer = p.timer&0x700 | uint16(value)

	default: // timer high and length counter load
		p.timer = p.timer&0x0ff | uint16(value&0x07)<<8
		p.length.Load(value)
		p.sequence = 0
		p.envelope.Start()
	}
}
