package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func BenchmarkConstructionRAM(b *testing.B) {
	for _, test := range []struct {
		name    string
		id      uint16
		prgSize int
		chrSize int
	}{
		{
			name:    "Camerica",
			id:      71,
			prgSize: 0x20000,
		},
		{
			name:    "NINA-001",
			id:      34,
			prgSize: 0x8000,
			chrSize: 0x4000,
		},
		{
			name:    "MMC3",
			id:      4,
			prgSize: 0x8000,
		},
	} {
		b.Run(test.name, func(b *testing.B) {
			cart := &cartridge.Cartridge{
				Mapper: test.id,
				PRG:    make([]byte, test.prgSize),
				CHR:    make([]byte, test.chrSize),
				Mirror: cartridge.MirrorHorizontal,
			}
			if test.id == 4 {
				cart.NES2 = &cartridge.NES2Metadata{
					RAMSizes: cartridge.RAMSizes{
						PRGVolatile: 0x2000,
						CHRVolatile: 0x2000,
					},
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				m, err := New(&bus.Bus{
					Cartridge: cart,
					NameTable: nametable.New(cart.Mirror),
				})
				assert.NoError(b, err)
				assert.NotNil(b, m)
			}
		})
	}
}
