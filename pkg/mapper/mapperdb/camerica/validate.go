package camerica

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// validateBoard checks the memory layout that all Camerica boards share:
// 64 to 256 KiB PRG ROM, 8 KiB CHR RAM, fixed initial mirroring, and no
// battery, trainer, PRG RAM, or nonvolatile CHR RAM. The name is the mapper
// name that is used in error messages.
func validateBoard(cart *cartridge.Cartridge, name string) error {
	if err := mapperbase.ValidatePowerOfTwoSize(name+" PRG ROM", len(cart.PRG), prgROMMinimum, prgROMMaximum); err != nil {
		return fmt.Errorf("checking PRG ROM size: %w", err)
	}
	if len(cart.CHR) != 0 || mapperbase.ChrRAMSize(cart) != chrRAMSize {
		return fmt.Errorf("%s requires 8 KiB CHR RAM and no CHR ROM", name)
	}
	if err := mapperbase.ValidateFixedMirroring(cart, name); err != nil {
		return fmt.Errorf("checking initial mirroring: %w", err)
	}
	if err := mapperbase.ValidateNoBatteryTrainer(cart, name); err != nil {
		return fmt.Errorf("checking battery and trainer: %w", err)
	}
	if cart.NES2 != nil &&
		(mapperbase.PrgRAMTotal(cart.NES2.RAMSizes) != 0 || cart.NES2.RAMSizes.CHRNonvolatile != 0) {

		return fmt.Errorf("%s PRG RAM or nonvolatile CHR RAM is not supported", name)
	}

	return nil
}

// validateSubmapper accepts NES 2.0 submapper 0 and 1. Both Camerica
// mappers use submapper 1 for a board variant.
func validateSubmapper(cart *cartridge.Cartridge, name string) error {
	if cart.NES2 != nil && cart.NES2.Submapper > 1 {
		return fmt.Errorf("unsupported %s submapper %d", name, cart.NES2.Submapper)
	}

	return nil
}
