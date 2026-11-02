package gtrom_test

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestMapperGTROM(t *testing.T) {
	prg := make([]byte, 0x8000*2)
	system := mappertest.NewBus(&cartridge.Cartridge{
		Mapper: 111,
		PRG:    prg,
		Mirror: cartridge.Mirror4,
	})
	m, err := mapper.New(system)
	assert.NoError(t, err)

	prg[0x7010] = 0x03 // bank 0
	prg[0xF010] = 0x04 // bank 1
	assert.Equal(t, 0x03, m.Read(0xF010))
	m.Write(0x5000, 1) // select PRG bank 1
	assert.Equal(t, 0x04, m.Read(0xF010))

	// Use the constructor's RAM through the public memory interface.
	m.Write(0x1010, 0x03)
	m.Write(0x5000, 1<<4)
	m.Write(0x1010, 0x04)
	m.Write(0x5000, 0)
	assert.Equal(t, 0x03, m.Read(0x1010))
	m.Write(0x5000, 1<<4)
	assert.Equal(t, 0x04, m.Read(0x1010))

	system.NameTable.Write(0x2100, 0x05)
	m.Write(0x5000, 1<<5)
	system.NameTable.Write(0x2100, 0x06)
	m.Write(0x5000, 0)
	assert.Equal(t, 0x05, system.NameTable.Read(0x2100))
	m.Write(0x5000, 1<<5)
	assert.Equal(t, 0x06, system.NameTable.Read(0x2100))
}

func TestMapperGTROMControlReadReturnsOpenBus(t *testing.T) {
	t.Parallel()

	system := mappertest.NewBus(&cartridge.Cartridge{
		Mapper: 111,
		PRG:    make([]byte, 0x8000),
		Mirror: cartridge.Mirror4,
	})
	m, err := mapper.New(system)
	assert.NoError(t, err)
	assert.Equal(t, mappertest.OpenBusValue, m.Read(0x5000))
}
