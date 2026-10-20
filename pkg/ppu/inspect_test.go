package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestSnapshotReportsRegisters(t *testing.T) {
	sys := &bus.Bus{Cartridge: cartridge.New()}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)

	p.Write(register.PPU_CTRL, 0x90)
	p.Write(register.PPU_MASK, 0x1e)
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
	sys := &bus.Bus{Cartridge: cartridge.New()}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)
	p.palette.Write(0x3f00, 0x0f)

	value, ok := p.PeekVRAM(0x3f00)

	assert.True(t, ok)
	assert.Equal(t, byte(0x0f), value)
}

func TestPeekVRAMPatternTableWithoutInspector(t *testing.T) {
	sys := &bus.Bus{Cartridge: cartridge.New()}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)

	_, ok := p.PeekVRAM(0x0000)

	assert.False(t, ok)
}
