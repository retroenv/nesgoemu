// Package gxrom implements mapper 66 GNROM and MHROM boards.
// PRG ROM uses one, two, or four 32 KiB banks. CHR ROM uses one, two, or
// four 8 KiB banks. Mirroring is fixed, and the boards have no cartridge RAM.
// NES 2.0 submapper 0 is supported. Oversized variants are not supported.
// Bank masks, bus conflicts, and board wiring follow:
// https://www.nesdev.org/wiki/GxROM
// https://www.nesdev.org/wiki/Bus_conflict#Emulating_bus_conflicts
package gxrom

import (
	"fmt"
	"math"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

const (
	prgWindowSize = 0x8000
	chrWindowSize = 0x2000
	prgROMMaximum = 0x20000
	chrROMMaximum = 0x8000

	prgBankShift = 4
	prgBankMask  = 0x03
	chrBankMask  = 0x03
)

// New creates a mapper 66 board with ROM and fixed mirroring.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	if err := validateCartridge(base.Cartridge()); err != nil {
		return nil, err
	}

	base.SetPrgRAM(nil)
	base.SetChrRAM(nil)
	m := &mapperGxROM{
		Base: base,
	}
	m.SetName("GxROM")
	m.DeclareFeature(feature.PRGBanking)
	m.DeclareFeature(feature.CHRBanking)
	m.SetPrgWindowSize(prgWindowSize)
	m.SetChrWindowSize(chrWindowSize)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}
	m.AddWriteHook(nes.CodeBaseAddress, math.MaxUint16, m.selectBanks)

	return m, nil
}

type mapperGxROM struct {
	*mapperbase.Base
}

// SnapshotFormat identifies the direct mapper state format.
func (*mapperGxROM) SnapshotFormat() string { return "gxrom-mapperGxROM-v1" }

func (m *mapperGxROM) selectBanks(address uint16, value byte) error {
	// The ROM and CPU both drive the bus. Apply the conflict before the
	// bank changes. Only bits 5-4 and 1-0 connect to the bank latch.
	value &= m.Read(address)
	m.SetPrgWindow(0, int(value>>prgBankShift&prgBankMask))
	m.SetChrWindow(0, int(value&chrBankMask))
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.CHRBanking)

	return nil
}

func validateCartridge(cart *cartridge.Cartridge) error {
	if err := mapperbase.ValidateROMOnly(cart, "gxrom"); err != nil {
		return fmt.Errorf("checking ROM-only metadata: %w", err)
	}
	if err := mapperbase.ValidatePowerOfTwoSize("gxrom PRG ROM", len(cart.PRG), prgWindowSize, prgROMMaximum); err != nil {
		return fmt.Errorf("checking ROM size: %w", err)
	}

	if err := mapperbase.ValidatePowerOfTwoSize("gxrom CHR ROM", len(cart.CHR), chrWindowSize, chrROMMaximum); err != nil {
		return fmt.Errorf("checking CHR size: %w", err)
	}

	return nil
}
