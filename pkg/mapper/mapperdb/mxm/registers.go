package mxm

import "github.com/retroenv/nesgoemu/pkg/feature"

func (m *Mapper) writeRegister(address uint16, value byte) {
	switch address & registerGroupMask {
	case regIdentityStart:
		m.writeStatus(address, value)

	case regPRGMode:
		m.writeCPURegisters(address, value)

	case regCHRMode:
		m.writePatternRegisters(address, value)

	case regIRQEnable:
		m.writeIRQRegisters(address, value)

	case regDescriptorMode:
		m.writeDescriptorRegisters(address, value)

	case regNametableMode:
		m.writeNametableRegisters(address, value)

	case regPlaneMode, regWriteBankGroup:
		m.writePlaneRegisters(address, value)

	case regCopyGroup:
		m.writeCopyRegisters(address, value)

	case regMetatileMode:
		m.writeMetatileRegisters(address, value)

	case regRemapMode:
		m.writeRemapRegisters(address, value)
	}
}

func (m *Mapper) writeStatus(address uint16, value byte) {
	if address == regStateCursor {
		m.streamCursor = value
	}
}

func (m *Mapper) writeCPURegisters(address uint16, value byte) {
	switch address {
	case regPRGMode:
		m.prgMode = value & modeMask

	case regPRGSelect:
		// Selection/flags take effect in the register file immediately, but
		// only a bank-byte write below rebuilds the backing memory window.
		m.prgSelect = value & prgSelectMask
		m.prgFlags[m.prgSelect] = value & prgFlagsMask

	case regPRGBankHigh, regPRGBankLow:
		m.MarkFeature(feature.PRGBanking)
		if m.prgSelect == 0 {
			m.MarkFeature(feature.LowBankMapping)
		}
		setBankByte(&m.prgBanks[m.prgSelect], address == regPRGBankHigh, value)
		m.mapSelectedCPU()

	case regAlternateMode:
		if enabled := value&controlEnable != 0; enabled != m.altEnabled {
			m.altEnabled = enabled
			m.mapAlternateCPU()
		}

	case regAlternateRAM:
		m.altRAM = value&bankSourceRAM != 0

	case regAlternateHigh, regAlternateLow:
		setBankByte(&m.altBank, address == regAlternateHigh, value)
		if m.altEnabled {
			m.mapAlternateCPU()
		}
	}
}

func (m *Mapper) writePatternRegisters(address uint16, value byte) {
	switch address {
	case regCHRMode:
		m.chrMode = value & modeMask

	case regCHRSelect:
		m.MarkFeature(feature.CHRSourceSelect)
		m.chrSelect = value & chrSelectMask
		m.chrRAM[m.chrSelect] = value&bankSourceRAM != 0

	case regCHRBankHigh, regCHRBankLow:
		m.MarkFeature(feature.CHRBanking)
		slot := int(m.chrSelect)
		setBankByte(&m.chrBanks[slot], address == regCHRBankHigh, value)
		w := window{
			data:   m.chrROM,
			offset: int(m.chrBanks[slot]) * chrBankSize,
		}
		if m.chrRAM[slot] {
			w.data, w.writable = m.ram, true
		}
		m.ppuWindows[slot] = w
	}
}

func (m *Mapper) writeIRQRegisters(address uint16, value byte) {
	switch address {
	case regIRQEnable:
		m.MarkFeature(feature.ScanlineIRQ)
		m.irqEnabled = value&controlEnable != 0

	case regIRQTarget:
		m.irqTarget = value

	case regIRQOffset:
		m.irqOffset = value // Stored only. The IRQ comparator does not use it.
	}
}

