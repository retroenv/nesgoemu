package mapperbase

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestPRGROMOffsetRejectsAbsentROM(t *testing.T) {
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	_, ok := base.PRGROMOffset(0x8000)
	assert.False(t, ok)
	base.Initialize()
	_, ok = base.PRGROMOffset(0xffff)
	assert.False(t, ok)
}

func TestPRGROMOffsetDoesNotCallReadHooks(t *testing.T) {
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{PRG: make([]byte, 0x8000)},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	base.Initialize()
	called := false
	hook := base.AddReadHook(0x8000, 0x9fff, func(uint16) (byte, error) {
		called = true
		return 0, nil
	})
	_, ok := base.PRGROMOffset(0x8000)
	assert.False(t, ok)
	physical, ok := base.PRGROMOffset(0xa000)
	assert.True(t, ok)
	assert.Equal(t, 0x2000, physical)

	hook.SetProxyOnly(true)
	physical, ok = base.PRGROMOffset(0x8001)
	assert.True(t, ok)
	assert.Equal(t, 1, physical)
	assert.False(t, called)
}
