package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
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
