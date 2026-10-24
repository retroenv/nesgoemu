package mapperbase

// setDefaultBankSizes sets the default CHR and PRG sizes based on the set window size.
func (b *Base) setDefaultBankSizes() {
	b.setDefaultChrBankSizes()
	b.setDefaultPrgBankSizes()
}

// setBanks sets the bank data based on each bank's length. This needs to be called after the bank lengths
// have been set.
func (b *Base) setBanks() {
	b.setChrBanks()
	b.setPrgBanks()
	b.createNameTableBanks()
}

// setWindows sets the CHR and PRG windows to banks based on a static window size.
func (b *Base) setWindows() {
	b.chrWindows = initialWindows(chrMemSize/b.chrWindowSize, len(b.chrBanks))
	b.prgWindows = initialWindows(prgMemSize/b.prgWindowSize, len(b.prgBanks))
}

type bank struct {
	data   []byte
	length int
}

// initialWindows returns the bank numbers of the windows after initialization.
// The windows use the banks in order and start again at the first bank after the last bank.
func initialWindows(windowCount, bankCount int) []int {
	windows := make([]int, windowCount)
	bank := 0
	for i := range windows {
		windows[i] = bank

		if bank+1 < bankCount {
			bank++
		} else {
			bank = 0
		}
	}
	return windows
}

// defaultBanks creates banks of the window size that cover the memory size.
func defaultBanks(memorySize, windowSize int) []bank {
	banks := make([]bank, memorySize/windowSize)
	for i := range banks {
		banks[i].length = windowSize
	}
	return banks
}

// assignBankData sets the data of each bank to its part of the memory, based on each bank's length.
func assignBankData(banks []bank, memory []byte) {
	startOffset := 0
	for i := range banks {
		bank := &banks[i]
		endOffset := startOffset + bank.length
		bank.data = memory[startOffset:endOffset]
		startOffset += bank.length
	}
}

// bankIndex returns the bank number for a bank argument of a window setter.
// A negative bank counts from the end of the memory. A bank that is too large wraps around.
// The bank count must not be zero.
func bankIndex(bank, bankCount int) int {
	if bank < 0 {
		bank += bankCount
	}
	return bank % bankCount
}
