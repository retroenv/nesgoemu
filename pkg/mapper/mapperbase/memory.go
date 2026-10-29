package mapperbase

import "fmt"

func (b *Base) prepareMemory() error {
	cart := b.Cartridge()
	if err := ValidateRAMSizes(cart); err != nil {
		return err
	}

	prgRAM, chrRAM := b.MemorySizes()
	if !b.prgRAMConfigured {
		prgRAM = PrgRAMSize(cart)
	}

	if !b.chrRAMConfigured {
		chrRAM = ChrRAMSize(cart)
	}

	chrSize := len(cart.CHR)
	if chrSize == 0 {
		chrSize = chrRAM
	}

	if err := b.validateBanks(len(cart.PRG), chrSize); err != nil {
		return err
	}
	if !b.prgRAMConfigured {
		b.SetPrgRAM(make([]byte, prgRAM))
	}

	if !b.chrRAMConfigured {
		b.SetChrRAM(make([]byte, chrRAM))
	}

	return nil
}

func (b *Base) validateBanks(prgSize, chrSize int) error {
	if b.prgWindowSize <= 0 || b.prgWindowSize > prgMemSize || prgMemSize%b.prgWindowSize != 0 {
		return fmt.Errorf("invalid PRG window size %d", b.prgWindowSize)
	}
	if b.chrWindowSize <= 0 || b.chrWindowSize > chrMemSize || chrMemSize%b.chrWindowSize != 0 {
		return fmt.Errorf("invalid CHR window size %d", b.chrWindowSize)
	}
	if b.nameTableCount <= 0 {
		return fmt.Errorf("invalid nametable count %d", b.nameTableCount)
	}
	if err := ValidateSize("PRG ROM", prgSize, b.prgWindowSize, prgSize, b.prgWindowSize); err != nil {
		return err
	}

	return ValidateSize("CHR memory", chrSize, 0, chrSize, b.chrWindowSize)
}
