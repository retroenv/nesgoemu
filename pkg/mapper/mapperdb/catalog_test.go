package mapperdb

import (
	"maps"
	"slices"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestCatalogConstructors(t *testing.T) {
	for mapperNumber, constructor := range constructors {
		assert.NotNil(t, constructor, "mapper %d has no constructor", mapperNumber)
	}
}

// The expected IDs are the supported mapper tables in README.md.
func TestCatalogMatchesSupportedMappers(t *testing.T) {
	expected := []uint16{0, 1, 2, 3, 4, 7, 30, 94, 111, 180, 682, 800}

	actual := slices.Sorted(maps.Keys(constructors))
	assert.Equal(t, expected, actual)
}

func TestNewRejectsUnsupportedMapper(t *testing.T) {
	for _, test := range []struct {
		name     string
		mapper   uint16
		metadata *cartridge.NES2Metadata
		message  string
	}{
		{
			name:    "legacy header",
			mapper:  4095,
			message: "mapper 4095 is not supported",
		},
		{
			name:     "NES2 submapper zero is explicit",
			mapper:   4095,
			metadata: &cartridge.NES2Metadata{},
			message:  "mapper 4095 is not supported (NES 2.0 submapper 0)",
		},
		{
			name:     "NES2 nonzero submapper",
			mapper:   220,
			metadata: &cartridge.NES2Metadata{Submapper: 1},
			message:  "mapper 220 is not supported (NES 2.0 submapper 1)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := mapperbase.New(&bus.Bus{
				Cartridge: &cartridge.Cartridge{
					Mapper: test.mapper,
					NES2:   test.metadata,
				},
			})
			mapper, err := New(base)
			assert.Nil(t, mapper)
			assert.Error(t, err)
			assert.Equal(t, test.message, err.Error())
		})
	}
}
