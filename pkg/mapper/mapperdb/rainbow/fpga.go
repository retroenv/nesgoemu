package rainbow

// Specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#fpga-ram-auto-readerwriter-415c-415f

const (
	fpgaPageSize             = 0x100
	fpgaBankedPageSize       = 0x1000 // 4K per banked page.
	fpgaAddressMask          = fpgaRAMSize - 1
	fpgaBankMask             = 0x01
	fpgaAutoAddressUpperMask = 0x1F
)

func (m *Mapper) readFPGA(address uint16) uint8 {
	offset := int(address - fpgaBankedStart)
	page := int(m.fpgaBankSelect & fpgaBankMask)
	byteOffset := page*fpgaBankedPageSize + offset
	return m.fpgaRAM[byteOffset%fpgaRAMSize]
}

func (m *Mapper) writeFPGA(address uint16, value uint8) {
	offset := int(address - fpgaBankedStart)
	page := int(m.fpgaBankSelect & fpgaBankMask)
	byteOffset := page*fpgaBankedPageSize + offset
	m.fpgaRAM[byteOffset%fpgaRAMSize] = value
}

func (m *Mapper) readAutoData() uint8 {
	value := m.fpgaRAM[m.fpgaAutoAddr&fpgaAddressMask]
	m.advanceAutoAddr()
	return value
}

func (m *Mapper) writeAutoData(value uint8) {
	m.fpgaRAM[m.fpgaAutoAddr&fpgaAddressMask] = value
	m.advanceAutoAddr()
}

func (m *Mapper) advanceAutoAddr() {
	m.fpgaAutoAddr = (m.fpgaAutoAddr + uint16(m.fpgaAutoInc)) & fpgaAddressMask
}
