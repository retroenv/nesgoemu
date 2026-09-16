package rainbow

// Specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#prg-banking-modes-4100-readwrite

// PRG banking mode window configurations for $8000-$FFFF:
//
// Mode 0: 1×32K window at $8000-$FFFF → highBanks[0].
// Mode 1: 2×16K windows → highBanks[0], highBanks[4].
// Mode 2: 16K+8K+8K → highBanks[0], highBanks[4], highBanks[6].
// Mode 3: 4×8K windows → highBanks[0], highBanks[2], highBanks[4], highBanks[6].
// Mode 4: 8×4K windows → highBanks[0..7].
//
// $6000-$7FFF uses lowBanks[0..1] with 3-way chip select via bits [15:14]:
//   0x = PRG-ROM, 10 = PRG-RAM, 11 = FPGA-RAM.

const (
	prgHighBankCount = 8
	prgLowBankCount  = 2

	prgWindowSize32K = 0x8000
	prgWindowSize16K = 0x4000
	prgWindowSize8K  = 0x2000
	prgWindowSize4K  = 0x1000

	prgHighBankIndexMask  = 0x07FF
	prgHighBankRAMBit     = 0x8000
	prgLowBankIndexMask   = 0x0FFF
	prgLowBankSourceMask  = 0x03
	prgLowBankSourceShift = 14

	prgLowBankSourceROM0 = 0
	prgLowBankSourceROM1 = 1
	prgLowBankSourceRAM  = 2
	prgLowBankSourceFPGA = 3

	prgRAMMode8K = 0
	prgRAMMode4K = 1

	prgMode32K       = 0
	prgMode16K       = 1
	prgMode16K8K8K   = 2
	prgMode8K        = 3
	prgMode4KLowest  = 4
	prgMode4KHighest = 7
)

func (m *Mapper) readPRG(address uint16) uint8 {
	regIdx, offset, windowSize := m.prgHighBankMapping(address)
	bank := m.highBanks[regIdx]

	bankIndex := int(bank & prgHighBankIndexMask)
	byteOffset := bankIndex*windowSize + int(offset)

	if bank&prgHighBankRAMBit == 0 {
		return m.readFromPRGROM(byteOffset)
	}
	return m.readFromPRGRAM(byteOffset)
}

func (m *Mapper) writePRG(address uint16, value uint8) {
	regIdx, offset, windowSize := m.prgHighBankMapping(address)
	bank := m.highBanks[regIdx]
	if bank&prgHighBankRAMBit != 0 {
		m.writeToRAM(m.prgRAM, int(bank&prgHighBankIndexMask)*windowSize+int(offset), value)
	} else {
		m.prgFlash.write(m.prgROM, int(bank&prgHighBankIndexMask)*windowSize+int(offset), value)
	}
}

func (m *Mapper) readPRGRAM(address uint16) uint8 {
	offset := int(address - prgRAMStart)

	if m.prgRAMMode == prgRAMMode8K {
		return m.readLowBank(m.lowBanks[0], offset, prgWindowSize8K)
	}

	// Mode 1: 2×4K windows.
	windowIdx := offset / prgWindowSize4K
	return m.readLowBank(m.lowBanks[windowIdx], offset%prgWindowSize4K, prgWindowSize4K)
}

func (m *Mapper) writePRGRAM(address uint16, value uint8) {
	offset := int(address - prgRAMStart)

	if m.prgRAMMode == prgRAMMode8K {
		m.writeLowBank(m.lowBanks[0], offset, prgWindowSize8K, value)
		return
	}

	// Mode 1: 2×4K windows.
	windowIdx := offset / prgWindowSize4K
	m.writeLowBank(m.lowBanks[windowIdx], offset%prgWindowSize4K, prgWindowSize4K, value)
}

// readLowBank reads from $6000-$7FFF using 3-way chip select.
func (m *Mapper) readLowBank(bank uint16, offset, windowSize int) uint8 {
	bankIndex := int(bank & prgLowBankIndexMask)
	byteOffset := bankIndex*windowSize + offset

	switch (bank >> prgLowBankSourceShift) & prgLowBankSourceMask {
	case prgLowBankSourceROM0, prgLowBankSourceROM1:
		// PRG-ROM.
		return m.readFromPRGROM(byteOffset)
	case prgLowBankSourceRAM:
		// PRG-RAM.
		return m.readFromPRGRAM(byteOffset)
	case prgLowBankSourceFPGA:
		// FPGA-RAM.
		return m.fpgaRAM[byteOffset%fpgaRAMSize]
	}
	return 0
}

// writeLowBank writes to $6000-$7FFF using 3-way chip select.
func (m *Mapper) writeLowBank(bank uint16, offset, windowSize int, value uint8) {
	bankIndex := int(bank & prgLowBankIndexMask)
	byteOffset := bankIndex*windowSize + offset

	switch (bank >> prgLowBankSourceShift) & prgLowBankSourceMask {
	case prgLowBankSourceROM0, prgLowBankSourceROM1:
		m.prgFlash.write(m.prgROM, byteOffset, value)
	case prgLowBankSourceRAM:
		// PRG-RAM.
		m.writeToRAM(m.prgRAM, byteOffset, value)
	case prgLowBankSourceFPGA:
		// FPGA-RAM.
		m.fpgaRAM[byteOffset%fpgaRAMSize] = value
	}
}

func (m *Mapper) prgHighBankMapping(address uint16) (int, uint16, int) {
	addr := address - prgROMStart

	switch m.prgMode {
	case prgMode32K:
		return 0, addr, prgWindowSize32K

	case prgMode16K:
		// 2×16K: highBanks[0] and highBanks[4].
		if addr < prgWindowSize16K {
			return 0, addr, prgWindowSize16K
		}
		return 4, addr - prgWindowSize16K, prgWindowSize16K

	case prgMode16K8K8K:
		return m.prgMode2Mapping(addr)

	case prgMode8K:
		// 4×8K: highBanks[0], [2], [4], [6].
		windowIdx := int(addr / prgWindowSize8K)
		return windowIdx * 2, addr % prgWindowSize8K, prgWindowSize8K

	case prgMode4KLowest, 5, 6, prgMode4KHighest:
		// 8×4K: highBanks[0..7].
		windowIdx := int(addr / prgWindowSize4K)
		return windowIdx, addr % prgWindowSize4K, prgWindowSize4K

	default:
		return 0, addr, prgWindowSize32K
	}
}

func (m *Mapper) prgMode2Mapping(addr uint16) (int, uint16, int) {
	switch {
	case addr < prgWindowSize16K:
		// $8000-$BFFF: 16K window, highBanks[0].
		return 0, addr, prgWindowSize16K

	case addr < prgWindowSize16K+prgWindowSize8K:
		// $C000-$DFFF: 8K window, highBanks[4].
		return 4, addr - prgWindowSize16K, prgWindowSize8K

	default:
		// $E000-$FFFF: 8K window, highBanks[6].
		return 6, addr - (prgWindowSize16K + prgWindowSize8K), prgWindowSize8K
	}
}

func (m *Mapper) readFromPRGROM(offset int) uint8 {
	return m.prgFlash.read(m.prgROM, offset)
}

func (m *Mapper) readFromPRGRAM(offset int) uint8 {
	if len(m.prgRAM) == 0 {
		return 0
	}
	return m.prgRAM[offset%len(m.prgRAM)]
}

func (m *Mapper) writeToRAM(ram []byte, offset int, value uint8) {
	if len(ram) == 0 {
		return
	}
	ram[offset%len(ram)] = value
}
