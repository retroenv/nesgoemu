package mxm1

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

const (
	reg801PRGBankLow  = 0x5010
	reg801CHRBankHigh = 0x5011
	reg801CHRBankLow  = 0x5013
	reg801PRGBankHigh = 0x5020
	reg801CHRSelect   = 0x5021
	reg801PRGSelect   = 0x5022

	reg801ZeroStatusEnd = 0x5013

	reg801RemapOffset        = 0x50e3
	reg801RemapPosition      = 0x50e4
	reg801RemapNTFlip        = 0x50e5
	reg801RemapPositionAlias = 0x50e6
)

const (
	prgBankHighMask801    = 0x07
	prgBankMask801        = 0x01ff
	chrBankHighMask801    = 0x3f
	chrSelectMask801      = 0x3f
	descriptorBankMask801 = 0x07ff
)

const (
	remapStart801           = 80
	nameTableHeight801      = 240
	verticalNameTableBit801 = 0x0800

	chrAddressMask801 = 0x7fffff
	maxPRGROM801      = 0x400000
	maxPRGRAM801      = 0x100000
)

// New801 constructs mapper 801, submapper 0.
func New801(base *mapperbase.Base) (bus.Mapper, error) {
	return newMapper(base, true)
}

func (m *Mapper) writeBank801(address uint16, value byte) bool {
	switch address {
	case reg801PRGSelect:
		m.prgSelect = value & prgSelectMask
		m.prgFlags[m.prgSelect] = value & bankSourceRAM
		m.mapSelectedCPU()

	case reg801PRGBankHigh, reg801PRGBankLow:
		if address == reg801PRGBankHigh {
			value &= prgBankHighMask801
		}

		m.writePRGBank(address == reg801PRGBankHigh, value)

	case reg801CHRSelect:
		m.chrSelect = value & chrSelectMask801

	case reg801CHRBankHigh, reg801CHRBankLow:
		slot := m.chrSelect
		if slot >= chrWindowCount {
			return true
		}

		if address == reg801CHRBankHigh {
			value &= chrBankHighMask801
		}

		m.writeCHRBank(slot, address == reg801CHRBankHigh, value, false)

	case regPRGBankHigh, regCHRBankLow:
		// These addresses have no bank-write function on mapper 801.

	case regAlternateRAM:
		m.altRAM = value&bankSourceRAM != 0
		if m.altEnabled {
			m.mapAlternateCPU()
		}

	case regAlternateHigh:
		m.writeCPURegisters(address, value&controlEnable)

	default:
		return false
	}

	return true
}

func (m *Mapper) readBankStatus801(address uint16) (byte, bool) {
	switch {
	case address >= regIdentityStart && address <= regIdentityEnd:
		return fpgaIdentityResponse[address-regIdentityStart], true

	case address == regPRGReadback:
		return m.prgFlags[0], true

	case address > regPRGReadback && address <= reg801ZeroStatusEnd:
		return 0, true

	case address > reg801ZeroStatusEnd && address <= regPRGReadbackEnd:
		return m.readPRGBankRecord(address), true

	case address == reg801PRGBankHigh:
		return byte(m.prgBanks[0]), true

	case address == reg801CHRSelect:
		return 0, true

	case address == reg801PRGSelect:
		return byte(m.prgBanks[0] >> 8), true

	default:
		return 0, false
	}
}

// remapAddress801 uses pairs of bytes from the table. The second byte and the
// offset select a row. Bit 0 of the first byte can change the nametable.
func (m *Mapper) remapAddress801(address uint16, position byte) (byte, uint16) {
	index := 2 * (int(m.remapPage)*remapPageSize + int(position))
	row := int(m.remapTable[index+1]) + int(m.remapOffset801)
	table := row/nameTableHeight801&1 != 0
	table = table != (m.remapTable[index]&controlEnable != 0)
	table = table != m.remapNTFlip801
	if position < remapStart801 {
		row = int(position)
	}

	mapped := address & (verticalRemapKeepMask &^ verticalNameTableBit801)
	if table {
		mapped |= verticalNameTableBit801
	}

	row %= nameTableHeight801

	return byte(row), mapped | uint16(row>>3)<<tileRowShift
}

// Mode 3 uses the low descriptor byte for both planes.
func (m *Mapper) descriptorBank801(bank uint16) uint16 {
	if m.attributeMode == attributeModePatternOnly {
		return bank & byteMask
	}

	return bank & descriptorBankMask801
}

func validateLayout801(cart *cartridge.Cartridge) error {
	if cart.NES2 == nil || cart.Mirror != cartridge.Mirror4 {
		return errors.New("mapper 801 requires an NES 2.0 header with four-screen mirroring")
	}
	if err := mapperbase.ValidatePowerOfTwoSize("PRG ROM", len(cart.PRG), prgWindowCount*prgBankSize, maxPRGROM801); err != nil {
		return fmt.Errorf("checking mapper 801 PRG ROM: %w", err)
	}
	if err := mapperbase.ValidateSize("CHR ROM", len(cart.CHR), chrBankSize, chrAddressMask801+1, chrBankSize); err != nil {
		return fmt.Errorf("checking mapper 801 CHR ROM: %w", err)
	}
	if err := mapperbase.ValidateSize("PRG RAM", mapperbase.PrgRAMSize(cart), 0, maxPRGRAM801, prgBankSize); err != nil {
		return fmt.Errorf("checking mapper 801 PRG RAM: %w", err)
	}
	if err := mapperbase.ValidateSize("CHR RAM", mapperbase.ChrRAMTotal(cart.NES2.RAMSizes), 0, defaultCHRRAM, 1); err != nil {
		return fmt.Errorf("checking mapper 801 CHR RAM: %w", err)
	}
	return nil
}
