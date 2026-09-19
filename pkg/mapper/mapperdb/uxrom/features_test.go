package uxrom

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestFeaturesPrgBankingMarkedOnWrite(t *testing.T) {
	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			CHR: make([]byte, 0x2000),
			PRG: make([]byte, 0xC000),
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	m, err := NewOR(base)
	assert.NoError(t, err)

	user, ok := m.(feature.User)
	assert.True(t, ok, "mapper must report its features")

	usage := user.Features()
	assert.Equal(t, []feature.Usage{
		{
			ID:   feature.PRGBanking,
			Name: "PRG banking",
		},
	}, usage)

	m.Write(0x8000, 1) // select bank 1

	usage = user.Features()
	assert.Equal(t, []feature.Usage{
		{
			ID:   feature.PRGBanking,
			Name: "PRG banking",
			Used: true,
		},
	}, usage)
}
