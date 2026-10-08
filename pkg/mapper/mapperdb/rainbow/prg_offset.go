package rainbow

// PRGROMOffset returns the physical ROM offset mapped at $8000-$FFFF.
// RAM, flash identification mode, vector registers, and an empty ROM return false.
// This method does not read memory or change mapper state.
func (m *Mapper) PRGROMOffset(address uint16) (int, bool) {
	if address < prgROMStart || len(m.prgROM) == 0 || m.prgFlash.ID {
		return 0, false
	}
	if m.nmiVectorEnabled && (address == nmiVectorLower || address == nmiVectorUpper) {
		return 0, false
	}
	if m.irqVectorEnabled && (address == irqVectorLower || address == irqVectorUpper) {
		return 0, false
	}
	regIdx, offset, windowSize := m.prgHighBankMapping(address)
	bank := m.highBanks[regIdx]
	if bank&prgHighBankRAMBit != 0 {
		return 0, false
	}
	physical := int(bank&prgHighBankIndexMask)*windowSize + int(offset)
	return physical % len(m.prgROM), true
}
