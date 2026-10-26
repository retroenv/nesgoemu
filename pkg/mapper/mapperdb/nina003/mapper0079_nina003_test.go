package nina003_test

import (
	"fmt"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	bankRegisterBase = 0x4100
	prgBankSize      = 0x8000
	chrBankSize      = 0x2000
	chrBankMarker    = 0x10
)

func TestInitialMapping(t *testing.T) {
	for _, mirror := range []cartridge.MirrorMode{cartridge.MirrorHorizontal, cartridge.MirrorVertical} {
		t.Run(fmt.Sprintf("mirroring_%d", mirror), func(t *testing.T) {
			cart := testCartridge()
			cart.Mirror = mirror
			m := testMapper(t, cart)
			assert.Equal(t, byte(0), m.Read(0x8000))
			assert.Equal(t, byte(0), m.Read(0xffff))
			assert.Equal(t, byte(chrBankMarker), m.Read(0))
			assert.Equal(t, byte(chrBankMarker), m.Read(0x1fff))
			assert.Equal(t, mirror, m.MirrorMode())
		})
	}
}

func TestFeatures(t *testing.T) {
	for _, header := range []string{"legacy", "NES2"} {
		t.Run(header, func(t *testing.T) {
			cart := testCartridge()
			if header == "NES2" {
				cart.NES2 = &cartridge.NES2Metadata{}
			}

			m := testMapper(t, cart)
			user, ok := m.(feature.User)
			assert.True(t, ok)
			want := []feature.Usage{
				{
					ID:   feature.PRGBanking,
					Name: "PRG banking",
				},
				{
					ID:   feature.CHRBanking,
					Name: "CHR banking",
				},
			}
			assert.Equal(t, want, user.Features())

			for _, address := range []uint16{0, 0x4000, 0x4200, 0x5eff, 0x6000, 0x8000, 0xffff} {
				m.Write(address, 0xff)
				m.Read(address)
				assert.Equal(t, want, user.Features(), "address %04X", address)
			}

			m.Write(bankRegisterBase, 0xff)
			want[0].Used = true
			want[1].Used = true
			assert.Equal(t, want, user.Features())
		})
	}
}

func TestRegisterAddressDecode(t *testing.T) {
	m := testMapper(t, testCartridge())
	for address := range 1 << 16 {
		m.Write(bankRegisterBase, 0)
		m.Write(uint16(address), 0xff)
		prg := byte(0)
		chr := byte(chrBankMarker)
		if address&0xe100 == bankRegisterBase {
			prg = 1
			chr = chrBankMarker + 7
		}

		assert.Equal(t, prg, m.Read(0x8001), "PRG address %04X", address)
		assert.Equal(t, chr, m.Read(0), "CHR address %04X", address)
	}
}

func TestLatchBitsAndBankBoundaries(t *testing.T) {
	for _, mirror := range []cartridge.MirrorMode{cartridge.MirrorHorizontal, cartridge.MirrorVertical} {
		cart := testCartridge()
		cart.Mirror = mirror
		cart.NES2 = &cartridge.NES2Metadata{}
		m := testMapper(t, cart)
		offsets, ok := m.(bus.PRGROMMapper)
		assert.True(t, ok)
		for value := range 256 {
			m.Write(0x5fff, byte(value))
			prg := value >> 3 & 1
			chr := byte(value&7) + chrBankMarker
			for _, address := range []uint16{0x8000, 0x9fff, 0xa000, 0xbfff, 0xc000, 0xdfff, 0xe000, 0xffff} {
				physical, mapped := offsets.PRGROMOffset(address)
				assert.True(t, mapped)
				assert.Equal(t, prg*prgBankSize+int(address-0x8000), physical)
				assert.Equal(t, byte(prg), m.Read(address))
			}

			assert.Equal(t, chr, m.Read(0))
			assert.Equal(t, chr, m.Read(0x1fff))
			assert.Equal(t, mirror, m.MirrorMode())
		}
	}
}

func TestROMOnlyAndUnconnectedBits(t *testing.T) {
	cart := testCartridge()
	cart.PRG = cart.PRG[:prgBankSize]
	cart.CHR = cart.CHR[:2*chrBankSize]
	m := testMapper(t, cart)
	m.Write(bankRegisterBase, 0xff)
	assert.Equal(t, byte(0), m.Read(0x8001))
	assert.Equal(t, byte(chrBankMarker+1), m.Read(0))
	for _, address := range []uint16{0x6000, 0x7fff} {
		m.Write(address, 0xff)
		assert.Equal(t, byte(0x5a), m.Read(address))
	}
	m.Write(0, 0xff)
	assert.Equal(t, byte(chrBankMarker+1), m.Read(0))
	m.Write(0x8000, 0)
	assert.Equal(t, byte(chrBankMarker+1), m.Read(0))
}

