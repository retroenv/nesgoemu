package mapperbase

import "github.com/retroenv/retrogolib/arch/system/nes/cartridge"

// defaultChrRAMSize is the CHR RAM size of a legacy iNES file without CHR ROM.
const defaultChrRAMSize = 0x2000

// ChrBankCount returns the amount of CHR banks.
func (b *Base) ChrBankCount() int {
	return len(b.chrBanks)
}

// SetChrWindow sets a CHR window to a specific bank.
// A negative bank counts from the end of the CHR memory. -1 is the last bank,
// and -2 is the bank before it. A bank that is too large wraps around.
// Without CHR memory, the call has no effect.
func (b *Base) SetChrWindow(window, bank int) {
	if len(b.chrBanks) == 0 {
		return
	}
	bank = bankIndex(bank, len(b.chrBanks))

	b.mu.Lock()
	changed := b.chrWindows[window] != bank
	b.chrWindows[window] = bank
	b.mu.Unlock()
	if changed {
		b.notifyCHRMapping(window, bank)
	}
}

// SetChrWindowSize sets the CHR window size.
func (b *Base) SetChrWindowSize(size int) {
	b.chrWindowSize = size
}

// SetChrRAM enables the usage of CHR RAM and sets the RAM buffer.
func (b *Base) SetChrRAM(ram []byte) {
	b.chrRAM = ram
}

// setDefaultChrBankSizes creates the banks with default lengths set based on the CHR and CHR window size.
// Some mapper and modes can have a mix of sizes, for example two 2KB banks and four 1KB banks for MMC6.
// In case of mixed sizing the bank sizes need to be initialized manually.
func (b *Base) setDefaultChrBankSizes() {
	chrSize := len(b.bus.Cartridge.CHR)
	if chrSize == 0 {
		chrSize = len(b.chrRAM)
	}

	b.chrBanks = defaultBanks(chrSize, b.chrWindowSize)
}

// setChrBanks sets the bank data based on each bank's length. This needs to be called after the bank lengths
// have been set, either by calling setDefaultChrBankSizes() or setting it manually.
func (b *Base) setChrBanks() {
	chr := b.bus.Cartridge.CHR
	if len(chr) == 0 {
		chr = b.chrRAM
	}

	assignBankData(b.chrBanks, chr)
}

// ChrRAMSize returns the CHR RAM size of a cartridge without CHR ROM. A legacy
// iNES file without CHR ROM specifies 8 KiB of CHR RAM. An NES 2.0 file
// specifies the size in its header. A cartridge with CHR ROM gets no default
// CHR RAM, because only some mappers support both.
// https://www.nesdev.org/wiki/INES
// https://www.nesdev.org/wiki/NES_2.0#CHR-RAM_size
func ChrRAMSize(cart *cartridge.Cartridge) int {
	if len(cart.CHR) > 0 {
		return 0
	}
	if cart.NES2 != nil {
		return ChrRAMTotal(cart.NES2.RAMSizes)
	}
	return defaultChrRAMSize
}

// ChrRAMTotal returns the sum of the volatile and the nonvolatile CHR RAM sizes of NES 2.0 metadata.
func ChrRAMTotal(sizes cartridge.RAMSizes) int {
	return sizes.CHRVolatile + sizes.CHRNonvolatile
}
