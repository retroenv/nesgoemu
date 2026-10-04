package mapperdb

import (
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

func TestNewRejectsUnsupportedMapper(t *testing.T) {
	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{Mapper: 4095},
	})

	mapper, err := New(base)
	assert.Nil(t, mapper)
	assert.Error(t, err)
}
