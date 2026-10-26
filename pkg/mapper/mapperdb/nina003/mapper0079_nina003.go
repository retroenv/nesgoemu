// Package nina003 implements mapper 79 NINA-003 and NINA-006 boards.
// The boards use 32 or 64 KiB PRG ROM, 8-64 KiB CHR ROM, and fixed mirroring.
// They have no cartridge RAM. NES 2.0 submapper 0 is supported.
// Register decoding, latch bits, and bus behavior follow:
// https://www.nesdev.org/wiki/NINA-003-006
package nina003

import (
	"errors"
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
	m.Initialize()
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
	if cart.NES2 != nil {
		if cart.NES2.Submapper != 0 {
			return fmt.Errorf("nina-003 submapper %d is not supported", cart.NES2.Submapper)
		}

		if cart.NES2.RAMSizes != (cartridge.RAMSizes{}) {
			return errors.New("nina-003 cartridge RAM is not supported")
		}
	}

	if cart.Battery != 0 || len(cart.Trainer) != 0 {
		return errors.New("nina-003 battery or trainer is not supported")
	}

	if cart.Mirror != cartridge.MirrorHorizontal && cart.Mirror != cartridge.MirrorVertical {
		return errors.New("nina-003 requires horizontal or vertical mirroring")
	}

	if size := len(cart.PRG); size != 0x8000 && size != 0x10000 {
		return fmt.Errorf("unsupported nina-003 PRG ROM size %d", size)
	}

	if size := len(cart.CHR); size < 0x2000 || size > 0x10000 || size&(size-1) != 0 {
		return fmt.Errorf("unsupported nina-003 CHR ROM size %d", size)
	}

	return nil
}