func (m *Mapper) writeDescriptorRegisters(address uint16, value byte) {
	switch address {
	case regDescriptorMode:
		m.descriptorEnabled, m.attributeMode = value&extendedModeEnable != 0, value&modeMask
		if m.descriptorEnabled {
			m.MarkFeature(feature.ExtendedAttributes)
			m.MarkFeature(feature.BGExtendedMode)
		}

	case regDescriptorSelect:
		m.descriptorSelect = value & ntSelectMask
		m.descriptorRAM[m.descriptorSelect] = value&bankSourceRAM != 0
		m.patternRAM[m.descriptorSelect] = value&descriptorPatternRAM != 0

	case regDescriptorHigh, regDescriptorLow:
		setBankByte(&m.descriptorBanks[m.descriptorSelect], address == regDescriptorHigh, value)

	case regManualScroll:
		m.manualScroll = value

	case regVerticalPosition:
		m.vertical = value
	}
}

func (m *Mapper) writeNametableRegisters(address uint16, value byte) {
	switch address {
	case regNametableMode:
		m.ntEnabled, m.ntMode = value&extendedModeEnable != 0, value&modeMask
		if !m.ntEnabled {
			// Empty windows fall back to the PPU's ordinary mirrored nametables.
			m.ntWindows = [ntWindowCount]window{}
		}

	case regNametableSelect:
		m.ntSelect = value & ntSelectMask
		m.ntRAM[m.ntSelect] = value&bankSourceRAM != 0

	case regNametableHigh, regNametableLow:
		m.MarkFeature(feature.NameTableBanking)
		slot := int(m.ntSelect)
		setBankByte(&m.ntBanks[slot], address == regNametableHigh, value)
		if m.ntEnabled {
			w := window{
				data:   m.chrROM,
				offset: int(m.ntBanks[slot]) * ntBankSize,
			}
			if m.ntRAM[slot] {
				w.data, w.writable = m.ram, true
			}
			m.ntWindows[slot] = w
		}
	}
}

func (m *Mapper) writePlaneRegisters(address uint16, value byte) {
	switch address {
	case regPlaneMode:
		// Background mode widens a descriptor bank to two 4 KiB halves.
		// Sprite mode makes fallback reads use their original PPU address.
		m.wide, m.spriteMode = value&planeBackgroundBit != 0, value&planeSpriteBit != 0

	case regPlaneSelect:
		// The background bit picks the other half and palette pair. The
		// sprite bit independently adds 4 KiB to fallback pattern reads.
		m.second, m.spriteSecond = value&planeBackgroundBit != 0, value&planeSpriteBit != 0

	case regReadInhibit:
		m.inhibit = value

	case regWriteBankHigh, regWriteBankLow:
		setBankByte(&m.writeBank, address == regWriteBankHigh, value)
	}
}

func (m *Mapper) writeCopyRegisters(address uint16, value byte) {
	switch address {
	case regCopySource:
		m.copySource = value & modeMask

	case regCopyDestination:
		m.copyDestination = value & modeMask // Destination remains WorkRam in this build.

	case regCopyFromLow, regCopyFromHigh:
		// The copy unit's byte order differs from PRG/CHR bank registers.
		setBankByte(&m.copyFrom, address == regCopyFromHigh, value)

	case regCopyToLow, regCopyToHigh:
		setBankByte(&m.copyTo, address == regCopyToHigh, value)

	case regCopyCount:
		m.copyBlocks(value)
	}
}

func (m *Mapper) writeMetatileRegisters(address uint16, value byte) {
	switch address {
	case regMetatileMode:
		m.metatileEnabled = value&controlEnable != 0
		if m.metatileEnabled {
			m.MarkFeature(feature.BGExtendedMode)
		}

	case regMetatileTable0, regMetatileTable1, regMetatileTable2, regMetatileTable3:
		m.metatileTables[address-regMetatileTable0] = value

	case regMetatileStride:
		m.metatileStride = metatileDefaultStride << (value & modeMask)

	case regMetatileFrame:
		m.metatileFrame = value
	}
}

