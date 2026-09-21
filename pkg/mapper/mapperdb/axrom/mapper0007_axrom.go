// Package axrom implements AxROM cartridge boards.
package axrom

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

/*
Boards: AMROM, ANROM, AN1ROM, AOROM, others
PRG ROM capacity: 256K
PRG ROM window: 32K
CHR capacity: 8K
*/

// New returns a new AxROM mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	m := &mapperAxROM{
		Base: base,
	}
	m.SetName("AxROM")
	m.DeclareFeature(feature.PRGBanking)
	m.DeclareFeature(feature.SingleScreenMirroring)
	m.SetPrgWindowSize(0x8000) // 32K
	m.Initialize()

	translation := mapperbase.MirrorModeTranslation{
		0: cartridge.MirrorSingle0,
		1: cartridge.MirrorSingle1,
	}
	m.SetMirrorModeTranslation(translation)

	m.AddWriteHook(0x8000, 0xFFFF, m.setPrgWindow)
	return m, nil
}

type mapperAxROM struct {
	*mapperbase.Base
}

func (m *mapperAxROM) setPrgWindow(_ uint16, value uint8) error {
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.SingleScreenMirroring)

	value &= 0b0000_0111
	m.SetPrgWindow(0, int(value)) // select 32 KB PRG ROM bank for CPU $8000-$FFFF

	mirrorMode := (value >> 4) & 1
	if err := m.SetNameTableMirrorModeIndex(mirrorMode); err != nil {
		return fmt.Errorf("setting AxROM mirror mode: %w", err)
	}

	return nil
}
