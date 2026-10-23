package gxrom_test

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestBankMasksAndBoundaries(t *testing.T) {
	for _, mirror := range []cartridge.MirrorMode{cartridge.MirrorHorizontal, cartridge.MirrorVertical} {
		cart := testCartridge()
		cart.Mirror = mirror
		m := testMapper(t, cart)
		offsets, ok := m.(bus.PRGROMMapper)
		assert.True(t, ok)
		for value := range 256 {
			m.Write(0x8000, byte(value))
			prg := value >> 4 & 3
			chr := value & 3
			assert.Equal(t, byte(prg), m.Read(0x8001))
			assert.Equal(t, byte(chr), m.Read(0))
			assert.Equal(t, byte(chr), m.Read(0x1fff))
			for _, address := range []uint16{0x8000, 0x9fff, 0xa000, 0xbfff, 0xc000, 0xdfff, 0xe000, 0xffff} {
				physical, mapped := offsets.PRGROMOffset(address)
				assert.True(t, mapped)
				assert.Equal(t, prg*0x8000+int(address-0x8000), physical)
				assert.Equal(t, cart.PRG[physical], m.Read(address))
			}
			assert.Equal(t, mirror, m.MirrorMode())
		}
	}
}

func TestBusConflictUsesPreviousBank(t *testing.T) {
	cart := testCartridge()
	cart.PRG[0x1234] = 0x21
	cart.PRG[2*0x8000+0x1234] = 0x12
	m := testMapper(t, cart)
	m.Write(0x9234, 0x33)
	assert.Equal(t, byte(2), m.Read(0x8001))
	assert.Equal(t, byte(1), m.Read(0))
	m.Write(0x9234, 0x33)
	assert.Equal(t, byte(1), m.Read(0x8001))
	assert.Equal(t, byte(2), m.Read(0))
	m.Write(0xffff, 0x33)
	assert.Equal(t, byte(3), m.Read(0x8001))
	assert.Equal(t, byte(3), m.Read(0))
}

func TestSmallerROMsIgnoreUnconnectedBankBits(t *testing.T) {
	cart := testCartridge()
	cart.PRG = cart.PRG[:0x10000]
	cart.CHR = cart.CHR[:0x4000]
	cart.NES2 = &cartridge.NES2Metadata{}
	m := testMapper(t, cart)
	m.Write(0xffff, 0x33)
	assert.Equal(t, byte(1), m.Read(0x8001))
	assert.Equal(t, byte(1), m.Read(0))
}

func TestNoRAMOrWritableCHR(t *testing.T) {
	m := testMapper(t, testCartridge())
	for _, address := range []uint16{0x6000, 0x7fff} {
		m.Write(address, 0x33)
		assert.Equal(t, byte(0x5a), m.Read(address))
	}
	m.Write(0, 0xff)
	assert.Equal(t, byte(0), m.Read(0))
	assert.Equal(t, byte(0), m.Read(0x8001))
	features, ok := m.(feature.User)
	assert.True(t, ok)
	for _, item := range features.Features() {
		assert.NotEqual(t, feature.PRGRAM, item.ID)
	}
}

func TestUnsupportedCartridges(t *testing.T) {
	cases := map[string]func(*cartridge.Cartridge){
		"no PRG":        func(c *cartridge.Cartridge) { c.PRG = nil },
		"partial PRG":   func(c *cartridge.Cartridge) { c.PRG = c.PRG[:0x9000] },
		"oversized PRG": func(c *cartridge.Cartridge) { c.PRG = make([]byte, 0x40000) },
		"no CHR ROM":    func(c *cartridge.Cartridge) { c.CHR = nil },
		"partial CHR":   func(c *cartridge.Cartridge) { c.CHR = c.CHR[:0x3000] },
		"oversized CHR": func(c *cartridge.Cartridge) { c.CHR = make([]byte, 0x10000) },
		"four screen":   func(c *cartridge.Cartridge) { c.Mirror = cartridge.Mirror4 },
		"battery":       func(c *cartridge.Cartridge) { c.Battery = 1 },
		"trainer":       func(c *cartridge.Cartridge) { c.Trainer = make([]byte, 512) },
		"submapper": func(c *cartridge.Cartridge) {
			c.NES2 = &cartridge.NES2Metadata{
				Submapper: 1,
			}
		},
		"PRG RAM": func(c *cartridge.Cartridge) {
			c.NES2 = &cartridge.NES2Metadata{
				RAMSizes: cartridge.RAMSizes{
					PRGVolatile: 0x2000,
				},
			}
		},
		"CHR RAM": func(c *cartridge.Cartridge) {
			c.NES2 = &cartridge.NES2Metadata{
				RAMSizes: cartridge.RAMSizes{
					CHRVolatile: 0x2000,
				},
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			cart := testCartridge()
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

func testCartridge() *cartridge.Cartridge {
	cart := &cartridge.Cartridge{
		Mapper: 66,
		Mirror: cartridge.MirrorVertical,
		PRG:    make([]byte, 0x20000),
		CHR:    make([]byte, 0x8000),
	}
	for i := range cart.PRG {
		cart.PRG[i] = 0xff
	}
	for bank := range 4 {
		cart.PRG[bank*0x8000+1] = byte(bank)
		for i := range 0x2000 {
			cart.CHR[bank*0x2000+i] = byte(bank)
		}
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
