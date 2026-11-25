// Package axrom implements AxROM cartridge boards.
package axrom

import (
	"fmt"
	"math"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

/*
Boards: AMROM, ANROM, AN1ROM, AOROM, others
PRG ROM capacity: 256K
PRG ROM window: 32K
CHR capacity: 8K
*/

const (
	prgWindowSize = 0x8000

	prgBankMask        = 0x07
	mirroringBankShift = 4
	mirroringBankMask  = 0x01
)

// New returns a new AxROM mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	m := &mapperAxROM{
		Base: base,
	}
	m.SetName("AxROM")
	m.DeclareFeature(feature.PRGBanking)
	m.DeclareFeature(feature.SingleScreenMirroring)
	m.SetPrgWindowSize(prgWindowSize)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}

	translation := mapperbase.MirrorModeTranslation{
		0: cartridge.MirrorSingle0,
		1: cartridge.MirrorSingle1,
	}
	m.SetMirrorModeTranslation(translation)

	m.AddWriteHook(nes.CodeBaseAddress, math.MaxUint16, m.setPrgWindow)

	return m, nil
}

type mapperAxROM struct {
	*mapperbase.Base
}

// SnapshotFormat identifies the direct mapper state format.
func (*mapperAxROM) SnapshotFormat() string { return "axrom-mapperAxROM-v1" }

func (m *mapperAxROM) setPrgWindow(_ uint16, value uint8) error {
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.SingleScreenMirroring)

	// Register layout xxxM xPPP: bit 4 selects the 1 KB VRAM page for all four
	// nametables, bits 2-0 select the 32 KB PRG ROM bank.
	// https://www.nesdev.org/wiki/AxROM
	m.SetPrgWindow(0, int(value&prgBankMask))

	mirrorMode := (value >> mirroringBankShift) & mirroringBankMask
	if err := m.SetNameTableMirrorModeIndex(mirrorMode); err != nil {
		return fmt.Errorf("setting AxROM mirror mode: %w", err)
	}

	return nil
}
