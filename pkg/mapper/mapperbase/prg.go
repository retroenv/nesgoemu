package mapperbase

import (
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// PrgBankCount returns the amount of PRG banks.
func (b *Base) PrgBankCount() int {
	return len(b.prgBanks)
}

// SetPrgWindow sets a PRG window to a specific bank.
func (b *Base) SetPrgWindow(window, bank int) {
	if bank < 0 {
		bank = len(b.prgBanks) + bank
	}
	bank %= len(b.prgBanks)

	b.mu.Lock()
	b.prgWindows[window] = bank
	b.mu.Unlock()
}

// SetPrgWindowSize sets the PRG window size.
func (b *Base) SetPrgWindowSize(size int) {
	b.prgWindowSize = size
}

// SetPrgRAM enables the usage of PRG RAM and sets the RAM buffer.
func (b *Base) SetPrgRAM(ram []byte) {
	b.prgRAM = ram
}

// setDefaultPrgBankSizes creates the banks with default lengths set based on the PRG and PRG window size.
// Some mapper and modes can have a mix of sizes, for example One 16KB bank and two 8KB banks for MMC5.
// In case of mixed sizing the bank sizes need to be initialized manually.
func (b *Base) setDefaultPrgBankSizes() {
	prgSize := len(b.bus.Cartridge.PRG)
	banks := prgSize / b.prgWindowSize
	b.prgBanks = make([]bank, banks)

	for i := range banks {
		bank := &b.prgBanks[i]
		bank.length = b.prgWindowSize
	}
}

// setPrgBanks sets the bank data based on each bank's length. This needs to be called after the bank lengths
// have been set, either by calling setDefaultPrgBankSizes() or setting it manually.
func (b *Base) setPrgBanks() {
	prg := b.bus.Cartridge.PRG
	startOffset := 0

	for i := range len(b.prgBanks) {
		bank := &b.prgBanks[i]
		endOffset := startOffset + bank.length
		bank.data = prg[startOffset:endOffset]
		startOffset += bank.length
	}
}

// PrgRAMSize returns the PRG RAM size of a cartridge in bytes.
//
// NES 2.0 sizes are explicit and a size of zero means that the memory is absent.
// The volatile and the nonvolatile size are added, a mapper maps one RAM block.
//
// A legacy iNES file gets 8 KiB. The format has a size field for PRG RAM, but it
// is rarely used, it can hold unrelated data, and the format defines 8 KiB as
// the default for compatibility. The field is ignored. A size above 8 KiB can
// add no memory, the window at $6000-$7FFF is 8 KiB in size and cannot reach the
// remaining memory.
//
// https://www.nesdev.org/wiki/NES_2.0#PRG-RAM/EEPROM
// https://www.nesdev.org/wiki/INES
func PrgRAMSize(cart *cartridge.Cartridge) int {
	if cart.NES2 != nil {
		return cart.NES2.RAMSizes.PRGVolatile + cart.NES2.RAMSizes.PRGNonvolatile
	}

	return defaultPrgRAMSize
}