func (m *Mapper) writeRemapRegisters(address uint16, value byte) {
	switch address {
	case regRemapMode:
		// Enabling/disabling remap does not reset its upload cursor; only
		// a READ of this same address does so (see readRegister).
		m.remapEnabled = value&controlEnable != 0

	case regRemapData:
		// Ignore writes after the end of the table. The demo uploads the
		// table linearly from the start and does not wrap at page ends.
		if int(m.remapCursor) < len(m.remapTable) {
			m.remapTable[m.remapCursor] = value
		}
		m.remapCursor++

	case regRemapPage:
		m.remapPage = value & remapPageMask
	}
}

func (m *Mapper) readRegister(address uint16) byte {
	if address >= regIdentityStart && address <= regIdentityEnd {
		return identityResponse[address-regIdentityStart]
	}

	if address >= regPRGReadback && address <= regPRGReadbackEnd {
		// Decode the readback record, not the write-register meanings at
		// these same addresses. The first byte includes the window index.
		slot, part := (address-regPRGReadback)/prgReadbackStride, (address-regPRGReadback)%prgReadbackStride

		switch part {
		case 0:
			return byte(slot) | m.prgFlags[slot]

		case 1:
			return byte(m.prgBanks[slot] >> 8)

		default:
			return byte(m.prgBanks[slot])
		}
	}

	switch address {
	case regStateData:
		cursor := m.streamCursor
		m.streamCursor++
		return m.streamValue(cursor)

	case regStateCursor:
		return m.streamCursor

	case regPRGSelection:
		return m.prgSelect | m.prgFlags[m.prgSelect]

	case regIRQAcknowledge:
		// A status read is also the acknowledgement. Inspection tools avoid
		// register reads so displaying state cannot accidentally clear IRQ.
		m.irqPending = false
		m.SetMapperIRQ(false)
		return m.irqPhase

	case regGraphicsLatch:
		return m.readLatch

	case regRemapMode:
		m.remapCursor = 0
		return boolByte(m.remapEnabled)

	case regScanline:
		return m.scanline
	}

	return m.OpenBus()
}

// copyBlocks copies 8 KiB blocks immediately. The mapper keeps the destination
// selector, but it has no effect: all copies go to WorkRam. The copy uses no
// emulated CPU cycles.
func (m *Mapper) copyBlocks(count byte) {
	m.MarkFeature(feature.PRGRAM)

	source := m.prgROM
	switch m.copySource {
	case copySourceRAM, copySourceRAMAlias:
		source = m.ram

	case copySourceCHR:
		source = m.chrROM
	}

	from, to, size := int(m.copyFrom)*prgBankSize, int(m.copyTo)*prgBankSize, int(count)*prgBankSize
	if from+size > len(source) || to+size > len(m.ram) {
		return
	}

	// A forward byte loop is intentional: overlapping RAM copies propagate data.
	for i := range size {
		m.ram[to+i] = source[from+i]
	}
}

// streamValue implements the sparse $5006 snapshot layout. A position is not a
// CPU register address; unsupported positions return $FF rather than open bus.
func (m *Mapper) streamValue(cursor byte) byte {
	switch cursor {
	case statePRGMode:
		return m.prgMode

	case statePRGSelect:
		return m.prgSelect

	case statePRGBankHigh:
		return byte(m.prgBanks[0] >> 8)

	case statePRGBankLow:
		return byte(m.prgBanks[0])

	case stateDescriptor:
		return m.descriptorSelect

	case stateDescriptorHigh:
		return byte(m.descriptorBanks[0] >> 8)

	case stateDescriptorLow:
		return byte(m.descriptorBanks[0])

	case stateWide:
		return boolByte(m.wide)

	case stateSpriteMode:
		return boolByte(m.spriteMode)

	case stateSecondPlane:
		return boolByte(m.second)

	case stateSpritePlane:
		return boolByte(m.spriteSecond) * planeSpriteBit

	default:
		return stateUnavailable
	}
}

func setBankByte(bank *uint16, high bool, value byte) {
	if high {
		*bank = *bank&byteMask | uint16(value)<<8
	} else {
		*bank = *bank&bankHighMask | uint16(value)
	}
}

func boolByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}
