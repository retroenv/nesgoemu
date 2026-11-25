package mapper

import (
	"fmt"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNewRejectsInvalidPRGBanks(t *testing.T) {
	// Invalid PRG sizes must return an error before bank setup or memory access.
	for _, id := range []uint16{testMapperNROM, testMapperMMC1, testMapperUxROM, testMapperCNROM, testMapperAxROM, testMapperUNROM512, testMapperUN1ROM, testMapperGTROM, testMapperUxROMAND} {
		for _, size := range []int{0, 0x2000, 0x8001} {
			t.Run(fmt.Sprintf("mapper_%d_size_%d", id, size), func(t *testing.T) {
				cart := &cartridge.Cartridge{
					Mapper: id,
					PRG:    make([]byte, size),
				}
				m, err := New(mappertest.NewBus(cart))
				assert.Error(t, err)
				assert.Nil(t, m)
			})
		}
	}
}

func TestNewRejectsGTROMWithCHRROM(t *testing.T) {
	// GTROM used to accept CHR ROM and modify it through the CHR RAM write path.
	cart := &cartridge.Cartridge{
		Mapper: testMapperGTROM,
		PRG:    make([]byte, testDefaultPRGROMSize),
		CHR:    make([]byte, testDefaultCHRROMSize),
	}
	m, err := New(mappertest.NewBus(cart))
	assert.Error(t, err)
	assert.Nil(t, m)
}

func TestNewRejectsInvalidRAMMetadata(t *testing.T) {
	for _, id := range []uint16{testMapperNROM, testMapperMMC3, testMapperRainbow, testMapperMXM} {
		cart := &cartridge.Cartridge{
			Mapper: id,
			PRG:    make([]byte, testDefaultPRGROMSize),
			NES2: &cartridge.NES2Metadata{
				RAMSizes: cartridge.RAMSizes{
					PRGVolatile: -1,
				},
			},
		}
		m, err := New(mappertest.NewBus(cart))
		assert.Error(t, err)
		assert.Nil(t, m)
	}
}
