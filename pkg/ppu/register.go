package ppu

import (
	"fmt"

	"github.com/retroenv/retrogolib/arch/nes/register"
)

// Read from a PPU memory register address.
func (p *PPU) Read(address uint16) uint8 {
	base := mirroredRegisterAddressToBase(address)

	switch base {
	case register.PPU_CTRL:
		return p.control.Value()

	case register.PPU_MASK:
		return p.mask.Value()

	case register.PPU_STATUS:
		return p.getStatus()

	case register.OAM_DATA:
		return p.sprites.Read()

	case register.PPU_DATA:
		return p.readData()

	default:
		panic(fmt.Sprintf("unhandled ppu read at address: 0x%04X", address))
	}
}

// Write to a PPU memory register address.
func (p *PPU) Write(address uint16, value uint8) {
	base := mirroredRegisterAddressToBase(address)

	switch base {
	case register.PPU_CTRL:
		p.control.Set(value)

	case register.PPU_MASK:
		p.mask.Set(value)

	case register.OAM_ADDR:
		p.sprites.SetAddress(value)

	case register.OAM_DATA:
		p.sprites.Write(value)

	case register.PPU_SCROLL:
		if !p.addressing.Latch() {
			p.fineX = uint16(value) & 0x07
		}
		p.addressing.SetScroll(value)

	case register.PPU_ADDR:
		p.addressing.SetAddress(value)

	case register.PPU_DATA:
		address := p.addressing.Address()
		p.memory.Write(address, value)
		p.addressing.Increment(p.control.VRAMIncrement)

	case register.OAM_DMA:
		p.sprites.WriteDMA(value)

	default:
		panic(fmt.Sprintf("unhandled ppu write at address: 0x%04X", address))
	}
}

// mirroredRegisterAddressToBase converts the mirrored addresses to the base address.
// PPU registers are mirrored in every 8 bytes from $2008 through $3FFF.
func mirroredRegisterAddressToBase(address uint16) uint16 {
	if address == register.OAM_DMA {
		return address
	}

	base := 0x2000 + address&0b0000_0111
	return base
}
