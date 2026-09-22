package ppu

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/ppu/control"
	"github.com/retroenv/nesgoemu/pkg/ppu/mask"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
)

// Read from a PPU memory register address.
func (p *PPU) Read(address uint16) uint8 {
	base := mirroredRegisterAddressToBase(address)

	switch base {
	case register.PPU_CTRL, register.PPU_MASK, register.OAM_ADDR, register.PPU_SCROLL, register.PPU_ADDR:
		// Write-only ports return the decay register value and do not refresh it.
		// https://www.nesdev.org/wiki/Open_bus_behavior#PPU_open_bus
		return p.openBus.Value()

	case register.PPU_STATUS:
		return p.getStatus()

	case register.OAM_DATA:
		// A read of $2004 refreshes the decay register with the value from OAM.
		// https://www.nesdev.org/wiki/Open_bus_behavior#PPU_open_bus
		value := p.sprites.Read()
		p.openBus.Set(value)
		return value

	case register.PPU_DATA:
		return p.readData()

	default:
		panic(fmt.Sprintf("unhandled ppu read at address: 0x%04X", address))
	}
}

// Write to a PPU memory register address.
func (p *PPU) Write(address uint16, value uint8) {
	base := mirroredRegisterAddressToBase(address)

	if address != register.OAM_DMA {
		// A write to a PPU port refreshes the decay register with the value.
		// https://www.nesdev.org/wiki/Open_bus_behavior#PPU_open_bus
		p.openBus.Set(value)
	}

	switch base {
	case register.PPU_CTRL:
		p.control.Set(value)
		p.markControlFeatures(value)

	case register.PPU_MASK:
		p.mask.Set(value)
		p.markMaskFeatures(value)

	case register.PPU_STATUS:
		// The status port takes a write and refreshes the decay register, which
		// the write above does. The register itself is not changed.
		// https://www.nesdev.org/wiki/Open_bus_behavior#PPU_open_bus

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
		if p.bus.DMA != nil {
			p.bus.DMA.RequestOAM(value)
		} else {
			// The transfer writes each byte to $2004. The last byte refreshes the register.
			p.openBus.Set(p.sprites.WriteDMA(value))
		}
		p.features.Mark(feature.OAMDMA)

	default:
		panic(fmt.Sprintf("unhandled ppu write at address: 0x%04X", address))
	}
}

func (p *PPU) markControlFeatures(value uint8) {
	if value&control.CTRL_NMI != 0 {
		p.features.Mark(feature.NMI)
	}
	if value&control.CTRL_BG_1000 != 0 {
		p.features.Mark(feature.BackgroundTableHigh)
	}
	if value&control.CTRL_SPR_1000 != 0 {
		p.features.Mark(feature.SpriteTableHigh)
	}
	if value&control.CTRL_8x16 != 0 {
		p.features.Mark(feature.SpriteSize8x16)
	}
	if value&control.CTRL_INC_32 != 0 {
		p.features.Mark(feature.VRAMIncrement32)
	}
}

func (p *PPU) markMaskFeatures(value uint8) {
	if value&mask.MASK_BG != 0 {
		p.features.Mark(feature.BackgroundRendering)
	}
	if value&mask.MASK_SPR != 0 {
		p.features.Mark(feature.SpriteRendering)
	}
	if value&mask.MASK_MONO != 0 {
		p.features.Mark(feature.Grayscale)
	}
	if value&(mask.MASK_TINT_RED|mask.MASK_TINT_BLUE|mask.MASK_TINT_GREEN) != 0 {
		p.features.Mark(feature.ColorEmphasis)
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
