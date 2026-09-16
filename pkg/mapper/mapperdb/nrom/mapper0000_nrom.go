// Package nrom implements NROM cartridge boards.
package nrom

/*
Boards: NROM, HROM*, RROM, RTROM, SROM, STROM
PRG ROM capacity: 16K or 32K
CHR capacity: 8K
*/

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
)

// New returns a new NROM mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	m := &mapperNROM{
		Base: base,
	}
	m.SetName("NROM")
	m.Initialize()
	return m, nil
}

type mapperNROM struct {
	*mapperbase.Base
}
