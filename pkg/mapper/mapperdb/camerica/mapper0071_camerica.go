// Package camerica implements mapper 71 BF9093 and BF9097 cartridge boards.
// These boards have a switchable 16 KiB lower PRG window, a fixed last bank,
// 8 KiB CHR RAM, and no PRG RAM or bus conflicts. The emulator selects lower
// bank zero at power-on. NES 2.0 submapper 1 selects the Fire Hawk board.
// Legacy headers use fixed mirroring until a write to $9000-$9FFF.
// Register addresses, bank masks, and the legacy mirroring rule follow:
// https://www.nesdev.org/wiki/INES_Mapper_071
// Board ROM capacities follow:
// https://www.nesdev.org/wiki/Camerica_BF909x_pinout
package camerica

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

const legacyMirroringRegisterStart = 0x9000

// New creates a mapper 71 board after it checks the cartridge memory layout.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	cart := base.Cartridge()
	if err := validateCartridge(cart); err != nil {
		return nil, err
	}

	m := &mapper71{
		Base:            base,
		bankMask:        0x0f,
		legacyMirroring: cart.NES2 == nil,
	}
	m.SetName("Camerica")
	m.SetPrgRAM(nil)
	m.SetChrRAM(make([]byte, 0x2000))
	m.SetPrgWindowSize(0x4000)
	m.DeclareFeature(feature.PRGBanking)
	m.Initialize()
	m.SetPrgWindow(1, -1)
	m.AddWriteHook(0xc000, 0xffff, m.selectPRG)
	fireHawk := cart.NES2 != nil && cart.NES2.Submapper == 1
	if fireHawk {
		m.bankMask = 7
	}

	if m.legacyMirroring || fireHawk {
		m.DeclareFeature(feature.Mirroring)
		m.DeclareFeature(feature.SingleScreenMirroring)
		m.AddWriteHook(0x8000, 0x9fff, m.selectMirroring)
	}

	return m, nil
}

type mapper71 struct {
	*mapperbase.Base

	bankMask        byte
	legacyMirroring bool
}

func (m *mapper71) selectPRG(_ uint16, value byte) error {
	m.SetPrgWindow(0, int(value&m.bankMask))
	m.MarkFeature(feature.PRGBanking)

	return nil
}

func (m *mapper71) selectMirroring(address uint16, value byte) error {
	if m.legacyMirroring && address < legacyMirroringRegisterStart {
		return nil
	}

	mode := cartridge.MirrorSingle0
	if value&0x10 != 0 {
		mode = cartridge.MirrorSingle1
	}

	m.MarkFeature(feature.Mirroring)
	m.MarkFeature(feature.SingleScreenMirroring)

	if err := m.SetNameTableMirrorMode(mode); err != nil {
		return fmt.Errorf("setting mapper 71 mirroring: %w", err)
	}

	return nil
}

func validateCartridge(cart *cartridge.Cartridge) error {
	size := len(cart.PRG)
	if size < 0x10000 || size > 0x40000 || size&(size-1) != 0 {
		return fmt.Errorf("unsupported mapper 71 PRG ROM size %d", size)
	}
	if len(cart.CHR) != 0 || mapperbase.ChrRAMSize(cart) != 0x2000 {
		return errors.New("mapper 71 requires 8 KiB CHR RAM and no CHR ROM")
	}
	if cart.Mirror != cartridge.MirrorHorizontal && cart.Mirror != cartridge.MirrorVertical {
		return errors.New("mapper 71 requires horizontal or vertical initial mirroring")
	}
	if cart.Battery != 0 || len(cart.Trainer) != 0 {
		return errors.New("mapper 71 battery or trainer is not supported")
	}

	return validateMetadata(cart)
}

func validateMetadata(cart *cartridge.Cartridge) error {
	if cart.NES2 == nil {
		return nil
	}
	if cart.NES2.Submapper > 1 {
		return fmt.Errorf("unsupported mapper 71 submapper %d", cart.NES2.Submapper)
	}
	if cart.NES2.Submapper == 1 && len(cart.PRG) > 0x20000 {
		return errors.New("mapper 71 Fire Hawk PRG ROM exceeds 128 KiB")
	}
	if mapperbase.PrgRAMTotal(cart.NES2.RAMSizes) != 0 || cart.NES2.RAMSizes.CHRNonvolatile != 0 {
		return errors.New("mapper 71 PRG RAM or nonvolatile CHR RAM is not supported")
	}

	return nil
}
