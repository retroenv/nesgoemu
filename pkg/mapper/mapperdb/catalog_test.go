package mapperdb

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestCatalogContainsSupportedMappers(t *testing.T) {
	mapperNumbers := []uint16{0, 1, 2, 3, 7, 30, 94, 111, 180, 682}

	for _, mapperNumber := range mapperNumbers {
		assert.NotNil(t, constructors[mapperNumber])
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
