// Package gxrom implements mapper 66 GNROM and MHROM boards.
// PRG ROM uses one, two, or four 32 KiB banks. CHR ROM uses one, two, or
// four 8 KiB banks. Mirroring is fixed, and the boards have no cartridge RAM.
// NES 2.0 submapper 0 is supported. Oversized variants are not supported.
// Bank masks, bus conflicts, and board wiring follow:
// https://www.nesdev.org/wiki/GxROM
// https://www.nesdev.org/wiki/Bus_conflict#Emulating_bus_conflicts
package gxrom

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
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
	m.SetPrgWindowSize(0x8000)
	m.SetChrWindowSize(0x2000)
	m.Initialize()
	m.AddWriteHook(0x8000, 0xffff, m.selectBanks)
	return m, nil
}

type mapperGxROM struct {
	*mapperbase.Base
}

func (m *mapperGxROM) selectBanks(address uint16, value byte) error {
	// The ROM and CPU both drive the bus. Apply the conflict before the
	// bank changes. Only bits 5-4 and 1-0 connect to the bank latch.
	value &= m.Read(address)
	m.SetPrgWindow(0, int(value>>4&3))
	m.SetChrWindow(0, int(value&3))
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.CHRBanking)
	return nil
}

func validateCartridge(cart *cartridge.Cartridge) error {
	if cart.NES2 != nil {
		if cart.NES2.Submapper != 0 {
			return fmt.Errorf("gxrom submapper %d is not supported", cart.NES2.Submapper)
		}
		if cart.NES2.RAMSizes != (cartridge.RAMSizes{}) {
			return errors.New("gxrom cartridge RAM is not supported")
		}
	}
	if cart.Battery != 0 || len(cart.Trainer) != 0 {
		return errors.New("gxrom battery or trainer is not supported")
	}
	if cart.Mirror != cartridge.MirrorHorizontal && cart.Mirror != cartridge.MirrorVertical {
		return errors.New("gxrom requires horizontal or vertical mirroring")
	}
	if size := len(cart.PRG); size != 0x8000 && size != 0x10000 && size != 0x20000 {
		return fmt.Errorf("unsupported gxrom PRG ROM size %d", size)
	}
	if size := len(cart.CHR); size != 0x2000 && size != 0x4000 && size != 0x8000 {
		return fmt.Errorf("unsupported gxrom CHR ROM size %d", size)
	}
	return nil
}
