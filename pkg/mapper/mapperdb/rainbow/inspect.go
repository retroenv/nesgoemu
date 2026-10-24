package rainbow

// InspectCHR returns the pattern table byte at a PPU address as it is mapped
// now, without PPU bus observation or feature marking.
func (m *Mapper) InspectCHR(address uint16) byte {
	address = m.chrReadAddress(address & (patternTableEnd - 1))
	switch m.chrSource {
	case chrSourceNT:
		return m.NameTableMemory().ReadCIRAM(address & ciramAddressMask)

	case chrSourceFPGA:
		return m.fpgaRAM[address&(chrWindowSize4K-1)]
	}

	offset := m.chrReadOffset(address)
	switch m.chrSource {
	case chrSourceROM:
		return m.chrFlash.read(m.chrROM, offset)

	case chrSourceRAM:
		if len(m.chrRAM) == 0 {
			return 0
		}

		return m.chrRAM[offset%len(m.chrRAM)]

	default:
		return 0
	}
}

// InspectPRGRAM returns a byte of the physical PRG-RAM array.
func (m *Mapper) InspectPRGRAM(offset int) (byte, bool) {
	if offset < 0 || offset >= len(m.prgRAM) {
		return 0, false
	}

	return m.prgRAM[offset], true
}
