// Package status handles PPU status fields.
package status

// Status implements a PPU status fields manager.
type Status struct {
	spriteOverflow bool // 0010 0000
	spriteZeroHit  bool // 0100 0000
	verticalBlank  bool // 1000 0000
}

// New returns a new status manager.
func New() *Status {
	return &Status{}
}

// Value returns the status flags encoded as byte. Bits 4-0 are not defined by
// the status. They keep the value of the PPU open bus.
func (s *Status) Value() byte {
	var value byte
	if s.spriteOverflow {
		value |= 1 << 5
	}
	if s.spriteZeroHit {
		value |= 1 << 6
	}
	if s.verticalBlank {
		value |= 1 << 7
	}
	return value
}

// SetSpriteOverflow sets the sprite overflow flag.
func (s *Status) SetSpriteOverflow(value bool) {
	s.spriteOverflow = value
}

// SetSpriteZeroHit sets the sprite zero hit flag.
func (s *Status) SetSpriteZeroHit(value bool) {
	s.spriteZeroHit = value
}

// SetVerticalBlank sets the vertical blank flag.
func (s *Status) SetVerticalBlank(value bool) {
	s.verticalBlank = value
}
