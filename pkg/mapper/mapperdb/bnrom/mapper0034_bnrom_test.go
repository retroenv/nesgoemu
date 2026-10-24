package bnrom_test

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestBNROMBanksAndConflicts(t *testing.T) {
	cart := testCartridge(false)
	cart.PRG[0x1234] = 1
	cart.PRG[0x8000+0x1234] = 2
	cart.PRG[0x10000+0x1234] = 0
	m := testMapper(t, cart)
	for _, bank := range []byte{1, 2, 0} {
		m.Write(0x9234, 3)
		assert.Equal(t, bank, m.Read(0x8001))
	}

	offsets, ok := m.(bus.PRGROMMapper)
	assert.True(t, ok)
	for value := range 256 {
		m.Write(0xffff, byte(value))
		bank := value % 16
		for _, address := range []uint16{0x8000, 0x9fff, 0xa000, 0xbfff, 0xc000, 0xffff} {
			physical, mapped := offsets.PRGROMOffset(address)
			assert.True(t, mapped)
			assert.Equal(t, bank*0x8000+int(address-0x8000), physical)
			assert.Equal(t, cart.PRG[physical], m.Read(address))
		}
	}

	m.Write(0x7ffd, 0)
	assert.Equal(t, byte(15), m.Read(0x8001))
	assert.Equal(t, byte(0x5a), m.Read(0x7ffd))
	m.Write(0x6000, 0x55)
	assert.Equal(t, byte(0x5a), m.Read(0x6000))
	m.Write(0, 0x33)
	m.Write(0x1fff, 0x44)
	assert.Equal(t, byte(0x33), m.Read(0))
	assert.Equal(t, byte(0x44), m.Read(0x1fff))
	assert.Equal(t, cart.Mirror, m.MirrorMode())
}

func TestNINABanksAndOverlappingRAM(t *testing.T) {
	cart := testCartridge(true)
	m := testMapper(t, cart)
	m.Write(0x7ffd, 0x81)
	assert.Equal(t, byte(1), m.Read(0x8001))
	assert.Equal(t, byte(0x81), m.Read(0x7ffd))
	m.Write(0x7ffe, 0xfe)
	m.Write(0x7fff, 0xfd)
	assert.Equal(t, byte(14), m.Read(0))
	assert.Equal(t, byte(14), m.Read(0x0fff))
	assert.Equal(t, byte(13), m.Read(0x1000))
	assert.Equal(t, byte(13), m.Read(0x1fff))
	assert.Equal(t, byte(0xfe), m.Read(0x7ffe))
	assert.Equal(t, byte(0xfd), m.Read(0x7fff))
	m.Write(0x7ffe, 2)
	assert.Equal(t, byte(2), m.Read(0))
	assert.Equal(t, byte(13), m.Read(0x1000))
	m.Write(0x6000, 0x55)
	assert.Equal(t, byte(0x55), m.Read(0x6000))
	m.Write(0x8000, 0)
	assert.Equal(t, byte(1), m.Read(0x8001))
	m.Write(0, 0x44)
	assert.Equal(t, byte(2), m.Read(0))
	assert.Equal(t, cart.Mirror, m.MirrorMode())
}

func TestSubmapperOverridesCHRHeuristic(t *testing.T) {
	cart := testCartridge(true)
	cart.CHR = cart.CHR[:0x2000]
	cart.NES2 = &cartridge.NES2Metadata{
		Submapper: 1,
		RAMSizes: cartridge.RAMSizes{
			PRGVolatile: 0x2000,
		},
	}
	m := testMapper(t, cart)
	m.Write(0x7ffd, 1)
	assert.Equal(t, byte(1), m.Read(0x8001))
	assert.Equal(t, byte(1), m.Read(0x7ffd))
}

func TestUnsupportedCartridges(t *testing.T) {
	cases := map[string]func(*cartridge.Cartridge){
		"missing PRG": func(c *cartridge.Cartridge) { c.PRG = nil },
		"partial PRG": func(c *cartridge.Cartridge) { c.PRG = c.PRG[:0x9000] },
		"submapper": func(c *cartridge.Cartridge) {
			c.NES2 = &cartridge.NES2Metadata{
				Submapper: 3,
			}
		},
		"four screen": func(c *cartridge.Cartridge) { c.Mirror = cartridge.Mirror4 },
		"BNROM PRG RAM": func(c *cartridge.Cartridge) {
			c.CHR = nil
			c.NES2 = &cartridge.NES2Metadata{
				Submapper: 2,
				RAMSizes: cartridge.RAMSizes{
					PRGVolatile: 0x2000,
					CHRVolatile: 0x2000,
				},
			}
		},
		"BNROM large CHR": func(c *cartridge.Cartridge) {
			c.NES2 = &cartridge.NES2Metadata{
				Submapper: 2,
			}
		},
		"NINA missing RAM": func(c *cartridge.Cartridge) {
			c.NES2 = &cartridge.NES2Metadata{
				Submapper: 1,
			}
		},
		"NINA hybrid CHR": func(c *cartridge.Cartridge) {
			c.NES2 = &cartridge.NES2Metadata{
				Submapper: 1,
				RAMSizes: cartridge.RAMSizes{
					PRGVolatile: 0x2000,
					CHRVolatile: 0x2000,
				},
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			cart := testCartridge(true)
			change(cart)
			m, err := mapper.New(&bus.Bus{
				Cartridge: cart,
				NameTable: nametable.New(cart.Mirror),
			})
			assert.Error(t, err)
			assert.Nil(t, m)
		})
	}
}

func testCartridge(nina bool) *cartridge.Cartridge {
	cart := &cartridge.Cartridge{
		Mapper: 34,
		Mirror: cartridge.MirrorVertical,
		PRG:    make([]byte, 0x80000),
	}
	for i := range cart.PRG {
		cart.PRG[i] = 0xff
	}

	if nina {
		cart.PRG = cart.PRG[:0x10000]
		cart.CHR = make([]byte, 0x10000)
	}

	for bank := range len(cart.PRG) / 0x8000 {
		cart.PRG[bank*0x8000+1] = byte(bank)
	}

	for i := range cart.CHR {
		cart.CHR[i] = byte(i / 0x1000)
	}

	return cart
}

func testMapper(t *testing.T, cart *cartridge.Cartridge) bus.Mapper {
	t.Helper()
	m, err := mapper.New(&bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
		OpenBus:   testOpenBus{},
	})
	assert.NoError(t, err)

	return m
}

type testOpenBus struct{}

func (testOpenBus) OpenBus() byte { return 0x5a }
