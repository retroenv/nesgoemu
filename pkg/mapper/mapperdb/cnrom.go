package mapperdb

/*
Boards: CNROM "and similar"
PRG ROM capacity: 16K or 32K
CHR capacity: 32K (2M oversize version)
CHR window: 8K
*/

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
)

type mapperCNROM struct {
	Base
}

// NewCNROM returns a new mapper instance.
func NewCNROM(base Base) (bus.Mapper, error) {
	m := &mapperCNROM{
		Base: base,
	}
	m.SetName("CNROM")
	m.Initialize()

	m.AddWriteHook(0x8000, 0xFFFF, m.setChrWindow)
	return m, nil
}

func (m *mapperCNROM) setChrWindow(_ uint16, value uint8) error {
	m.SetChrWindow(0, int(value)) // select 8 KB CHR ROM bank for PPU $0000-$1FFF
	return nil
}
