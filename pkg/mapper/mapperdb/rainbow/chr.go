package rainbow

// Specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#chr-configuration

// CHR banking mode window configurations:
//
// Mode 0: 1×8K window (register 0).
// Mode 1: 2×4K windows (registers 0-1).
// Mode 2: 4×2K windows (registers 0-3).
// Mode 3: 8×1K windows (registers 0-7).
// Mode 4: 16×512B windows (registers 0-15).

const (
	chrBankCount = 16

	chrSourceROM  = 0
	chrSourceRAM  = 1
	chrSourceFPGA = 2
	chrSourceNT   = 3
	chrSourceMask = 0x03

	chrMode8K         = 0
	chrMode4K         = 1
	chrMode2K         = 2
	chrMode1K         = 3
	chrMode512Lowest  = 4
	chrMode512Highest = 7

	chrWindowSize8K  = 0x2000
	chrWindowSize4K  = 0x1000
	chrWindowSize2K  = 0x0800
	chrWindowSize1K  = 0x0400
	chrWindowSize512 = 0x0200

	bgExtBankMask  = 0x3F
	bgExtBankShift = 12
	bgExtModeMask  = 0x1F
	bgExtModeShift = 18

	spriteSize8x16           = 16
	spriteBankUpperShift8x8  = 20
	spriteBankLowerShift8x8  = 12
	spriteBankUpperShift8x16 = 21
	spriteBankLowerShift8x16 = 13
)

func (m *Mapper) bgExtCHROffset(address uint16) int {
	return int((uint32(address) & (chrWindowSize4K - 1)) |
		(uint32(m.bgExtData&bgExtBankMask) << bgExtBankShift) |
		(uint32(m.bgExtModeOffset) << bgExtModeShift))
}

func (m *Mapper) readCHR(address uint16) uint8 {
	m.observePPURead(address)
	if m.windowEnabled && m.activeSpriteIndex < 0 {
		if _, line, inside := m.windowPosition(); inside {
			address = address&^(ppuTileSize-1) |
				uint16((line+int(m.windowSplitRegs[windowSplitYScrollIndex]))%ppuVisibleHeight&(ppuTileSize-1))
		}
	}
	if m.chrSource == chrSourceNT {
		return m.NameTableMemory().ReadCIRAM(address & ciramAddressMask)
	}
	if m.chrSource == chrSourceFPGA {
		return m.fpgaRAM[address&(chrWindowSize4K-1)]
	}
	if m.spriteExtMode && m.activeSpriteIndex >= 0 {
		return m.readSpriteCHR(address)
	}

	regIdx, offset, windowSize := m.chrBankMapping(address)
	bankIndex := int(m.chrBanks[regIdx])
	byteOffset := bankIndex*windowSize + int(offset)

	if m.bgExtActive && m.activeSpriteIndex < 0 {
		byteOffset = m.bgExtCHROffset(address)
	}

	return m.readCHRSource(byteOffset)
}

func (m *Mapper) readSpriteCHR(address uint16) uint8 {
	i := m.activeSpriteIndex
	var addr uint32
	if m.activeSpriteSize == spriteSize8x16 {
		addr = (uint32(m.spriteBankUpper) << spriteBankUpperShift8x16) |
			(uint32(m.spriteBankLower[i]) << spriteBankLowerShift8x16) |
			(uint32(address) & (chrWindowSize8K - 1))
	} else {
		addr = (uint32(m.spriteBankUpper) << spriteBankUpperShift8x8) |
			(uint32(m.spriteBankLower[i]) << spriteBankLowerShift8x8) |
			(uint32(address) & (chrWindowSize4K - 1))
	}
	return m.readCHRSource(int(addr))
}

func (m *Mapper) writeCHR(address uint16, value uint8) {
	if m.chrSource == chrSourceNT {
		m.NameTableMemory().WriteCIRAM(address&ciramAddressMask, value)
		return
	}
	if m.chrSource == chrSourceFPGA {
		m.fpgaRAM[address&(chrWindowSize4K-1)] = value
		return
	}
	regIdx, offset, windowSize := m.chrBankMapping(address)

	bankIndex := int(m.chrBanks[regIdx])
	byteOffset := bankIndex*windowSize + int(offset)

	switch m.chrSource {
	case chrSourceROM:
		m.chrFlash.write(m.chrROM, byteOffset, value)
	case chrSourceRAM:
		m.writeToRAM(m.chrRAM, byteOffset, value)
	case chrSourceNT:
		m.NameTableMemory().WriteCIRAM(uint16(byteOffset&ciramAddressMask), value)
	}
}

func (m *Mapper) chrBankMapping(address uint16) (int, uint16, int) {
	switch m.chrMode {
	case chrMode8K:
		return 0, address, chrWindowSize8K

	case chrMode4K:
		windowIdx := int(address / chrWindowSize4K)
		return windowIdx, address % chrWindowSize4K, chrWindowSize4K

	case chrMode2K:
		windowIdx := int(address / chrWindowSize2K)
		return windowIdx, address % chrWindowSize2K, chrWindowSize2K

	case chrMode1K:
		windowIdx := int(address / chrWindowSize1K)
		return windowIdx, address % chrWindowSize1K, chrWindowSize1K

	case chrMode512Lowest, 5, 6, chrMode512Highest:
		windowIdx := int(address / chrWindowSize512)
		return windowIdx, address % chrWindowSize512, chrWindowSize512

	default:
		return 0, address, chrWindowSize8K
	}
}

func (m *Mapper) readCHRSource(offset int) uint8 {
	switch m.chrSource {
	case chrSourceROM:
		return m.chrFlash.read(m.chrROM, offset)

	case chrSourceRAM:
		if len(m.chrRAM) == 0 {
			return 0
		}
		return m.chrRAM[offset%len(m.chrRAM)]

	case chrSourceFPGA:
		return m.fpgaRAM[offset%fpgaRAMSize]

	case chrSourceNT:
		return m.NameTableMemory().ReadCIRAM(uint16(offset & ciramAddressMask))

	default:
		return 0
	}
}
