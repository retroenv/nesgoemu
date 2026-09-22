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

	cycle uint64

	controllerAddress uint16
	controllerCycle   uint64
	controllerValue   byte
}

// New returns a new memory instance, embedded it has
// new instances for PPU and the Controllers.
func New(bus *bus.Bus) *Memory {
	return &Memory{
		bus: bus,
		ram: NewRAM(0, 0x2000),
	}
}

// BeginCycle advances the bus clock. Adjacent reads of the same controller
// keep its output enable active and must not shift a second button bit.
// https://www.nesdev.org/wiki/DMA#Register_conflicts
func (m *Memory) BeginCycle() {
	m.cycle++
}

// Write a byte to a memory address.
func (m *Memory) Write(address uint16, value byte) {
	switch {
	case address < register.PPU_CTRL:
		m.ram.Write(address&nes.RAMEndAddress, value)

	case address < register.APU_PL1_VOL:
		m.bus.PPU.Write(address, value)

	case address == register.OAM_DMA:
		m.bus.PPU.Write(address, value)

	case address == register.JOYPAD1:
		m.bus.Controller1.SetStrobeMode(value)
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
		return m.ram.Read(address & nes.RAMEndAddress)

	case address < register.APU_PL1_VOL:
		return m.bus.PPU.Read(address)

	case address == controller.JOYPAD1:
		return m.readController(address, m.bus.Controller1)

	case address == controller.JOYPAD2:
		return m.readController(address, m.bus.Controller2)

	case address <= register.APU_FRAME:
		if address == 0x4011 {
			if reader, ok := m.bus.Mapper.(bus.PCMReader); ok {
				return reader.ReadPCM()
			}
		}
		return m.bus.APU.Read(address)

	case address >= 0x4020: // GTROM allow writes starting 0x5000, MMC1 has RAM starting at 0x6000
		return m.bus.Mapper.Read(address)

	default:
		panic(fmt.Sprintf("unhandled memory read at address: 0x%04X", address))
	}
}

// InspectRAM reads internal CPU RAM without access to MMIO or mapper state.
func (m *Memory) InspectRAM(address uint16) (byte, bool) {
	if address >= register.PPU_CTRL {
		return 0, false
	}

	return m.ram.Read(address & nes.RAMEndAddress), true
}

func (m *Memory) readController(address uint16, device bus.Controller) byte {
	if m.cycle == 0 || m.controllerCycle+1 != m.cycle || m.controllerAddress != address {
		m.controllerValue = device.Read()
	}
	m.controllerCycle = m.cycle
	m.controllerAddress = address
	return m.controllerValue
}
