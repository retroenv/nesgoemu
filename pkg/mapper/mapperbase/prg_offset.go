package mapperbase

import "github.com/retroenv/retrogolib/arch/system/nes"

// PRGROMOffset returns the physical PRG ROM offset at $8000-$FFFF.
// A read hook that replaces memory data returns false.
// The query does not call read hooks or change mapper state.
func (b *Base) PRGROMOffset(address uint16) (int, bool) {
	if address < nes.CodeBaseAddress {
		return 0, false
	}
	for _, hook := range b.readHooks {
		if address >= hook.startAddress && address <= hook.endAddress && !hook.onlyProxy {
			return 0, false
		}
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.prgBanks) == 0 || b.prgBankMapper == nil {
		return 0, false
	}
	bank, offset := b.prgBankMapper(address)
	return bank*b.prgWindowSize + int(offset), true
}
