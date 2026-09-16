// Package uxrom implements UxROM cartridge boards and logic variants.
package uxrom

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
)

/*
Boards: UNROM, UOROM
PRG ROM capacity: 256K/4096K
PRG ROM window: 16K + 16K fixed
CHR capacity: 8K
*/

// NewOR returns a new mapper with the OR logic configuration.
func NewOR(base *mapperbase.Base) (bus.Mapper, error) {
	m := newMapperUxROM(base)
	m.SetName("UxROM")

	// $8000-$BFFF: 16 KB switchable PRG ROM bank
	// $C000-$FFFF: 16 KB PRG ROM bank, fixed to the last bank
	m.SetPrgWindow(1, -1) // $C000-$FFFF: 16 KB PRG ROM bank, fixed to the last bank
	return m, nil
}

// NewUN1 returns a new mapper with the UN1ROM configuration.
func NewUN1(base *mapperbase.Base) (bus.Mapper, error) {
	m := newMapperUxROM(base)
	m.SetName("UN1ROM")

	// $8000-$BFFF: 16 KB switchable PRG ROM bank
	// $C000-$FFFF: 16 KB PRG ROM bank, fixed to the last bank
	m.SetPrgWindow(1, -1) // $C000-$FFFF: 16 KB PRG ROM bank, fixed to the last bank
	m.valueShift = 2      // very similar to UxROM, but the register is shifted by two bits
	return m, nil
}

// NewAND returns a new mapper with the AND logic configuration.
func NewAND(base *mapperbase.Base) (bus.Mapper, error) {
	m := newMapperUxROM(base)
	m.SetName("UxROM")

	// $8000-$BFFF: 16 KB PRG ROM bank, fixed to the first bank
	// $C000-$FFFF: 16 KB switchable PRG ROM bank
	m.windowIndex = 1
	return m, nil
}

type mapperUxROM struct {
	*mapperbase.Base

	valueShift  int
	windowIndex int
}

func newMapperUxROM(base *mapperbase.Base) *mapperUxROM {
	m := &mapperUxROM{
		Base: base,
	}
	m.Initialize()

	m.AddWriteHook(0x8000, 0xFFFF, m.setPrgWindow)
	return m
}

func (m *mapperUxROM) setPrgWindow(_ uint16, value uint8) error {
	value >>= m.valueShift
	value &= 0b0000_0111                      // UNROM uses bits 2-0; UOROM/UN1ROM uses bits 3-0
	m.SetPrgWindow(m.windowIndex, int(value)) // select 16 KB PRG ROM bank
	return nil
}
