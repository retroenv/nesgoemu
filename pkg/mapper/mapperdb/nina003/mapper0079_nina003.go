// Package nina003 implements mapper 79 NINA-003 and NINA-006 boards.
// The boards use 32 or 64 KiB PRG ROM, 8-64 KiB CHR ROM, and fixed mirroring.
// They have no cartridge RAM. NES 2.0 submapper 0 is supported.
// Register decoding, latch bits, and bus behavior follow:
// https://www.nesdev.org/wiki/NINA-003-006
package nina003

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

const bankRegisterBase = 0x4100

// New creates a NINA-003/006 board with ROM and fixed mirroring.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	if err := validateCartridge(base.Cartridge()); err != nil {
		return nil, err
	}

	base.SetPrgRAM(nil)
	base.SetChrRAM(nil)
	m := &mapperNINA003{Base: base}
	m.SetName("NINA-003/006")
	m.DeclareFeature(feature.PRGBanking)
	m.DeclareFeature(feature.CHRBanking)
	m.SetPrgWindowSize(0x8000)
	m.SetChrWindowSize(0x2000)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}
	m.AddWriteHook(bankRegisterBase, 0x5fff, m.selectBanks)

	return m, nil
}

type mapperNINA003 struct{ *mapperbase.Base }

func (m *mapperNINA003) selectBanks(address uint16, value byte) error {
	if address&0xe100 != bankRegisterBase {
		return nil
	}

	m.SetPrgWindow(0, int(value>>3&1))
	m.SetChrWindow(0, int(value&7))
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.CHRBanking)

	return nil
}

func validateCartridge(cart *cartridge.Cartridge) error {
	if err := mapperbase.ValidateROMOnly(cart, "nina-003"); err != nil {
		return fmt.Errorf("checking ROM-only metadata: %w", err)
	}
	if err := mapperbase.ValidatePowerOfTwoSize("nina-003 PRG ROM", len(cart.PRG), 0x8000, 0x10000); err != nil {
		return fmt.Errorf("checking ROM size: %w", err)
	}

	if err := mapperbase.ValidatePowerOfTwoSize("nina-003 CHR ROM", len(cart.CHR), 0x2000, 0x10000); err != nil {
		return fmt.Errorf("checking CHR size: %w", err)
	}

	return nil
}