func TestSupportedROMSizesIgnoreUnconnectedBankBits(t *testing.T) {
	for _, prgSize := range []int{prgBankSize, 2 * prgBankSize} {
		for _, chrSize := range []int{chrBankSize, 2 * chrBankSize, 4 * chrBankSize, 8 * chrBankSize} {
			t.Run(fmt.Sprintf("PRG_%dKiB_CHR_%dKiB", prgSize/1024, chrSize/1024), func(t *testing.T) {
				cart := testCartridge()
				cart.PRG = cart.PRG[:prgSize]
				cart.CHR = cart.CHR[:chrSize]
				m := testMapper(t, cart)
				for value := range 16 {
					m.Write(bankRegisterBase, byte(value))
					prg := byte((value / 8) % (prgSize / prgBankSize))
					chr := byte((value%8)%(chrSize/chrBankSize)) + chrBankMarker
					assert.Equal(t, prg, m.Read(0x8000), "latch %02X", value)
					assert.Equal(t, prg, m.Read(0xffff), "latch %02X", value)
					assert.Equal(t, chr, m.Read(0), "latch %02X", value)
					assert.Equal(t, chr, m.Read(0x1fff), "latch %02X", value)
				}
			})
		}
	}
}

func TestUnsupportedMemorySizes(t *testing.T) {
	for _, size := range []int{0, 0x4000, 0xc000, 0x18000, 0x20000} {
		t.Run(fmt.Sprintf("PRG_%dKiB", size/1024), func(t *testing.T) {
			cart := testCartridge()
			cart.PRG = make([]byte, size)
			assertRejected(t, cart)
		})
	}

	for _, size := range []int{0, 0x1000, 0x3000, 0x18000, 0x20000} {
		t.Run(fmt.Sprintf("CHR_%dKiB", size/1024), func(t *testing.T) {
			cart := testCartridge()
			cart.CHR = make([]byte, size)
			assertRejected(t, cart)
		})
	}
}

func TestUnsupportedMirroring(t *testing.T) {
	for name, mirror := range map[string]cartridge.MirrorMode{
		"four_screen":     cartridge.Mirror4,
		"single_screen_0": cartridge.MirrorSingle0,
		"single_screen_1": cartridge.MirrorSingle1,
	} {
		t.Run(name, func(t *testing.T) {
			cart := testCartridge()
			cart.Mirror = mirror
			cart.NES2 = &cartridge.NES2Metadata{}
			assertRejected(t, cart)
		})
	}
}

func TestUnsupportedMetadata(t *testing.T) {
	t.Run("battery", func(t *testing.T) {
		cart := testCartridge()
		cart.Battery = 1
		assertRejected(t, cart)
	})

	t.Run("trainer", func(t *testing.T) {
		cart := testCartridge()
		cart.Trainer = make([]byte, 512)
		assertRejected(t, cart)
	})

	t.Run("submapper", func(t *testing.T) {
		cart := testCartridge()
		cart.NES2 = &cartridge.NES2Metadata{
			Submapper: 1,
		}
		assertRejected(t, cart)
	})
}

func TestUnsupportedRAM(t *testing.T) {
	for name, ram := range map[string]cartridge.RAMSizes{
		"PRG_volatile": {
			PRGVolatile: 0x2000,
		},
		"PRG_nonvolatile": {
			PRGNonvolatile: 0x2000,
		},
		"CHR_volatile": {
			CHRVolatile: 0x2000,
		},
		"CHR_nonvolatile": {
			CHRNonvolatile: 0x2000,
		},
	} {
		t.Run(name, func(t *testing.T) {
			cart := testCartridge()
			cart.NES2 = &cartridge.NES2Metadata{
				RAMSizes: ram,
			}
			assertRejected(t, cart)
		})
	}
}

func assertRejected(t *testing.T, cart *cartridge.Cartridge) {
	t.Helper()
	m, err := mapper.New(&bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
	})
	assert.Error(t, err)
	assert.Nil(t, m)
}

func testCartridge() *cartridge.Cartridge {
	cart := &cartridge.Cartridge{
		Mapper: 79,
		PRG:    make([]byte, 2*prgBankSize),
		CHR:    make([]byte, 8*chrBankSize),
		Mirror: cartridge.MirrorVertical,
	}

	for i := range cart.PRG {
		cart.PRG[i] = byte(i / prgBankSize)
	}

	for i := range cart.CHR {
		cart.CHR[i] = chrBankMarker + byte(i/chrBankSize)
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
