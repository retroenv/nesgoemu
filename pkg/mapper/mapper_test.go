package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNewProvidesPrgRAMForLegacyCartridge(t *testing.T) {
	t.Parallel()

	m := newTestMapper(t, &cartridge.Cartridge{
		PRG: make([]byte, 0x8000),
		CHR: make([]byte, 0x2000),
		RAM: 1,
	})

	m.Write(0x6000, 0x5A)

	assert.Equal(t, byte(0x5A), m.Read(0x6000))
}

func TestNewProvidesDefaultPrgRAMForLegacyCartridgeWithoutSize(t *testing.T) {
	t.Parallel()

	m := newTestMapper(t, &cartridge.Cartridge{
		PRG: make([]byte, 0x4000),
		CHR: make([]byte, 0x2000),
	})

	m.Write(0x6000, 0x5A)

	assert.Equal(t, byte(0x5A), m.Read(0x6000))
}

func newTestMapper(t *testing.T, cart *cartridge.Cartridge) bus.Mapper {
	t.Helper()

	system := &bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
	}
	m, err := New(system)
	assert.NoError(t, err)

	return m
}
