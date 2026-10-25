package camerica_test

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	fireHawkBoard = "Fire Hawk"
	standardBoard = "standard"
)

func TestCamericaPRGBanksAndWriteRanges(t *testing.T) {
	cart := testCartridge(0x40000)
	m := testMapper(t, cart)
	assert.Equal(t, byte(0), m.Read(0x8001))
	assert.Equal(t, byte(15), m.Read(0xc001))
	offsets, ok := m.(bus.PRGROMMapper)
	assert.True(t, ok)
	for _, register := range []uint16{0xc000, 0xdfff, 0xe000, 0xffff} {
		for value := range 256 {
			m.Write(register, byte(value))
			bank := value & 15
			assert.Equal(t, byte(bank), m.Read(0x8001))
			assert.Equal(t, byte(15), m.Read(0xc001))
			for _, address := range []uint16{0x8000, 0x9fff, 0xa000, 0xbfff, 0xc000, 0xffff} {
				physical, mapped := offsets.PRGROMOffset(address)
				assert.True(t, mapped)
				want := bank*0x4000 + int(address-0x8000)
				if address >= 0xc000 {
					want = 0x3c000 + int(address-0xc000)
				}

				assert.Equal(t, want, physical)
				assert.Equal(t, cart.PRG[want], m.Read(address))
			}
		}
	}

	m.Write(0xc000, 13)
	for _, address := range []uint16{0x6000, 0x7fff, 0x8000, 0x9000, 0x9fff, 0xa000, 0xbfff} {
		m.Write(address, 0)
		assert.Equal(t, byte(13), m.Read(0x8001))
	}

	assert.Equal(t, byte(0x5a), m.Read(0x6000))
	m.Write(0, 0x33)
	m.Write(0x1fff, 0x44)
	assert.Equal(t, byte(0x33), m.Read(0))
	assert.Equal(t, byte(0x44), m.Read(0x1fff))
}

func TestCamericaMirroringVariants(t *testing.T) {
	for _, kind := range []string{"legacy", standardBoard, fireHawkBoard} {
		t.Run(kind, func(t *testing.T) {
			cart := testCartridge(0x20000)
			if kind != "legacy" {
				cart.NES2 = testMetadata(0)
			}

			if kind == fireHawkBoard {
				cart.NES2.Submapper = 1
			}

			m := testMapper(t, cart)
			m.Write(0x8000, 0x10)
			mode := cartridge.MirrorVertical
			if kind == fireHawkBoard {
				mode = cartridge.MirrorSingle1
			}

			assert.Equal(t, mode, m.MirrorMode())
			m.Write(0x9000, 0x10)
			if kind != standardBoard {
				mode = cartridge.MirrorSingle1
			}

			assert.Equal(t, mode, m.MirrorMode())
			m.Write(0x9fff, 0xef)
			if kind != standardBoard {
				mode = cartridge.MirrorSingle0
			}

			assert.Equal(t, mode, m.MirrorMode())
			m.Write(0xc000, 0xff)
			assert.Equal(t, byte(7), m.Read(0x8001))
			assert.Equal(t, mode, m.MirrorMode())
		})
	}
}

func TestCamericaUnconnectedBankBits(t *testing.T) {
	for _, size := range []int{0x10000, 0x20000} {
		m := testMapper(t, testCartridge(size))
		for value := range 16 {
			m.Write(0xc000, byte(value))
			assert.Equal(t, byte(value%(size/0x4000)), m.Read(0x8001))
		}
	}
}

func TestCamericaRejectsUnsupportedBoards(t *testing.T) {
	for _, kind := range []string{
		"missing PRG", "partial PRG", "small PRG", "oversized PRG", "CHR ROM", "four screen", "battery", "trainer",
		"submapper", "Fire Hawk size", "PRG RAM", "missing CHR RAM", "large CHR RAM", "nonvolatile CHR RAM",
	} {
		t.Run(kind, func(t *testing.T) {
			cart := unsupportedCartridge(kind)
			m, err := mapper.New(&bus.Bus{
				Cartridge: cart,
				NameTable: nametable.New(cart.Mirror),
			})
			assert.Error(t, err)
			assert.Nil(t, m)
		})
	}
}

func testCartridge(size int) *cartridge.Cartridge {
	cart := &cartridge.Cartridge{
		Mapper: 71,
		Mirror: cartridge.MirrorVertical,
		PRG:    make([]byte, size),
	}
	for bank := range size / 0x4000 {
		cart.PRG[bank*0x4000+1] = byte(bank)
	}

	return cart
}

func testMetadata(submapper byte) *cartridge.NES2Metadata {
	return &cartridge.NES2Metadata{
		Submapper: submapper,
		RAMSizes: cartridge.RAMSizes{
			CHRVolatile: 0x2000,
		},
	}
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

func unsupportedCartridge(kind string) *cartridge.Cartridge {
	cart := testCartridge(0x40000)
	cart.NES2 = testMetadata(0)
	switch kind {
	case "missing PRG":
		cart.PRG = nil
	case "partial PRG":
		cart.PRG = cart.PRG[:0x9000]
	case "small PRG":
		cart.PRG = cart.PRG[:0x8000]
	case "oversized PRG":
		cart.PRG = make([]byte, 0x80000)
	case "CHR ROM":
		cart.CHR = make([]byte, 0x2000)
	case "four screen":
		cart.Mirror = cartridge.Mirror4
	case "battery":
		cart.Battery = 1
	case "trainer":
		cart.Trainer = make([]byte, 512)
	case "submapper":
		cart.NES2.Submapper = 2
	case "Fire Hawk size":
		cart.NES2.Submapper = 1
	case "PRG RAM":
		cart.NES2.RAMSizes.PRGVolatile = 0x2000
	case "missing CHR RAM":
		cart.NES2.RAMSizes.CHRVolatile = 0
	case "large CHR RAM":
		cart.NES2.RAMSizes.CHRVolatile = 0x4000

	case "nonvolatile CHR RAM":
		cart.NES2.RAMSizes.CHRVolatile = 0
		cart.NES2.RAMSizes.CHRNonvolatile = 0x2000
	}

	return cart
}

type testOpenBus struct{}

func (testOpenBus) OpenBus() byte { return 0x5a }
