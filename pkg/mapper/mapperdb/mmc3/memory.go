package mmc3

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
)

// validateCartridge checks the standard MMC3 memory limits before bank setup.
// Mixed CHR ROM and RAM require mapper 119. Other explicit submappers use
// different registers, IRQ behavior, or fixed mirroring.
// https://www.nesdev.org/wiki/MMC3
// https://www.nesdev.org/wiki/NES_2.0_submappers#004:_MMC3
func validateCartridge(base *mapperbase.Base) error {
	cart := base.Cartridge()
	prgRAM := mapperbase.PrgRAMSize(cart)
	chrRAM := mapperbase.ChrRAMSize(cart)
	if err := mapperbase.ValidateRAMSizes(cart); err != nil {
		return fmt.Errorf("checking RAM sizes: %w", err)
	}
	if err := mapperbase.ValidateSubmapper(cart, "mmc3", 0); err != nil {
		return fmt.Errorf("checking submapper: %w", err)
	}
	if cart.NES2 != nil {
		sizes := cart.NES2.RAMSizes
		prgRAM = mapperbase.PrgRAMTotal(sizes)
		chrRAM = mapperbase.ChrRAMTotal(sizes)
	}

	return validateMemorySizes(len(cart.PRG), len(cart.CHR), prgRAM, chrRAM)
}

func validateMemorySizes(prgROM, chrROM, prgRAM, chrRAM int) error {
	if err := mapperbase.ValidateSize("MMC3 PRG ROM", prgROM, 2*prgWindowSize, 0x80000, prgWindowSize); err != nil {
		return fmt.Errorf("checking memory size: %w", err)
	}
	if prgRAM < 0 || prgRAM > 0x2000 {
		return fmt.Errorf("unsupported MMC3 PRG RAM size %d", prgRAM)
	}
	if chrROM > 0 && chrRAM > 0 {
		return fmt.Errorf("mixed MMC3 CHR ROM (%d bytes) and RAM (%d bytes) require mapper 119", chrROM, chrRAM)
	}

	chrSize := chrROM + chrRAM
	if err := mapperbase.ValidateSize("MMC3 CHR", chrSize, 2*chrWindowSize, 0x40000, chrWindowSize); err != nil {
		return fmt.Errorf("checking CHR size: %w", err)
	}

	return nil
}
