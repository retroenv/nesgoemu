package mapperbase

import (
	"fmt"
	"slices"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// ValidateSize checks an inclusive size range and the bank alignment.
func ValidateSize(name string, size, minimum, maximum, alignment int) error {
	if size < minimum || size > maximum || alignment <= 0 || size%alignment != 0 {
		return fmt.Errorf("unsupported %s size %d", name, size)
	}

	return nil
}

// ValidatePowerOfTwoSize checks an inclusive range of nonzero powers of two.
func ValidatePowerOfTwoSize(name string, size, minimum, maximum int) error {
	if size <= 0 || size < minimum || size > maximum || size&(size-1) != 0 {
		return fmt.Errorf("unsupported %s size %d", name, size)
	}

	return nil
}

// ValidateSubmapper checks an explicit NES 2.0 submapper against the board list.
func ValidateSubmapper(cart *cartridge.Cartridge, name string, supported ...byte) error {
	if cart.NES2 != nil && !slices.Contains(supported, cart.NES2.Submapper) {
		return fmt.Errorf("%s submapper %d is not supported", name, cart.NES2.Submapper)
	}

	return nil
}

// ValidateFixedMirroring permits horizontal or vertical mirroring.
func ValidateFixedMirroring(cart *cartridge.Cartridge, name string) error {
	if cart.Mirror != cartridge.MirrorHorizontal && cart.Mirror != cartridge.MirrorVertical {
		return fmt.Errorf("%s requires horizontal or vertical mirroring", name)
	}

	return nil
}

// ValidateNoBatteryTrainer rejects a battery or trainer on boards without them.
func ValidateNoBatteryTrainer(cart *cartridge.Cartridge, name string) error {
	if cart.Battery != 0 || len(cart.Trainer) != 0 {
		return fmt.Errorf("%s battery or trainer is not supported", name)
	}

	return nil
}

// ValidateROMOnly checks metadata for fixed-mirroring boards without RAM.
func ValidateROMOnly(cart *cartridge.Cartridge, name string) error {
	if err := ValidateSubmapper(cart, name, 0); err != nil {
		return err
	}
	if cart.NES2 != nil && cart.NES2.RAMSizes != (cartridge.RAMSizes{}) {
		return fmt.Errorf("%s cartridge RAM is not supported", name)
	}
	if err := ValidateNoBatteryTrainer(cart, name); err != nil {
		return err
	}

	return ValidateFixedMirroring(cart, name)
}

// ValidateRAMSizes rejects negative sizes and totals that overflow an int.
func ValidateRAMSizes(cart *cartridge.Cartridge) error {
	if cart.NES2 == nil {
		return nil
	}

	sizes := cart.NES2.RAMSizes
	maximum := int(^uint(0) >> 1)
	if sizes.PRGVolatile < 0 || sizes.PRGNonvolatile < 0 || sizes.CHRVolatile < 0 || sizes.CHRNonvolatile < 0 ||
		sizes.PRGVolatile > maximum-sizes.PRGNonvolatile || sizes.CHRVolatile > maximum-sizes.CHRNonvolatile {

		return fmt.Errorf("invalid cartridge RAM sizes: %+v", sizes)
	}

	return nil
}
