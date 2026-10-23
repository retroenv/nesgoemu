// Package mapperdb provides the mapper implementation catalog.
package mapperdb

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/axrom"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/cnrom"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/gtrom"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/gxrom"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/mmc1"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/mmc3"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/mxm"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/nrom"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/rainbow"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/unrom512"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb/uxrom"
)

// New creates the mapper selected by the cartridge metadata. Each constructor
// checks the submappers that it supports.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	mapperNumber := base.Cartridge().Mapper
	newMapper, ok := constructors[mapperNumber]
	if !ok {
		if metadata := base.Cartridge().NES2; metadata != nil {
			return nil, fmt.Errorf("mapper %d is not supported (NES 2.0 submapper %d)", mapperNumber, metadata.Submapper)
		}
		return nil, fmt.Errorf("mapper %d is not supported", mapperNumber)
	}

	return newMapper(base)
}

type constructor func(base *mapperbase.Base) (bus.Mapper, error)

var constructors = map[uint16]constructor{
	0:   nrom.New,
	1:   mmc1.New,
	2:   uxrom.NewOR,
	3:   cnrom.New,
	4:   mmc3.New,
	7:   axrom.New,
	30:  unrom512.New,
	66:  gxrom.New,
	94:  uxrom.NewUN1,
	111: gtrom.New,
	180: uxrom.NewAND,
	682: rainbow.New,
	800: mxm.New,
}
