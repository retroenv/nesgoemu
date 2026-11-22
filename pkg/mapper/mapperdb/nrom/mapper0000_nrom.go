// Package nrom implements NROM cartridge boards.
package nrom

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
)

/*
Boards: NROM, HROM*, RROM, RTROM, SROM, STROM
PRG ROM capacity: 16K or 32K
CHR capacity: 8K
*/

// New returns a new NROM mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	if err := mapperbase.ValidateSize("NROM PRG ROM", len(base.Cartridge().PRG), 0x4000, 0x8000, 0x4000); err != nil {
		return nil, fmt.Errorf("checking memory size: %w", err)
	}

	m := &mapperNROM{
		Base: base,
	}
	m.SetName("NROM")
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}

	return m, nil
}

type mapperNROM struct {
	*mapperbase.Base
}

// SnapshotFormat identifies the direct mapper state format.
func (*mapperNROM) SnapshotFormat() string { return "nrom-v1" }
