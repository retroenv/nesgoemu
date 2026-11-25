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

const (
	prgBankSize   = 0x4000
	prgROMMaximum = 0x8000
)

// New returns a new NROM mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	if err := mapperbase.ValidateSize("NROM PRG ROM", len(base.Cartridge().PRG), prgBankSize, prgROMMaximum, prgBankSize); err != nil {
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
