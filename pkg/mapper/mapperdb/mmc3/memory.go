package mmc3

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
)

// prepareMemory checks the standard MMC3 memory limits before bank setup.
// Mixed CHR ROM and RAM require mapper 119. Other explicit submappers use
// different registers, IRQ behavior, or fixed mirroring.
// https://www.nesdev.org/wiki/MMC3
// https://www.nesdev.org/wiki/NES_2.0_submappers#004:_MMC3
func prepareMemory(base *mapperbase.Base) error {
	cart := base.Cartridge()
	prgRAM, chrRAM := base.MemorySizes()
	if cart.NES2 != nil {
		if cart.NES2.Submapper != 0 {
			return fmt.Errorf("mmc3 submapper %d is not supported", cart.NES2.Submapper)
		}
		sizes := cart.NES2.RAMSizes
		if sizes.PRGVolatile < 0 || sizes.PRGNonvolatile < 0 || sizes.CHRVolatile < 0 || sizes.CHRNonvolatile < 0 {
			return fmt.Errorf("invalid MMC3 RAM sizes: %+v", sizes)
		}
		prgRAM = sizes.PRGVolatile + sizes.PRGNonvolatile
		chrRAM = sizes.CHRVolatile + sizes.CHRNonvolatile
	}

	if err := validateMemorySizes(len(cart.PRG), len(cart.CHR), prgRAM, chrRAM); err != nil {
		return err
	}
	if cart.NES2 != nil && chrRAM > 0 {
		base.SetChrRAM(make([]byte, chrRAM))
	}
	return nil
}

func validateMemorySizes(prgROM, chrROM, prgRAM, chrRAM int) error {
	if prgROM < 2*prgWindowSize || prgROM > 0x80000 || prgROM%prgWindowSize != 0 {
		return fmt.Errorf("unsupported MMC3 PRG ROM size %d", prgROM)
	}
	if prgRAM < 0 || prgRAM > 0x2000 {
		return fmt.Errorf("unsupported MMC3 PRG RAM size %d", prgRAM)
	}
	if chrROM > 0 && chrRAM > 0 {
		return fmt.Errorf("mixed MMC3 CHR ROM (%d bytes) and RAM (%d bytes) require mapper 119", chrROM, chrRAM)
	}
	chrSize := chrROM + chrRAM
	if chrSize < 2*chrWindowSize || chrSize > 0x40000 || chrSize%chrWindowSize != 0 {
		return fmt.Errorf("unsupported MMC3 CHR size %d", chrSize)
	}
	return nil
}
