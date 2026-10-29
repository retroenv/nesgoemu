package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const openBusValue = 0x5A

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

func TestLegacyCartridgeWithoutCHRUsesRAM(t *testing.T) {
	m := newTestMapper(t, &cartridge.Cartridge{PRG: make([]byte, 0x8000)})
	m.Write(0, 0x23)
	m.Write(0x1fff, 0x45)
	assert.Equal(t, byte(0x23), m.Read(0))
	assert.Equal(t, byte(0x45), m.Read(0x1fff))
}

func TestNES2CartridgeWithoutCHRUsesHeaderRAMSize(t *testing.T) {
	t.Parallel()

	// MMC1 selects CHR banks at power-on, which requires CHR memory.
	m := newTestMapper(t, &cartridge.Cartridge{
		PRG:    make([]byte, 0x20000),
		Mapper: 1,
		NES2: &cartridge.NES2Metadata{
			RAMSizes: cartridge.RAMSizes{CHRVolatile: 0x2000},
		},
	})
	m.Write(0x8000, 0x80) // reset the shift register, which applies the control register
	m.Write(0, 0x23)
	m.Write(0x1fff, 0x45)
	assert.Equal(t, byte(0x23), m.Read(0))
	assert.Equal(t, byte(0x45), m.Read(0x1fff))
}

func TestNES2CartridgeWithoutCHRMemory(t *testing.T) {
	t.Parallel()

	m := newTestMapper(t, &cartridge.Cartridge{
		PRG:    make([]byte, 0x20000),
		Mapper: 1,
		NES2:   &cartridge.NES2Metadata{},
	})
	m.Write(0x8000, 0x80) // must not fail without CHR banks
}

func TestNewWithoutPrgRAMReturnsOpenBus(t *testing.T) {
	t.Parallel()

	m := newTestMapper(t, &cartridge.Cartridge{
		PRG:  make([]byte, 0x4000),
		CHR:  make([]byte, 0x2000),
		NES2: &cartridge.NES2Metadata{},
	})

	m.Write(0x6000, 0x11) // ignored, no memory is present

	assert.Equal(t, byte(openBusValue), m.Read(0x6000))
	assert.Equal(t, byte(openBusValue), m.Read(0x7FFF))
}

func newTestMapper(t *testing.T, cart *cartridge.Cartridge) bus.Mapper {
	t.Helper()

	system := &bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
		OpenBus:   openBusTestValue(openBusValue),
	}
	m, err := New(system)
	assert.NoError(t, err)

	return m
}

type openBusTestValue byte

func (v openBusTestValue) OpenBus() byte {
	return byte(v)
}

func mappingTestCartridge(id uint16, chrSize int) *cartridge.Cartridge {
	prgSize := 0x10000
	if id == 0 {
		prgSize = 0x8000
		chrSize = 0x2000
	}
	if id == 30 || id == 111 {
		chrSize = 0
	}

	return &cartridge.Cartridge{
		Mapper: id,
		PRG:    make([]byte, prgSize),
		CHR:    make([]byte, chrSize),
	}
}
