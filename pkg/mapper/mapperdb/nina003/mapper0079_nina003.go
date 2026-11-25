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

const (
	bankRegisterBase = 0x4100
	bankRegisterEnd  = 0x5fff
	bankRegisterMask = 0xe100

	prgWindowSize = 0x8000
	chrWindowSize = 0x2000
	prgROMMaximum = 0x10000
	chrROMMaximum = 0x10000

	prgBankShift = 3
	prgBankMask  = 0x01
	chrBankMask  = 0x07
)

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
	m.SetPrgWindowSize(prgWindowSize)
	m.SetChrWindowSize(chrWindowSize)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}
	m.AddWriteHook(bankRegisterBase, bankRegisterEnd, m.selectBanks)

	return m, nil
}

type mapperNINA003 struct{ *mapperbase.Base }

// SnapshotFormat identifies the direct mapper state format.
func (*mapperNINA003) SnapshotFormat() string { return "nina003-mapperNINA003-v1" }

func (m *mapperNINA003) selectBanks(address uint16, value byte) error {
	if address&bankRegisterMask != bankRegisterBase {
		return nil
	}

	m.SetPrgWindow(0, int(value>>prgBankShift&prgBankMask))
	m.SetChrWindow(0, int(value&chrBankMask))
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.CHRBanking)

	return nil
}

func validateCartridge(cart *cartridge.Cartridge) error {
	if err := mapperbase.ValidateROMOnly(cart, "nina-003"); err != nil {
		return fmt.Errorf("checking ROM-only metadata: %w", err)
	}
	if err := mapperbase.ValidatePowerOfTwoSize("nina-003 PRG ROM", len(cart.PRG), prgWindowSize, prgROMMaximum); err != nil {
		return fmt.Errorf("checking ROM size: %w", err)
	}

	if err := mapperbase.ValidatePowerOfTwoSize("nina-003 CHR ROM", len(cart.CHR), chrWindowSize, chrROMMaximum); err != nil {
		return fmt.Errorf("checking CHR size: %w", err)
	}

	return nil
}
