// Package openbus provides the decay register of the PPU I/O bus.
//
// A write to a PPU port refreshes the register with the written value. A bit
// that is not refreshed with a one for about 600 milliseconds decays to zero.
// The decay time varies with the NES and the temperature.
//
// Sources:
//   - https://www.nesdev.org/wiki/Open_bus_behavior#PPU_open_bus
//   - https://github.com/christopherpow/nes-test-roms/blob/master/ppu_open_bus/readme.txt
package openbus

// DecayCycles is the number of PPU cycles after which a bit decays to zero
// when it is not refreshed with a one. The value is 600 milliseconds at the
// NTSC PPU clock of 5.369318 MHz.
const DecayCycles = 3_221_591

// Register implements the decay register of the PPU I/O bus.
type Register struct {
	value  byte
	stamps [8]uint64 // time of the last refresh with a one for each bit
	clock  uint64
}

// New returns a new register with all bits clear.
func New() *Register {
	return &Register{}
}

// Tick advances the clock by the given number of PPU cycles.
func (r *Register) Tick(cycles int) {
	r.clock += uint64(cycles)
}

// Set refreshes all bits with the given value.
func (r *Register) Set(value byte) {
	r.SetBits(0xFF, value)
}

// SetBits refreshes the bits in mask with the bits of the value. The other
// bits keep their value and are not refreshed.
func (r *Register) SetBits(mask, value byte) {
	for i := range 8 {
		bit := byte(1) << i
		if mask&bit == 0 {
			continue
		}

		if value&bit == 0 {
			r.value &^= bit
			continue
		}

		r.value |= bit
		r.stamps[i] = r.clock
	}
}

// Value returns the register value. The function clears the bits that were not
// refreshed with a one within the decay time.
func (r *Register) Value() byte {
	for i := range 8 {
		bit := byte(1) << i
		if r.value&bit == 0 {
			continue
		}

		if r.clock-r.stamps[i] >= DecayCycles {
			r.value &^= bit
		}
	}

	return r.value
}
