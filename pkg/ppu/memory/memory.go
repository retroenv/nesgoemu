// Package memory provides PPU memory access.
package memory

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
)

// Memory implements PPU memory support.
type Memory struct {
	mapper    bus.Mapper
	nametable bus.NameTable
	palette   cpu6502.BasicMemory
}

// New returns a new memory manager.
func New(mapper bus.Mapper, nametable bus.NameTable, palette cpu6502.BasicMemory) *Memory {
	return &Memory{
		mapper:    mapper,
		nametable: nametable,
		palette:   palette,
	}
}

// Read from a PPU memory address.
func (m *Memory) Read(address uint16) uint8 {
	address &= nes.PPUAddressMask

	switch {
	case address < nes.PatternTableEndExclusive:
		return m.mapper.Read(address)

	case address < register.PALETTE_START:
		return m.nametable.Read(address)

	default:
		return m.palette.Read(address)
	}
}

// Write to a PPU memory address.
func (m *Memory) Write(address uint16, value uint8) {
	address &= nes.PPUAddressMask

	switch {
	case address < nes.PatternTableEndExclusive:
		m.mapper.Write(address, value)

	case address < register.PALETTE_START:
		m.nametable.Write(address, value)

	default:
		m.palette.Write(address, value)
	}
}
