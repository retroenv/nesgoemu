package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

// TestSetControl verifies that the control byte gets handled correctly.
func TestSetControl(t *testing.T) {
	t.Parallel()

	sys := &bus.Bus{
		Cartridge: cartridge.New(),
	}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)

	p.Write(register.PPU_CTRL, 0b1111_1111)

	assert.Equal(t, 0x2C00, p.control.BaseNameTable)
	assert.Equal(t, 32, p.control.VRAMIncrement)
	assert.Equal(t, 0x01, p.control.SpritePatternTable)
	assert.Equal(t, 0x01, p.control.BackgroundPatternTable)
	assert.Equal(t, 0x01, p.control.SpriteSize)
	assert.Equal(t, 0x01, p.control.MasterSlave)
	assert.True(t, p.nmi.Enabled())
}

// TestSetMask verifies that the mask byte gets handled correctly.
func TestSetMask(t *testing.T) {
	t.Parallel()

	sys := &bus.Bus{
		Cartridge: cartridge.New(),
	}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)

	p.Write(register.PPU_MASK, 0b1111_1111)

	assert.True(t, p.mask.Grayscale)
	assert.True(t, p.mask.RenderBackgroundLeft())
	assert.True(t, p.mask.RenderSpritesLeft())
	assert.True(t, p.mask.RenderBackground())
	assert.True(t, p.mask.RenderSprites())
	assert.True(t, p.mask.EnhanceRed)
	assert.True(t, p.mask.EnhanceGreen)
	assert.True(t, p.mask.EnhanceBlue)
}

func TestResetKeepsVRAMAndVBlank(t *testing.T) {
	sys := &bus.Bus{Cartridge: cartridge.New()}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)

	p.Write(register.PPU_CTRL, 0xFF)
	p.Write(register.PPU_MASK, 0xFF)

	p.addressing.SetAddress(0x2B)
	p.addressing.SetAddress(0x45)
	p.addressing.SetScroll(0xFF)
	p.nmi.SetOccurred(true)
	p.palette.Write(0, 0x2A)
	p.fineX = 3
	p.dataReadBuffer = 7

	p.Reset()

	assert.Equal(t, byte(0), p.control.Value())
	assert.Equal(t, byte(0), p.mask.Value())
	assert.False(t, p.nmi.Enabled())
	assert.True(t, p.nmi.Occurred())
	assert.False(t, p.addressing.Latch())
	assert.Equal(t, uint16(0x2B45), p.addressing.Address())
	assert.Equal(t, uint16(0), p.fineX)
	assert.Equal(t, byte(0), p.dataReadBuffer)
	assert.Equal(t, byte(0x2A), p.palette.Read(0))
}

func TestGrayscalePaletteRead(t *testing.T) {
	cart := cartridge.New()
	nameTable := nametable.New(cart.Mirror)
	nameTable.SetVRAM(make([]byte, nametable.VramSize))
	sys := &bus.Bus{
		Cartridge: cart,
		NameTable: nameTable,
	}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)

	for _, address := range []uint16{0x3F00, 0x3F04, 0x3F10, 0x3F20} {
		p.palette.Write(address, 0x2A)
		p.Write(register.PPU_MASK, 1)
		p.Write(register.PPU_ADDR, byte(address>>8))
		p.Write(register.PPU_ADDR, byte(address))
		assert.Equal(t, byte(0x20), p.Read(register.PPU_DATA))
		assert.Equal(t, byte(0x2A), p.palette.Read(address))

		p.Write(register.PPU_MASK, 0)
		p.Write(register.PPU_ADDR, byte(address>>8))
		p.Write(register.PPU_ADDR, byte(address))
		assert.Equal(t, byte(0x2A), p.Read(register.PPU_DATA))
	}
}
