package mxm1

import "github.com/retroenv/nesgoemu/pkg/feature"

func (m *Mapper) writeRegister(address uint16, value byte) {
	if m.isMapper801 && m.writeBank801(address, value) {
		return
	}

	switch address & registerGroupMask {
	case regStateCursor & registerGroupMask:
		if address == regStateCursor {
			m.streamCursor = value
		}

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

func (m *Mapper) writeCPURegisters(address uint16, value byte) {
	switch address {
	case regPRGMode:
		m.prgMode = value & modeMask

	case regPRGSelect:
		// The register file changes at once. Only a write to a bank byte
		// rebuilds the memory window.
		m.prgSelect = value & prgSelectMask
		m.prgFlags[m.prgSelect] = value & prgFlagsMask

	case regPRGBankHigh, regPRGBankLow:
		m.writePRGBank(address == regPRGBankHigh, value)

	case regAlternateMode:
		if enabled := value&controlEnable != 0; enabled != m.altEnabled {
			if enabled {
				m.MarkFeature(feature.AlternateBank)
			}

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

func (m *Mapper) writePRGBank(high bool, value byte) {
	m.MarkFeature(feature.PRGBanking)
	if m.prgSelect == 0 {
		m.MarkFeature(feature.LowBankMapping)
	}

	setBankByte(&m.prgBanks[m.prgSelect], high, value)
	m.mapSelectedCPU()
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
		m.writeCHRBank(m.chrSelect, address == regCHRBankHigh, value, m.chrRAM[m.chrSelect])
	}
}

func (m *Mapper) writeCHRBank(slot byte, high bool, value byte, ram bool) {
	m.MarkFeature(feature.CHRBanking)
	setBankByte(&m.chrBanks[slot], high, value)
	w := window{
		data:   m.chrROM,
		offset: int(m.chrBanks[slot]) * chrBankSize,
	}
	if ram {
		w.data = m.ram
		w.writable = true
	}

	m.ppuWindows[slot] = w
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
	m.descriptor.write(address, value)
	switch address {
	case regDescriptorMode:
		m.attributeMode = value & modeMask
		m.markGraphicsModes()

	case regDescriptorSelect:
		m.descriptorRAM[m.descriptor.selected] = value&bankSourceRAM != 0
		m.patternRAM[m.descriptor.selected] = value&descriptorPatternRAM != 0

	case regManualScroll:
		m.manualScroll = value

	case regVerticalPosition:
		m.verticalPosition = value
	}
}

func (m *Mapper) writeNametableRegisters(address uint16, value byte) {
	m.nametable.write(address, value)
	switch address {
	case regNametableMode:
		m.ntMode = value & modeMask
		if !m.nametable.enabled {
			// Empty windows use the ordinary mirrored nametables of the PPU.
			m.ntWindows = [ntWindowCount]window{}
		}

	case regNametableSelect:
		m.ntRAM[m.nametable.selected] = value&bankSourceRAM != 0

	case regNametableHigh, regNametableLow:
		m.MarkFeature(feature.NameTableBanking)
		slot := int(m.nametable.selected)
		if m.nametable.enabled {
			w := window{
				data:   m.chrROM,
				offset: int(m.nametable.banks[slot]) * ntBankSize,
			}
			if m.ntRAM[slot] {
				w.data = m.ram
				w.writable = true
			}

			m.ntWindows[slot] = w
		}
	}
}

func (m *Mapper) writePlaneRegisters(address uint16, value byte) {
	switch address {
	case regPlaneMode:
		// Background mode makes a descriptor bank two 4 KiB halves wide.
		// Sprite mode makes fallback reads use the original PPU address.
		m.wideBanks = value&planeBackgroundBit != 0
		m.spriteMode = value&planeSpriteBit != 0
		m.markGraphicsModes()

	case regPlaneSelect:
		// The background bit selects the other half and palette pair. The
		// sprite bit adds 4 KiB to fallback pattern reads. The bits are
		// independent.
		m.secondPlane = value&planeBackgroundBit != 0
		m.spriteSecondPlane = value&planeSpriteBit != 0

	case regReadInhibit:
		m.readInhibit = value

	case regWriteBankHigh, regWriteBankLow:
		setBankByte(&m.writeBank, address == regWriteBankHigh, value)
	}
}

func (m *Mapper) writeCopyRegisters(address uint16, value byte) {
	switch address {
	case regCopySource:
		m.copySource = value & modeMask

	case regCopyDestination:
		m.copyDestination = value & modeMask // The destination stays WorkRam in this build.

	case regCopyFromLow, regCopyFromHigh:
		// The byte order of the copy unit is different from the PRG and CHR
		// bank registers.
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
		m.markGraphicsModes()

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
		// Enable and disable do not reset the upload cursor. Only a read of
		// this address resets it (see readRegister).
		m.remapEnabled = value&controlEnable != 0
		m.markGraphicsModes()

	case regRemapData:
		// Ignore writes after the end of the table. The demo uploads the
		// table in order from the start. The cursor does not wrap at page
		// ends.
		if int(m.remapCursor) < len(m.remapTable) {
			m.remapTable[m.remapCursor] = value
		}

		m.remapCursor++

	case regRemapPage:
		m.remapPage = value & remapPageMask
		if m.isMapper801 {
			m.remapPage &= controlEnable
		}

	case reg801RemapOffset:
		if m.isMapper801 {
			m.remapOffset801 = value
		}

	case reg801RemapNTFlip:
		if m.isMapper801 {
			m.remapNTFlip801 = value&controlEnable != 0
		}

	case reg801RemapPosition, reg801RemapPositionAlias:
		if m.isMapper801 {
			// These ports set the table index. PPUSCROLL sets the ordinary row.
			m.remapPosition801 = value
		}
	}
}

// markGraphicsModes marks the background and sprite modes that the register
// values enable. A metatile ID selects the palette for each pixel row, so
// metatile mode also gives 8x1 attributes. In attribute mode 3, the attribute
// fetch reads pattern data, so this mode gives no attributes.
func (m *Mapper) markGraphicsModes() {
	descriptorAttributes := m.descriptor.enabled && m.attributeMode != attributeModePatternOnly
	rowAttributes := m.descriptor.enabled &&
		(m.attributeMode == attributeModeFineY || m.attributeMode == attributeModePackedRows)

	if m.descriptor.enabled || m.metatileEnabled {
		m.MarkFeature(feature.BGExtendedMode)
		if m.wideBanks {
			m.MarkFeature(feature.GraphicsSets)
		}
	}

	if descriptorAttributes || m.metatileEnabled {
		m.MarkFeature(feature.ExtendedAttributes)
	}

	if rowAttributes || m.metatileEnabled {
		m.MarkFeature(feature.RowAttributes)
	}

	if m.metatileEnabled {
		m.MarkFeature(feature.Metatiles)
	}

	if m.spriteMode {
		m.MarkFeature(feature.SpriteExtendedMode)
	}

	if m.remapEnabled {
		m.MarkFeature(feature.ScanlineRemap)
	}
}

func (m *Mapper) readRegister(address uint16) byte {
	readback := emulatorReadback
	if m.isMapper801 {
		readback = n8ProCompatibilityReadback
	}

	if value, ok := read5000(address, readback); ok {
		return value
	}

	if value, ok := m.readBankStatus(address); ok {
		return value
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
		// A status read also acknowledges the IRQ. Inspection tools do not
		// read registers, so they cannot clear the IRQ by accident.
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

func (m *Mapper) readBankStatus(address uint16) (byte, bool) {
	if m.isMapper801 {
		return m.readBankStatus801(address)
	}

	if address >= regPRGReadback && address <= regPRGReadbackEnd {
		return m.readPRGBankRecord(address), true
	}

	return 0, false
}

// readPRGBankRecord returns one byte of a PRG window's readback record.
func (m *Mapper) readPRGBankRecord(address uint16) byte {
	offset := address - regPRGReadback
	slot := offset / prgReadbackStride
	switch offset % prgReadbackStride {
	case 0:
		return byte(slot) | m.prgFlags[slot]

	case 1:
		return byte(m.prgBanks[slot] >> 8)

	default:
		return byte(m.prgBanks[slot])
	}
}

// copyBlocks copies 8 KiB blocks at once. The mapper keeps the destination
// selector, but the selector has no effect: all copies go to WorkRam. The copy
// uses no emulated CPU cycles.
func (m *Mapper) copyBlocks(count byte) {
	m.MarkFeature(feature.PRGRAM)

	source := m.prgROM
	switch m.copySource {
	case copySourceRAM, copySourceRAMAlias:
		source = m.ram

	case copySourceCHR:
		source = m.chrROM
	}

	from := int(m.copyFrom) * prgBankSize
	to := int(m.copyTo) * prgBankSize
	size := int(count) * prgBankSize
	if from+size > len(source) || to+size > len(m.ram) {
		return
	}

	m.MarkFeature(feature.BlockCopy)

	// The forward byte loop is necessary. When the ranges overlap, the copy
	// repeats the data.
	for i := range size {
		m.ram[to+i] = source[from+i]
	}
}

// streamValue implements the sparse $5006 snapshot layout. A position is not a
// CPU register address. An unsupported position returns $FF, not open bus.
func (m *Mapper) streamValue(cursor byte) byte {
	switch cursor {
	case statePRGMode:
		return m.prgMode

	case statePRGSelect:
		return m.prgSelect

	case statePRGBankHigh:
		bank := m.prgBanks[0]
		if m.isMapper801 {
			bank &= prgBankMask801
		}

		return byte(bank >> 8)

	case statePRGBankLow:
		return byte(m.prgBanks[0])

	case stateDescriptor:
		return m.descriptor.selected

	case stateDescriptorHigh:
		return byte(m.descriptor.banks[0] >> 8)

	case stateDescriptorLow:
		return byte(m.descriptor.banks[0])

	case stateWide:
		return boolByte(m.wideBanks)

	case stateSpriteMode:
		return boolByte(m.spriteMode)

	case stateSecondPlane:
		return boolByte(m.secondPlane)

	case stateSpritePlane:
		return boolByte(m.spriteSecondPlane) * planeSpriteBit

	default:
		return stateUnavailable
	}
}

func read5000(address uint16, readback string) (byte, bool) {
	offset := int(address) - regReadbackStart
	if offset < 0 || offset >= len(readback) {
		return 0, false
	}

	return readback[offset], true
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
