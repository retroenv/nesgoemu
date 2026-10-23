package mapperdb

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

// A mapper table cell starts with the mapper ID, optionally inside a link.
var readmeMapperCell = regexp.MustCompile(`<td>(?:<a [^>]*>)?(\d{3,4}) `)

func TestCatalogConstructors(t *testing.T) {
	for mapperNumber, constructor := range constructors {
		assert.NotNil(t, constructor, "mapper %d has no constructor", mapperNumber)
	}
}

// The supported mapper tables in README.md are the source of the mapper IDs.
func TestCatalogMatchesREADME(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "..", "README.md"))
	assert.NoError(t, err)

	matches := readmeMapperCell.FindAllSubmatch(readme, -1)
	documented := make([]uint16, 0, len(matches))
	for _, match := range matches {
		id, err := strconv.ParseUint(string(match[1]), 10, 16)
		assert.NoError(t, err)
		documented = append(documented, uint16(id))
	}
	assert.NotEmpty(t, documented)

	assert.Equal(t, slices.Sorted(slices.Values(documented)), slices.Sorted(maps.Keys(constructors)))
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
