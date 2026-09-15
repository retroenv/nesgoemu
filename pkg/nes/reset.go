package nes

import "github.com/retroenv/nesgoemu/pkg/bus"

// Reset resets the cartridge, PPU controls, and CPU in that order.
// The CPU reads its reset vector after the cartridge returns to its reset bank.
// Call this method while emulation is stopped.
func (sys *System) Reset() {
	if mapper, ok := sys.Bus.Mapper.(bus.MapperResetter); ok {
		mapper.Reset()
	}
	if ppu, ok := sys.Bus.PPU.(interface{ Reset() }); ok {
		ppu.Reset()
	}
	sys.CPU.Reset()
}
