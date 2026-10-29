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
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}
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
	if err := validateBoard(cart, "mapper 71"); err != nil {
		return err
	}
	if err := validateSubmapper(cart, "mapper 71"); err != nil {
		return err
	}
	if cart.NES2 != nil && cart.NES2.Submapper == 1 && len(cart.PRG) > 0x20000 {
		return errors.New("mapper 71 Fire Hawk PRG ROM exceeds 128 KiB")
	}

	return nil
}
