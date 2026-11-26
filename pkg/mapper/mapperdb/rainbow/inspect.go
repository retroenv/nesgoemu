package rainbow

import (
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
)

// InspectCHR returns the pattern table byte at a PPU address as it is mapped
// now, without PPU bus observation or feature marking.
func (m *Mapper) InspectCHR(address uint16) byte {
	address = m.chrReadAddress(address & (nes.PatternTableEndExclusive - 1))
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

// InspectCPU reads the current CPU mapping without mapper bus effects.
// PRG bank selection follows readPRG and readPRGRAM.
func (m *Mapper) InspectCPU(address uint16) (byte, bool) {
	if value, ok := m.inspectVector(address); ok {
		return value, true
	}

	if address >= nes.CodeBaseAddress {
		index, offset, size := m.prgHighBankMapping(address)
		bank := m.highBanks[index]
		physical := int(bank&prgBankIndexMask)*size + int(offset)
		if bank&prgHighBankRAMBit != 0 {
			return inspectByte(m.prgRAM, physical)
		}
		if len(m.prgROM) == 0 {
			return 0, false
		}
		return m.prgFlash.read(m.prgROM, physical), true
	}

	bank, offset, size, ok := m.lowMapping(address)
	if !ok {
		return 0, false
	}

	physical := int(bank&prgBankIndexMask)*size + offset
	switch (bank >> prgLowBankSourceShift) & prgLowBankSourceMask {
	case prgLowBankSourceROM0, prgLowBankSourceROM1:
		if len(m.prgROM) == 0 {
			return 0, false
		}
		return m.prgFlash.read(m.prgROM, physical), true

	case prgLowBankSourceRAM:
		return inspectByte(m.prgRAM, physical)
	case prgLowBankSourceFPGA:
		return inspectByte(m.fpgaRAM[:], physical)
	default:
		return 0, false
	}
}

// CPURAMOffset reports a mapped PRG-RAM destination. ROM and FPGA are refused.
func (m *Mapper) CPURAMOffset(address uint16) (int, bool) {
	bank, offset, size, ok := m.lowMapping(address)
	if !ok || len(m.prgRAM) == 0 || (bank>>prgLowBankSourceShift)&prgLowBankSourceMask != prgLowBankSourceRAM {
		return 0, false
	}

	return (int(bank&prgBankIndexMask)*size + offset) % len(m.prgRAM), true
}

// WritePRGRAM changes one physical RAM byte without mapper bus effects.
func (m *Mapper) WritePRGRAM(offset int, value byte) bool {
	if offset < 0 || offset >= len(m.prgRAM) {
		return false
	}
	m.prgRAM[offset] = value

	return true
}

func (m *Mapper) inspectVector(address uint16) (byte, bool) {
	switch address {
	case cpu6502.NMIAddress:
		if m.nmiVectorEnabled {
			return byte(m.nmiAddr), true
		}

	case nmiVectorUpper:
		if m.nmiVectorEnabled {
			return byte(m.nmiAddr >> registerByteShift), true
		}

	case cpu6502.IrqAddress:
		if m.irqVectorEnabled {
			return byte(m.irqAddr), true
		}

	case irqVectorUpper:
		if m.irqVectorEnabled {
			return byte(m.irqAddr >> registerByteShift), true
		}
	}

	return 0, false
}

func (m *Mapper) lowMapping(address uint16) (uint16, int, int, bool) {
	if address < mapperbase.PRGRAMStart || address > mapperbase.PRGRAMEnd {
		return 0, 0, 0, false
	}

	offset := int(address - mapperbase.PRGRAMStart)
	if m.prgRAMMode == prgRAMMode8K {
		return m.lowBanks[0], offset, prgWindowSize8K, true
	}
	return m.lowBanks[offset/prgWindowSize4K], offset % prgWindowSize4K, prgWindowSize4K, true
}

func inspectByte(data []byte, offset int) (byte, bool) {
	if len(data) == 0 || offset < 0 {
		return 0, false
	}
	return data[offset%len(data)], true
}
