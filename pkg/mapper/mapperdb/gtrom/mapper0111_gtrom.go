// Package gtrom implements GTROM cartridge boards.
package gtrom

/*
Boards: GTROM
PRG ROM capacity: 512K
PRG ROM window: 32K
CHR capacity: 16K
CHR window: 8K

32K CHR RAM used as two 8K CHR RAM and two 8K nametables
*/

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
)

// New returns a new GTROM mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	m := &mapperGTROM{
		Base: base,
	}
	m.SetName("Cheapocabra (GTROM)")
	m.DeclareFeature(feature.CHRBanking)
	m.DeclareFeature(feature.NameTableBanking)
	m.DeclareFeature(feature.PRGBanking)
	m.SetPrgWindowSize(0x8000) // 32K
	m.SetNameTableCount(2)
	m.SetChrRAM(make([]byte, 0x4000)) // 16K
	m.Initialize()

	m.AddReadHook(0x5000, 0x5FFF, m.getControl)
	m.AddReadHook(0x7000, 0x7FFF, m.getControl)
	m.AddWriteHook(0x5000, 0x5FFF, m.setBanks)
	m.AddWriteHook(0x7000, 0x7FFF, m.setBanks)

	return m, nil
}

type mapperGTROM struct {
	*mapperbase.Base
}

func (m *mapperGTROM) getControl(_ uint16) (uint8, error) {
	return m.OpenBus(), nil
}

func (m *mapperGTROM) setBanks(_ uint16, value uint8) error {
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.CHRBanking)
	m.MarkFeature(feature.NameTableBanking)

	prgBank := value & 0b0000_1111

	m.SetPrgWindow(0, int(prgBank)) // select 32 KB PRG ROM bank for CPU $8000-$FFFF

	chrBank := int(value>>4) & 1
	m.SetChrWindow(0, chrBank)

	nameTableBank := int(value>>5) & 1
	m.SetNameTableWindow(nameTableBank)
	return nil
}
