// Package memory provides Memory functionality.
package memory

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/retrogolib/arch/nes/register"
)

// Memory represents the memory controller.
type Memory struct {
	bus *bus.Bus
	ram *RAM
}

// New returns a new memory instance, embedded it has
// new instances for PPU and the Controllers.
func New(bus *bus.Bus) *Memory {
	return &Memory{
		bus: bus,
		ram: NewRAM(0, 0x2000),
	}
}

// Write a byte to a memory address.
func (m *Memory) Write(address uint16, value byte) {
	switch {
	case address < register.PPU_CTRL:
		m.ram.Write(address&0x07FF, value)

	case address < register.APU_PL1_VOL:
		m.bus.PPU.Write(address, value)

	case address == register.OAM_DMA:
		m.bus.PPU.Write(address, value)

	case address == register.JOYPAD1:
		m.bus.Controller1.SetStrobeMode(value)

	case address == register.JOYPAD2:
		m.bus.Controller2.SetStrobeMode(value)

	case address <= register.APU_FRAME:
		m.bus.APU.Write(address, value)

	case address >= 0x4020: // mappers like GTROM allow writes starting 0x5000
		m.bus.Mapper.Write(address, value)

	default:
		panic(fmt.Sprintf("unhandled memory write at address: 0x%04X", address))
	}
}

// Read a byte from a memory address.
func (m *Memory) Read(address uint16) byte {
	switch {
	case address < register.PPU_CTRL:
		return m.ram.Read(address & 0x07FF)

	case address < register.APU_PL1_VOL:
		return m.bus.PPU.Read(address)

	case address == controller.JOYPAD1:
		return m.bus.Controller1.Read()

	case address == controller.JOYPAD2:
		return m.bus.Controller2.Read()

	case address <= register.APU_FRAME:
		return m.bus.APU.Read(address)

	case address >= 0x4020: // GTROM allow writes starting 0x5000, MMC1 has RAM starting at 0x6000
		return m.bus.Mapper.Read(address)

	default:
		panic(fmt.Sprintf("unhandled memory read at address: 0x%04X", address))
	}
}
