package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	benchmarkCamericaPRGROMSize = 0x20000
	benchmarkNINAPRGROMSize     = 0x8000
	benchmarkNINACHRROMSize     = 0x4000
	benchmarkMMC3PRGROMSize     = 0x8000
	benchmarkMMC3PRGRAMSize     = 0x2000
	benchmarkMMC3CHRRAMSize     = 0x2000
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
			id:      testMapperCamerica,
			prgSize: benchmarkCamericaPRGROMSize,
		},
		{
			name:    "NINA-001",
			id:      testMapperBNROM,
			prgSize: benchmarkNINAPRGROMSize,
			chrSize: benchmarkNINACHRROMSize,
		},
		{
			name:    "MMC3",
			id:      testMapperMMC3,
			prgSize: benchmarkMMC3PRGROMSize,
		},
	} {
		b.Run(test.name, func(b *testing.B) {
			cart := &cartridge.Cartridge{
				Mapper: test.id,
				PRG:    make([]byte, test.prgSize),
				CHR:    make([]byte, test.chrSize),
				Mirror: cartridge.MirrorHorizontal,
			}
			if test.id == testMapperMMC3 {
				cart.NES2 = &cartridge.NES2Metadata{
					RAMSizes: cartridge.RAMSizes{
						PRGVolatile: benchmarkMMC3PRGRAMSize,
						CHRVolatile: benchmarkMMC3CHRRAMSize,
					},
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				m, err := New(mappertest.NewBus(cart))
				assert.NoError(b, err)
				assert.NotNil(b, m)
			}
		})
	}
}
