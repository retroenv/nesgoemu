package mapper

import (
	"fmt"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNewRejectsInvalidPRGBanks(t *testing.T) {
	// Invalid PRG sizes must return an error before bank setup or memory access.
	for _, id := range []uint16{0, 1, 2, 3, 7, 30, 94, 111, 180} {
		for _, size := range []int{0, 0x2000, 0x8001} {
			t.Run(fmt.Sprintf("mapper_%d_size_%d", id, size), func(t *testing.T) {
				cart := &cartridge.Cartridge{
					Mapper: id,
					PRG:    make([]byte, size),
				}
				m, err := New(&bus.Bus{
					Cartridge: cart,
					NameTable: nametable.New(cart.Mirror),
				})
				assert.Error(t, err)
				assert.Nil(t, m)
			})
		}
	}
}

func TestNewRejectsGTROMWithCHRROM(t *testing.T) {
	// GTROM used to accept CHR ROM and modify it through the CHR RAM write path.
	cart := &cartridge.Cartridge{
		Mapper: 111,
		PRG:    make([]byte, 0x8000),
		CHR:    make([]byte, 0x2000),
	}
	m, err := New(&bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
	})
	assert.Error(t, err)
	assert.Nil(t, m)
}

func TestNewRejectsInvalidRAMMetadata(t *testing.T) {
	for _, id := range []uint16{0, 4, 682, 800} {
		cart := &cartridge.Cartridge{
			Mapper: id,
			PRG:    make([]byte, 0x8000),
			NES2: &cartridge.NES2Metadata{
				RAMSizes: cartridge.RAMSizes{
					PRGVolatile: -1,
				},
			},
		}
		m, err := New(&bus.Bus{
			Cartridge: cart,
			NameTable: nametable.New(cart.Mirror),
		})
		assert.Error(t, err)
		assert.Nil(t, m)
	}
}
