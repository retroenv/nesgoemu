// Package bnrom implements mapper 34 BNROM and NINA-001/NINA-002 boards.
// Both boards use 32 KiB PRG windows and fixed nametable mirroring.
// NINA has 8 KiB PRG RAM and two 4 KiB CHR ROM windows. BNROM uses AND bus
// conflicts, no PRG RAM, and fixed 8 KiB CHR ROM or RAM.
// Submappers 1 and 2 select NINA and BNROM. Submapper 0 uses the CHR size.
// PRG ROM supports powers of two from 32 KiB to 8 MiB. NINA CHR ROM supports
// powers of two from 4 KiB to 1 MiB. Bank bits without a ROM address line
// are ignored. Other memory layouts are not supported.
// Register decoding, RAM overlap, and bus conflicts follow:
// https://www.nesdev.org/wiki/INES_Mapper_034
// The original BNROM board uses two bank bits and supports 128 KiB of PRG ROM.
// The BNROM bank register and oversize PRG ROM range follow:
// https://www.nesdev.org/wiki/BNROM
// The NESdev wiki recommends an 8-bit bank register for emulators. It allows
// up to 8 MiB of PRG ROM.
package bnrom

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// New creates a mapper 34 board after it checks the cartridge memory layout.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	cart := base.Cartridge()
	nina, err := ninaBoard(cart)
	if err != nil {
		return nil, err
	}
	if err = validateCartridge(cart, nina); err != nil {
		return nil, err
	}
	m := &mapper34{
		Base: base,
	}
	m.DeclareFeature(feature.PRGBanking)
	m.SetPrgWindowSize(0x8000)
	if nina {
		m.SetName("NINA-001")
		m.SetPrgRAM(make([]byte, 0x2000))
		m.SetChrRAM(nil)
		m.SetChrWindowSize(0x1000)
		m.DeclareFeature(feature.CHRBanking)
		// The default RAM write follows the register hook and keeps the full byte.
		m.AddWriteHook(0x7ffd, 0x7fff, m.selectNINA).SetProxyOnly(true)
	} else {
		m.SetName("BNROM")
		m.SetPrgRAM(nil)
		m.SetChrWindowSize(0x2000)
		if len(cart.CHR) == 0 {
			m.SetChrRAM(make([]byte, 0x2000))
		} else {
			m.SetChrRAM(nil)
		}
		m.AddWriteHook(0x8000, 0xffff, m.selectBNROM)
	}
	m.Initialize()
	return m, nil
}

type mapper34 struct {
	*mapperbase.Base
}

func (m *mapper34) selectBNROM(address uint16, value byte) error {
	value &= m.Read(address)
	m.SetPrgWindow(0, int(value))
	m.MarkFeature(feature.PRGBanking)
	return nil
}

func (m *mapper34) selectNINA(address uint16, value byte) error {
	if address == 0x7ffd {
		m.SetPrgWindow(0, int(value))
		m.MarkFeature(feature.PRGBanking)
	} else {
		m.SetChrWindow(int(address-0x7ffe), int(value))
		m.MarkFeature(feature.CHRBanking)
	}
	return nil
}

func ninaBoard(cart *cartridge.Cartridge) (bool, error) {
	if cart.NES2 != nil {
		switch cart.NES2.Submapper {
		case 0:
		case 1:
			return true, nil
		case 2:
			return false, nil
		default:
			return false, fmt.Errorf("unsupported mapper 34 submapper %d", cart.NES2.Submapper)
		}
	}
	return len(cart.CHR) > 0x2000, nil
}

func validateCartridge(cart *cartridge.Cartridge, nina bool) error {
	if cart.Mirror != cartridge.MirrorHorizontal && cart.Mirror != cartridge.MirrorVertical {
		return errors.New("mapper 34 requires horizontal or vertical mirroring")
	}
	size := len(cart.PRG)
	if size < 0x8000 || size > 0x800000 || size&(size-1) != 0 {
		return fmt.Errorf("unsupported mapper 34 PRG ROM size %d", size)
	}
	if nina {
		return validateNINA(cart)
	}
	return validateBNROM(cart)
}

func validateNINA(cart *cartridge.Cartridge) error {
	size := len(cart.CHR)
	if size < 0x1000 || size > 0x100000 || size&(size-1) != 0 {
		return fmt.Errorf("unsupported NINA CHR ROM size %d", size)
	}
	if cart.NES2 != nil {
		ram := cart.NES2.RAMSizes
		if mapperbase.PrgRAMTotal(ram) != 0x2000 || mapperbase.ChrRAMTotal(ram) != 0 {
			return errors.New("NINA requires 8 KiB PRG RAM and no CHR RAM")
		}
	}
	return nil
}

func validateBNROM(cart *cartridge.Cartridge) error {
	if cart.Battery != 0 || len(cart.Trainer) != 0 {
		return errors.New("BNROM battery or trainer is not supported")
	}
	if cart.NES2 != nil {
		ram := cart.NES2.RAMSizes
		if mapperbase.PrgRAMTotal(ram) != 0 {
			return errors.New("BNROM PRG RAM is not supported")
		}
		if len(cart.CHR) > 0 && mapperbase.ChrRAMTotal(ram) != 0 {
			return errors.New("BNROM mixed CHR ROM and RAM is not supported")
		}
	}
	if len(cart.CHR) == 0 {
		if mapperbase.ChrRAMSize(cart) != 0x2000 {
			return errors.New("BNROM requires 8 KiB CHR RAM")
		}
	} else if len(cart.CHR) != 0x2000 {
		return fmt.Errorf("unsupported BNROM CHR ROM size %d", len(cart.CHR))
	}
	return nil
}
