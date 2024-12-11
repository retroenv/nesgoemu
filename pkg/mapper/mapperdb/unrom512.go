package mapperdb

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/nes/cartridge"
)

/*
Boards: UNROM-512-8, UNROM-512-16, UNROM-512-32, INL-D-RAM, UNROM-512-F
PRG ROM capacity: 256K/512K
PRG ROM window: 16K + 16K fixed
CHR capacity: 32K
CHR window: 8K
*/

type mapperUNROM512 struct {
	Base
}

// NewUNROM512 returns a new mapper instance.
func NewUNROM512(base Base) (bus.Mapper, error) {
	m := &mapperUNROM512{
		Base: base,
	}
	m.SetName("UNROM 512")
	m.SetChrRAM(make([]byte, 0x8000)) // 32K
	m.Initialize()

	m.AddWriteHook(0x8000, 0xFFFF, m.setBanks)

	translation := mapperbase.MirrorModeTranslation{
		0: cartridge.MirrorHorizontal,
		1: cartridge.MirrorVertical,
		2: cartridge.MirrorSingle0,
		3: cartridge.Mirror4,
	}
	m.SetMirrorModeTranslation(translation)

	cart := m.Cartridge()
	if err := m.SetNameTableMirrorModeIndex(uint8(cart.Mirror)); err != nil {
		return nil, err
	}

	m.SetPrgWindow(1, -1)
	return m, nil
}

func (m *mapperUNROM512) setBanks(_ uint16, value uint8) error {
	prgBank := value & 0b0001_1111

	m.SetPrgWindow(0, int(prgBank)) // select 16 KB PRG ROM bank at $8000

	chrBank := int(value>>5) & 0b0000_0011
	m.SetChrWindow(0, chrBank)

	screen := int(value>>7) & 1
	if screen == 0 {
		return m.SetNameTableMirrorMode(cartridge.MirrorSingle0)
	}
	return m.SetNameTableMirrorMode(cartridge.MirrorSingle1)
}
