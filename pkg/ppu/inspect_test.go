package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/control"
	"github.com/retroenv/nesgoemu/pkg/ppu/mask"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestSnapshotReportsRegisters(t *testing.T) {
	sys := &bus.Bus{
		Cartridge: cartridge.New(),
	}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)

	p.Write(register.PPU_CTRL, control.CTRL_NMI|control.CTRL_BG_1000)
	p.Write(register.PPU_MASK, mask.MASK_BG|mask.MASK_SPR|mask.MASK_BG_CLIP|mask.MASK_SPR_CLIP)
	p.addressing.SetAddress(0x21)
	p.addressing.SetAddress(0x08)
	p.fineX = 5

	snapshot := p.Snapshot()

	assert.Equal(t, byte(0x90), snapshot.Control)
	assert.Equal(t, byte(0x1e), snapshot.Mask)
	assert.Equal(t, uint16(0x2108), snapshot.V)
	assert.Equal(t, uint16(0x2108), snapshot.T)
	assert.Equal(t, uint16(5), snapshot.FineX)
	assert.False(t, snapshot.WriteLatch)
}

func TestPeekVRAMPalette(t *testing.T) {
	sys := &bus.Bus{
		Cartridge: cartridge.New(),
	}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)
	p.palette.Write(register.PALETTE_START, 0x0f)

	value, ok := p.PeekVRAM(register.PALETTE_START)

	assert.True(t, ok)
	assert.Equal(t, byte(0x0f), value)
}

func TestPeekVRAMPatternTableWithoutInspector(t *testing.T) {
	sys := &bus.Bus{
		Cartridge: cartridge.New(),
	}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)

	_, ok := p.PeekVRAM(0x0000)

	assert.False(t, ok)
}
