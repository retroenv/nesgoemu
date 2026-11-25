package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	testMapperNROM     = 0
	testMapperMMC1     = 1
	testMapperUxROM    = 2
	testMapperCNROM    = 3
	testMapperMMC3     = 4
	testMapperAxROM    = 7
	testMapperUNROM512 = 30
	testMapperBNROM    = 34
	testMapperCamerica = 71
	testMapperUN1ROM   = 94
	testMapperGTROM    = 111
	testMapperUxROMAND = 180
	testMapperRainbow  = 682
	testMapperMXM      = 800

	testDefaultPRGROMSize  = 0x8000
	testMirroredPRGROMSize = 0x4000
	testBankedPRGROMSize   = 0x10000
	testMMC1PRGROMSize     = 0x20000
	testDefaultCHRROMSize  = 0x2000
	testCHRRAMSize         = 0x2000

	testPRGRAMAddress = 0x6000
	testPRGRAMValue   = 0x5a

	testCHRFirstValue = 0x23
	testCHRLastValue  = 0x45

	testMMC1ResetAddress = 0x8000
	testMMC1ResetValue   = 0x80
)

func TestNewProvidesPrgRAMForLegacyCartridge(t *testing.T) {
	t.Parallel()

	m := newTestMapper(t, &cartridge.Cartridge{
		PRG: make([]byte, testDefaultPRGROMSize),
		CHR: make([]byte, testDefaultCHRROMSize),
		RAM: 1,
	})

	m.Write(testPRGRAMAddress, testPRGRAMValue)

	assert.Equal(t, byte(testPRGRAMValue), m.Read(testPRGRAMAddress))
}

func TestNewProvidesDefaultPrgRAMForLegacyCartridgeWithoutSize(t *testing.T) {
	t.Parallel()

	m := newTestMapper(t, &cartridge.Cartridge{
		PRG: make([]byte, testMirroredPRGROMSize),
		CHR: make([]byte, testDefaultCHRROMSize),
	})

	m.Write(testPRGRAMAddress, testPRGRAMValue)

	assert.Equal(t, byte(testPRGRAMValue), m.Read(testPRGRAMAddress))
}

func TestLegacyCartridgeWithoutCHRUsesRAM(t *testing.T) {
	m := newTestMapper(t, &cartridge.Cartridge{PRG: make([]byte, testDefaultPRGROMSize)})
	m.Write(0, testCHRFirstValue)
	m.Write(0x1fff, testCHRLastValue)
	assert.Equal(t, byte(testCHRFirstValue), m.Read(0))
	assert.Equal(t, byte(testCHRLastValue), m.Read(0x1fff))
}

func TestNES2CartridgeWithoutCHRUsesHeaderRAMSize(t *testing.T) {
	t.Parallel()

	// MMC1 selects CHR banks at power-on, which requires CHR memory.
	m := newTestMapper(t, &cartridge.Cartridge{
		PRG:    make([]byte, testMMC1PRGROMSize),
		Mapper: testMapperMMC1,
		NES2: &cartridge.NES2Metadata{
			RAMSizes: cartridge.RAMSizes{CHRVolatile: testCHRRAMSize},
		},
	})
	m.Write(testMMC1ResetAddress, testMMC1ResetValue) // reset the shift register, which applies the control register
	m.Write(0, testCHRFirstValue)
	m.Write(0x1fff, testCHRLastValue)
	assert.Equal(t, byte(testCHRFirstValue), m.Read(0))
	assert.Equal(t, byte(testCHRLastValue), m.Read(0x1fff))
}

func TestNES2CartridgeWithoutCHRMemory(t *testing.T) {
	t.Parallel()

	m := newTestMapper(t, &cartridge.Cartridge{
		PRG:    make([]byte, testMMC1PRGROMSize),
		Mapper: testMapperMMC1,
		NES2:   &cartridge.NES2Metadata{},
	})
	m.Write(testMMC1ResetAddress, testMMC1ResetValue) // must not fail without CHR banks
}

func TestNewWithoutPrgRAMReturnsOpenBus(t *testing.T) {
	t.Parallel()

	m := newTestMapper(t, &cartridge.Cartridge{
		PRG:  make([]byte, testMirroredPRGROMSize),
		CHR:  make([]byte, testDefaultCHRROMSize),
		NES2: &cartridge.NES2Metadata{},
	})

	m.Write(testPRGRAMAddress, 0x11) // ignored, no memory is present

	assert.Equal(t, mappertest.OpenBusValue, m.Read(testPRGRAMAddress))
	assert.Equal(t, mappertest.OpenBusValue, m.Read(0x7FFF))
}

func newTestMapper(t *testing.T, cart *cartridge.Cartridge) bus.Mapper {
	t.Helper()

	system := mappertest.NewBus(cart)
	m, err := New(system)
	assert.NoError(t, err)

	return m
}

func mappingTestCartridge(id uint16, chrSize int) *cartridge.Cartridge {
	prgSize := testBankedPRGROMSize
	if id == testMapperNROM {
		prgSize = testDefaultPRGROMSize
		chrSize = testDefaultCHRROMSize
	}
	if id == testMapperUNROM512 || id == testMapperGTROM {
		chrSize = 0
	}

	return &cartridge.Cartridge{
		Mapper: id,
		PRG:    make([]byte, prgSize),
		CHR:    make([]byte, chrSize),
	}
}
