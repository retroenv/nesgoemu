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
	size := len(cart.PRG)
	if size < 0x10000 || size > 0x40000 || size&(size-1) != 0 {
		return fmt.Errorf("unsupported %s PRG ROM size %d", name, size)
	}
	if len(cart.CHR) != 0 || mapperbase.ChrRAMSize(cart) != 0x2000 {
		return fmt.Errorf("%s requires 8 KiB CHR RAM and no CHR ROM", name)
	}
	if cart.Mirror != cartridge.MirrorHorizontal && cart.Mirror != cartridge.MirrorVertical {
		return fmt.Errorf("%s requires horizontal or vertical initial mirroring", name)
	}
	if cart.Battery != 0 || len(cart.Trainer) != 0 {
		return fmt.Errorf("%s battery or trainer is not supported", name)
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
