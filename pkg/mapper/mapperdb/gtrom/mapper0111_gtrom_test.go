package gtrom_test

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	testMapperID       = 111
	testControlAddress = 0x5000

	testPRGBankSize     = 0x8000
	testPRGBankCount    = 2
	testPRGProbeOffset  = 0x7010
	testPRGProbeAddress = 0xf010
	testPRGBank0Value   = 0x03
	testPRGBank1Value   = 0x04

	testCHRBank1Select  = 0x10
	testCHRProbeAddress = 0x1010
	testCHRBank0Value   = 0x03
	testCHRBank1Value   = 0x04

	testNameTableBank1Select  = 0x20
	testNameTableProbeAddress = 0x2100
	testNameTableBank0Value   = 0x05
	testNameTableBank1Value   = 0x06
)

func TestMapperGTROM(t *testing.T) {
	prg := make([]byte, testPRGBankSize*testPRGBankCount)
	system := mappertest.NewBus(&cartridge.Cartridge{
		Mapper: testMapperID,
		PRG:    prg,
		Mirror: cartridge.Mirror4,
	})
	m, err := mapper.New(system)
	assert.NoError(t, err)

	prg[testPRGProbeOffset] = testPRGBank0Value
	prg[testPRGBankSize+testPRGProbeOffset] = testPRGBank1Value
	assert.Equal(t, testPRGBank0Value, m.Read(testPRGProbeAddress))
	m.Write(testControlAddress, 1)
	assert.Equal(t, testPRGBank1Value, m.Read(testPRGProbeAddress))

	// Use the constructor's RAM through the public memory interface.
	m.Write(testCHRProbeAddress, testCHRBank0Value)
	m.Write(testControlAddress, testCHRBank1Select)
	m.Write(testCHRProbeAddress, testCHRBank1Value)
	m.Write(testControlAddress, 0)
	assert.Equal(t, testCHRBank0Value, m.Read(testCHRProbeAddress))
	m.Write(testControlAddress, testCHRBank1Select)
	assert.Equal(t, testCHRBank1Value, m.Read(testCHRProbeAddress))

	system.NameTable.Write(testNameTableProbeAddress, testNameTableBank0Value)
	m.Write(testControlAddress, testNameTableBank1Select)
	system.NameTable.Write(testNameTableProbeAddress, testNameTableBank1Value)
	m.Write(testControlAddress, 0)
	assert.Equal(t, testNameTableBank0Value, system.NameTable.Read(testNameTableProbeAddress))
	m.Write(testControlAddress, testNameTableBank1Select)
	assert.Equal(t, testNameTableBank1Value, system.NameTable.Read(testNameTableProbeAddress))
}

func TestMapperGTROMControlReadReturnsOpenBus(t *testing.T) {
	t.Parallel()

	system := mappertest.NewBus(&cartridge.Cartridge{
		Mapper: testMapperID,
		PRG:    make([]byte, testPRGBankSize),
		Mirror: cartridge.Mirror4,
	})
	m, err := mapper.New(system)
	assert.NoError(t, err)
	assert.Equal(t, mappertest.OpenBusValue, m.Read(testControlAddress))
}
