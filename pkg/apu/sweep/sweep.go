// Package sweep provides the APU pulse channel sweep unit.
// The unit adjusts the period of a pulse channel at half frame intervals and
// mutes the channel while a period is out of range.
// https://www.nesdev.org/wiki/APU_Sweep
package sweep

// NegateMode selects how the unit subtracts the change amount. The two pulse
// channels have the carry inputs of their adders wired differently.
// https://www.nesdev.org/wiki/APU_Sweep
type NegateMode uint8

const (
	// OnesComplement subtracts the change amount plus one. Pulse 1 uses it.
	OnesComplement NegateMode = iota
	// TwosComplement subtracts the change amount. Pulse 2 uses it.
	TwosComplement
)

// Sweep provides period changes and muting for the pulse channels.
type Sweep struct {
	mode NegateMode

	divider byte
	period  byte
	shift   byte

	enabled bool
	negate  bool
	reload  bool
}

// New returns a new sweep unit for the given negation mode.
func New(mode NegateMode) *Sweep {
	return &Sweep{mode: mode}
}

// Clock advances the unit by one half frame and returns the new channel period.
// The divider counts down and reloads in every case. The period changes only
// when the divider reaches zero, the unit is enabled, the shift count is not
// zero, and the unit does not mute the channel.
// https://www.nesdev.org/wiki/APU_Sweep
func (s *Sweep) Clock(period uint16) uint16 {
	switch {
	case s.reload:
		s.reload = false
		s.divider = s.period

	case s.divider > 0:
		s.divider--

	default:
		s.divider = s.period
		if s.enabled && s.shift > 0 && !s.Muted(period) {
			return uint16(s.target(period))
		}
	}

	return period
}

// Muted reports whether the unit mutes the channel. Muting applies whether the
// unit is enabled or not, and whether the divider outputs a clock or not.
func (s *Sweep) Muted(period uint16) bool {
	if period < 8 {
		return true
	}

	return s.target(period) > 0x7ff
}

// Reset returns the unit to its power-up state.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (s *Sweep) Reset() {
	mode := s.mode
	*s = Sweep{mode: mode}
}

// Write sets the unit parameters from the sweep register value.
// Bit 7 enables the unit, bits 4 to 6 set the divider period, bit 3 selects
// negation, and bits 0 to 2 set the shift count. A write reloads the divider.
func (s *Sweep) Write(value byte) {
	s.enabled = value&0x80 != 0
	s.period = (value >> 4) & 0x07
	s.negate = value&0x08 != 0
	s.shift = value & 0x07
	s.reload = true
}

// target returns the period that the unit selects next. A negative sum is
// clamped to zero.
func (s *Sweep) target(period uint16) int {
	change := int(period >> s.shift)
	if s.negate {
		if s.mode == OnesComplement {
			change = -change - 1
		} else {
			change = -change
		}
	}

	return max(int(period)+change, 0)
}
