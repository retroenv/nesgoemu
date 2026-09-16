// Package mapper provides hardware mapper support.
// It maps CHR and PRG chips into the NES address space.
package mapper

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperdb"
)

// New creates a new mapper for the mapper defined by the cartridge.
func New(bus *bus.Bus) (bus.Mapper, error) {
	base := mapperbase.New(bus)
	mapper, err := mapperdb.New(base)
	if err != nil {
		return nil, fmt.Errorf("creating mapper: %w", err)
	}

	return mapper, nil
}
