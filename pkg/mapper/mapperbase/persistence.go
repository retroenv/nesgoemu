package mapperbase

import (
	"errors"
	"fmt"
	"io"
)

var errBatteryData = errors.New("invalid prg ram save data")

// BatteryBacked reports whether the cartridge has nonvolatile PRG RAM.
func (b *Base) BatteryBacked() bool {
	cart := b.Cartridge()
	if cart.NES2 != nil {
		return cart.NES2.RAMSizes.PRGNonvolatile > 0
	}

	return cart.Battery != 0
}

// LoadBattery replaces the nonvolatile PRG RAM with saved data.
// Call this method while emulation is stopped.
func (b *Base) LoadBattery(reader io.Reader) error {
	ram, err := b.nonvolatilePrgRAM()
	if err != nil {
		return err
	}

	data, err := io.ReadAll(io.LimitReader(reader, int64(len(ram)+1)))
	if err != nil {
		return fmt.Errorf("reading prg ram save: %w", err)
	}
	if len(data) != len(ram) {
		return errBatteryData
	}

	copy(ram, data)
	return nil
}

// SaveBattery writes the nonvolatile PRG RAM.
// Call this method while emulation is stopped.
func (b *Base) SaveBattery(writer io.Writer) error {
	ram, err := b.nonvolatilePrgRAM()
	if err != nil {
		return err
	}

	written, err := writer.Write(ram)
	if err != nil {
		return fmt.Errorf("writing prg ram save: %w", err)
	}
	if written != len(ram) {
		return fmt.Errorf("writing prg ram save: %w", io.ErrShortWrite)
	}

	return nil
}

func (b *Base) nonvolatilePrgRAM() ([]byte, error) {
	cart := b.Cartridge()
	if cart.NES2 == nil {
		if cart.Battery == 0 {
			return nil, nil
		}

		return b.prgRAM, nil
	}

	start := cart.NES2.RAMSizes.PRGVolatile
	size := cart.NES2.RAMSizes.PRGNonvolatile
	if start < 0 || size < 0 || start > len(b.prgRAM) || size > len(b.prgRAM)-start {
		return nil, errBatteryData
	}

	return b.prgRAM[start : start+size], nil
}
