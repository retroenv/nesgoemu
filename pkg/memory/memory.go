// Package memory provides Memory functionality.
package memory

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
)

// Memory represents the memory controller.
type Memory struct {
	bus *bus.Bus
	ram *RAM

	// openBus is the last value on the CPU data bus. A read from an address
	// with no active device repeats it.
	// https://www.nesdev.org/wiki/Open_bus_behavior#CPU_open_bus
	openBus byte
}

// New returns a new memory instance, embedded it has
// new instances for PPU and the Controllers.
func New(bus *bus.Bus) *Memory {
	return &Memory{
		bus: bus,
		ram: NewRAM(0, 0x2000),
	}
}

// Write a byte to a memory address. A write places the value on the CPU data bus.
// https://www.nesdev.org/wiki/Open_bus_behavior#CPU_open_bus
func (m *Memory) Write(address uint16, value byte) {
	m.openBus = value

	switch {
	case address < register.PPU_CTRL:
		m.ram.Write(address&nes.RAMEndAddress, value)

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

	case address <= nes.IORegisterEndAddress: // $4018 to $401F are not mapped
		// The APU and I/O test mode registers are disabled, writes are ignored.
		// https://www.nesdev.org/wiki/CPU_memory_map

	default:
		panic(fmt.Sprintf("unhandled memory write at address: 0x%04X", address))
	}
}

// Read a byte from a memory address. A read places the value on the CPU data bus.
// https://www.nesdev.org/wiki/Open_bus_behavior#CPU_open_bus
func (m *Memory) Read(address uint16) byte {
	m.openBus = m.read(address)

	return m.openBus
}

// OpenBus returns the value of the CPU data bus.
func (m *Memory) OpenBus() byte {
	return m.openBus
}

// InspectRAM reads internal CPU RAM without access to MMIO or mapper state.
func (m *Memory) InspectRAM(address uint16) (byte, bool) {
	if address >= register.PPU_CTRL {
		return 0, false
	}

	return m.ram.Read(address & nes.RAMEndAddress), true
}

// read returns the value of a memory address without updating the data bus.
func (m *Memory) read(address uint16) byte {
	switch {
	case address < register.PPU_CTRL:
		return m.ram.Read(address & nes.RAMEndAddress)

	case address < register.APU_PL1_VOL:
		return m.bus.PPU.Read(address)

	case address == controller.JOYPAD1:
		return m.readController(m.bus.Controller1)

	case address == controller.JOYPAD2:
		return m.readController(m.bus.Controller2)

	case address <= register.APU_FRAME:
		if address == 0x4011 {
			if reader, ok := m.bus.Mapper.(bus.PCMReader); ok {
				return reader.ReadPCM()
			}
		}
		return m.bus.APU.Read(address)

	case address >= 0x4020: // GTROM allow writes starting 0x5000, MMC1 has RAM starting at 0x6000
		return m.bus.Mapper.Read(address)

	case address <= nes.IORegisterEndAddress: // $4018 to $401F are not mapped
		return m.openBus

	default:
		panic(fmt.Sprintf("unhandled memory read at address: 0x%04X", address))
	}
}

// readController reads a controller and combines it with the open bus value.
// The controller port drives bits 4-0. Bits 7-5 repeat the previous value on the
// bus, which is usually 010 from the high byte $40 of an absolute address. Games
// by Mindscape rely on the value $41 for a pressed button.
// https://www.nesdev.org/wiki/Open_bus_behavior#CPU_open_bus
func (m *Memory) readController(controller bus.Controller) byte {
	return m.openBus&0xE0 | controller.Read()&0x1F
}
