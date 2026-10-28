package camerica_test

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestQuattroRegisterRangesAndPhysicalOffsets(t *testing.T) {
	for _, size := range []int{0x10000, 0x20000, 0x40000} {
		cart := testCartridge(size)
		cart.Mapper = 232
		m := testMapper(t, cart)
		assert.Equal(t, byte(0), m.Read(0x8001))
		assert.Equal(t, byte(3), m.Read(0xc001))
		offsets, ok := m.(bus.PRGROMMapper)
		assert.True(t, ok)
		for _, outer := range []uint16{0x8000, 0x9fff, 0xbfff} {
			for _, inner := range []uint16{0xc000, 0xdfff, 0xffff} {
				for block := range 4 {
					m.Write(outer, byte(block<<3)|0xe7)
					for page := range 4 {
						m.Write(inner, byte(page)|0xfc)
						for _, address := range []uint16{0x8000, 0x9fff, 0xbfff, 0xc000, 0xdfff, 0xffff} {
							bank := block*4 + page
							if address >= 0xc000 {
								bank = block*4 + 3
							}
							want := (bank*0x4000 + int(address&0x3fff)) % size
							physical, mapped := offsets.PRGROMOffset(address)
							assert.True(t, mapped)
							assert.Equal(t, want, physical)
							assert.Equal(t, cart.PRG[want], m.Read(address))
						}
					}
				}
			}
		}
	}
}

func TestQuattroAladdinBitsAndFixedMemory(t *testing.T) {
	cart := testCartridge(0x40000)
	cart.Mapper = 232
	cart.NES2 = testMetadata(1)
	m := testMapper(t, cart)
	for block, value := range []byte{0, 0x10, 8, 0x18} {
		m.Write(0x8000, value)
		m.Write(0xc000, 1)
		assert.Equal(t, byte(block*4+1), m.Read(0x8001))
		assert.Equal(t, byte(block*4+3), m.Read(0xc001))
	}

	m.Write(0x6000, 0)
	m.Write(0x7fff, 0)
	assert.Equal(t, byte(13), m.Read(0x8001))
	assert.Equal(t, byte(0x5a), m.Read(0x6000))
	assert.Equal(t, cartridge.MirrorVertical, m.MirrorMode())
	m.Write(0x1fff, 0x45)
	assert.Equal(t, byte(0x45), m.Read(0x1fff))
}

func TestQuattroRejectsUnsupportedBoards(t *testing.T) {
	for _, kind := range unsupportedCamericaBoards {
		t.Run(kind, func(t *testing.T) {
			cart := unsupportedCartridge(kind)
			cart.Mapper = 232
			m, err := mapper.New(&bus.Bus{
				Cartridge: cart,
				NameTable: nametable.New(cart.Mirror),
			})
			assert.Error(t, err)
			assert.Nil(t, m)
		})
	}
}
