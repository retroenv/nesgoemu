package mapperbase

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestFeaturesReturnsDeclaredCatalog(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{}})
	base.DeclareFeature(feature.CHRBanking)
	base.DeclareFeature(feature.PRGBanking)

	base.MarkFeature(feature.PRGBanking)

	usage := base.Features()
	assert.Equal(t, []feature.Usage{
		{
			ID:   feature.CHRBanking,
			Name: "CHR banking",
		},
		{
			ID:   feature.PRGBanking,
			Name: "PRG banking",
			Used: true,
		},
	}, usage)
}

func TestMemorySizesReportsAllocatedRAM(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{}})
	base.SetPrgRAM(make([]byte, 0x2000))

	prgRAM, chrRAM := base.MemorySizes()
	assert.Equal(t, 0x2000, prgRAM)
	assert.Equal(t, 0, chrRAM)
}

func TestPrgRAMFeatureDeclaredAndMarkedOnAccess(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{}})
	assert.Empty(t, base.Features())

	base.SetPrgRAM(make([]byte, 0x2000))

	usage := base.Features()
	assert.Equal(t, []feature.Usage{
		{
			ID:   feature.PRGRAM,
			Name: "PRG RAM",
		},
	}, usage)

	base.Write(prgRAMStart, 0x5A)

	usage = base.Features()
	assert.Equal(t, []feature.Usage{
		{
			ID:   feature.PRGRAM,
			Name: "PRG RAM",
			Used: true,
		},
	}, usage)
}
