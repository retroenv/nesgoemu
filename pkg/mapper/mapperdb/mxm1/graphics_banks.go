package mxm1

// graphicsBanks holds the fields common to the descriptor and nametable bank
// registers. Each mapper applies the register changes to its own memory.
type graphicsBanks struct {
	banks    [ntWindowCount]uint16
	enabled  bool
	selected byte
}

func (b *graphicsBanks) write(address uint16, value byte) {
	switch address {
	case regDescriptorMode, regNametableMode:
		b.enabled = value&extendedModeEnable != 0

	case regDescriptorSelect, regNametableSelect:
		b.selected = value & ntSelectMask

	case regDescriptorHigh, regNametableHigh:
		setBankByte(&b.banks[b.selected], true, value)

	case regDescriptorLow, regNametableLow:
		setBankByte(&b.banks[b.selected], false, value)
	}
}

func descriptorOffset(bank, tile uint16) int {
	return int(bank)*descriptorBankSize + int(tile)*descriptorSize
}
