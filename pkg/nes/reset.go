package nes

import "github.com/retroenv/nesgoemu/pkg/bus"

// Reset resets the cartridge, APU, PPU controls, and CPU in that order.
// The CPU reads its reset vector after the cartridge returns to its reset bank.
// Call this method while emulation is stopped.
func (sys *System) Reset() {
	sys.dma = dmaController{}
	if mapper, ok := sys.Bus.Mapper.(bus.MapperResetter); ok {
		mapper.Reset()
	}
	if device, ok := sys.Bus.APU.(resetter); ok {
		device.Reset()
	}
	if ppu, ok := sys.Bus.PPU.(resetter); ok {
		ppu.Reset()
	}
	sys.CPU.Reset()
}

type resetter interface {
	Reset()
}
