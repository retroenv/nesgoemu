package mxm

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
)

// metatileTableBases are the WorkRam offsets of the nine metatile tables. A
// table selector selects one of them.
var metatileTableBases = [...]int{
	metatileTable0,
	metatileTable1,
	metatileExtraTables,
	metatileExtraTables + metatileTableSize,
	metatileExtraTables + 2*metatileTableSize,
	metatileExtraTables + 3*metatileTableSize,
	metatileExtraTables + 4*metatileTableSize,
	metatileExtraTables + 5*metatileTableSize,
	metatileExtraTables + 6*metatileTableSize,
}

// observeFetch runs before data substitution, also when reads are inhibited.
// Only tile-name reads increment nameReads. Attribute and pattern reads update
// the repeated-address detector.
func (m *Mapper) observeFetch(address uint16) bool {
	name := address >= nameTableStart && address < nameTableFetchEnd && address&ntOffsetMask < attributeTableOffset
	if name {
		m.nameReads++
		if !m.inFrame && m.frameStart {
			m.frameStart, m.inFrame = false, true
		}
	}

	m.trackRepeated(address)
	m.lastAddress = address

	if m.nameReads == backgroundFetchCount && m.previousNameReads != backgroundFetchCount {
		// Advance once, when the count reaches the checkpoint. Do not advance
		// on the attribute and pattern reads that follow, while the name count
		// does not change.
		m.vertical++
	}

	if m.manualScroll == 0 && m.scrollDirty {
		m.MarkFeature(feature.PPURegisterCapture)
		m.vertical = m.ppuRegisters[ppuScrollIndex]
		m.scrollDirty = false
	}

	m.previousNameReads = byte(m.nameReads)
	return name
}

// trackRepeated finds three identical consecutive nametable-space reads. The
// two reads at the end of a scanline and the first tile read after them make
// this sequence. The detector does not use a PPU scanline number.
func (m *Mapper) trackRepeated(address uint16) {
	if address < patternTableEnd || address >= nameTableFetchEnd || address != m.lastAddress {
		m.repeated = 0
		return
	}

	m.repeated++
	if m.repeated <= 1 {
		return
	}

	m.MarkFeature(feature.PPUBusTiming)
	m.nameReads = 0

	if !m.inFrame && !m.frameStart {
		m.frameStart, m.scanline = true, 0
		return
	}

	m.scanline++
	if m.scanline == m.irqTarget {
		m.irqPending = true
		if m.irqEnabled {
			m.SetMapperIRQ(true)
		}
	}
}

func (m *Mapper) readVRAM(address uint16) byte {
	if m.dummyAttribute && address >= nameTableStart && address < nameTableFetchEnd {
		address = attributeTableBase | address&nameTableSelectMask | address>>4&attributeRowMask | address>>2&fineYMask
	}

	name := m.observeFetch(address)

	y, mapped := m.vertical, address
	if m.remapEnabled {
		// Remap coarse Y in the bus address. Keep fine Y separate for
		// descriptors and metatiles. Sprite-mode fallback can keep the
		// original address, so the code keeps both address and mapped.
		y = m.remapTable[int(m.remapPage)*remapPageSize+int(y)]
		mapped = address&verticalRemapKeepMask | uint16(y>>3)<<tileRowShift
	}

	if m.inhibit != 0 {
		return 0
	}

	eligible := (m.nameReads+fetchPhaseOffset)%fetchPhaseCount < backgroundFetchCount
	if eligible {
		if value, handled := m.readBackground(address, mapped, y, name); handled {
			return value
		}
	}

	if name || !m.spriteMode {
		return m.mappedVRAM(mapped)
	}
	if !m.spriteSecond {
		return m.mappedVRAM(address)
	}

	slot := int(address/chrBankSize) & chrSelectMask
	return readWrapped(m.patternMemory(m.chrRAM[slot]), int(m.chrBanks[slot])*chrBankSize+patternPageSize+int(address&chrOffsetMask), 0)
}

// readBackground gives metatiles priority over descriptors. A name read latches
// the tile context. The next three reads give the attribute byte and two plane
// bytes. A false result lets readVRAM use the ordinary or sprite fallback
// mapping.
func (m *Mapper) readBackground(address, mapped uint16, y byte, name bool) (byte, bool) {
	if m.metatileEnabled {
		if name {
			m.fetchMetatile(mapped, y&fineYMask)
			return 0, true
		}
		if m.metatilePhase != 0 {
			m.metatilePhase--
			return [postNameFetchCount]byte{m.metatileHigh, m.metatileLow, m.metatilePalette}[m.metatilePhase], true
		}
	}

	if m.descriptorEnabled {
		return m.readDescriptor(address, mapped, y, name)
	}
	return 0, false
}

func (m *Mapper) readDescriptor(address, mapped uint16, y byte, name bool) (byte, bool) {
	if name {
		// Capture the nametable, tile offset, and fine Y once. Later PPU
		// pattern addresses do not contain enough information to find them
		// again.
		m.descriptorPhase = postNameFetchCount
		m.tileTable, m.tileOffset, m.tileFineY = byte(mapped/ntBankSize)&ntSelectMask, mapped&ntOffsetMask, y&fineYMask

		if m.ntEnabled && m.ntRAM[m.tileTable] {
			return readWrapped(m.ram, int(m.ntBanks[m.tileTable])*ntBankSize+int(m.tileOffset), 0), true
		}
		return m.mappedVRAM(mapped), true
	}

	if m.descriptorPhase == 0 {
		return 0, false
	}

	m.descriptorPhase--
	if m.descriptorPhase == 2 && m.attributeMode != attributeModePatternOnly {
		return m.fetchAttribute(y), true
	}
	return m.fetchPattern(address), true
}

func (m *Mapper) patternMemory(ram bool) []byte {
	if ram {
		return m.ram
	}
	return m.chrROM
}

func (m *Mapper) descriptorByte(offset int) byte {
	return readWrapped(m.patternMemory(m.descriptorRAM[m.tileTable]), offset, 0)
}

func (m *Mapper) fetchAttribute(y byte) byte {
	// "8x1 attributes" change the palette choice on each pixel row of an 8x8
	// tile. The tile size stays 8x8. The author of MXM explains this here:
	// https://forums.nesdev.org/viewtopic.php?p=276887
	base := int(m.descriptorBanks[m.tileTable])*descriptorBankSize + int(m.tileOffset)*descriptorSize
	var palette byte

	switch m.attributeMode {
	case attributeModeTile, attributeModeFineY:
		if m.attributeMode == attributeModeFineY {
			base += int(m.tileFineY) * descriptorBankSize
		}
		value := m.descriptorByte(base)
		palette = value >> 6
		if m.wide && m.second {
			palette = value >> 4 & paletteMask
		}

	case attributeModePackedRows:
		// Two extra bytes store eight row palettes, with four two-bit values
		// in each byte. Select the byte with Y/4. Then extract the palette,
		// from the high bits to the low bits.
		value := m.descriptorByte(base + descriptorBankSize + int((y&fineYMask)>>2))
		palette = value >> uint(6-2*(y&paletteMask)) & paletteMask
	}

	// Put the chosen palette in every quadrant of the attribute byte. Then
	// the ordinary PPU attribute shifter selects it for this tile.
	return palette * paletteRepeat
}

func (m *Mapper) fetchPattern(address uint16) byte {
	base := int(m.descriptorBanks[m.tileTable])*descriptorBankSize + int(m.tileOffset)*descriptorSize
	descriptor := uint16(m.descriptorByte(base))<<8 | uint16(m.descriptorByte(base+1))

	if m.remapEnabled {
		address = address&patternFineYKeepMask | uint16(m.tileFineY)
	}

	// The descriptor gives the large bank number. The PPU gives the tile and
	// row offset in that bank. Wide mode doubles the bank stride. The
	// second-plane selector then selects the lower or upper 4 KiB half.
	offset := int(descriptor&descriptorBankMask) * patternPageSize
	if m.wide {
		offset *= 2
		if m.second {
			offset += patternPageSize
		}
	}

	return readWrapped(m.patternMemory(m.patternRAM[m.tileTable]), offset+int(address&patternPageMask), 0)
}

func (m *Mapper) fetchMetatile(address uint16, fine byte) {
	selector := m.metatileTables[address/ntBankSize&ntSelectMask]
	base := metatileFallbackTable
	if int(selector) < len(metatileTableBases) {
		base = metatileTableBases[selector]
	}

	// Each 32-column tile row contains eight fine-Y rows of big-endian IDs.
	// A table selector after the nine known tables selects a fixed fallback
	// table. The selector does not wrap.
	cell := descriptorSize * ((int(address&ntOffsetMask)>>tileRowShift)*tileHeight*tilesPerRow + int(fine)*tilesPerRow + int(address&tileColumnMask))
	id := uint16(m.ramByte(base+cell))<<8 | uint16(m.ramByte(base+cell+1))
	if id&metatileAnimationMask == metatileAnimationMask {
		// Resolve exactly one animation indirection. The replacement is a
		// complete 16-bit final ID. Do not interpret it again.
		m.MarkFeature(feature.MetatileAnimation)
		index := metatileAnimationTable + descriptorSize*(int(m.metatileStride)*int(id&metatileIndexMask)+int(m.metatileFrame))
		id = uint16(m.ramByte(index))<<8 | uint16(m.ramByte(index+1))
	}

	// Each ID owns two pairs of low and high planes. A palette byte holds two
	// IDs: an even ID uses the upper nibble and an odd ID uses the lower
	// nibble. The plane set selects the two-bit palette in that nibble.
	data, packed := metatilePlaneTable+int(id)*metatileRecordSize, m.ramByte(metatilePaletteTable+int(id>>1))
	shift := uint(6)

	if id&1 != 0 {
		shift = 2
	}
	if m.wide && m.second {
		data += 2
		shift -= 2
	}

	m.metatileLow, m.metatileHigh = m.ramByte(data), m.ramByte(data+1)
	m.metatilePalette, m.metatilePhase = (packed>>shift&paletteMask)*paletteRepeat, postNameFetchCount
}

func (m *Mapper) ramByte(offset int) byte {
	if offset >= len(m.ram) {
		return 0
	}
	return m.ram[offset]
}

func (m *Mapper) mappedVRAM(address uint16) byte {
	if address < patternTableEnd {
		w := m.ppuWindows[address/chrBankSize]
		return readWrapped(w.data, w.offset+int(address&chrOffsetMask), byte(address))
	}

	if address < paletteStart {
		w := m.ntWindows[address/ntBankSize&ntSelectMask]
		if w.data != nil {
			return readWrapped(w.data, w.offset+int(address&ntOffsetMask), byte(address))
		}
		if inspector, ok := m.NameTableMemory().(bus.NameTableInspector); ok {
			return inspector.InspectNameTable(address)
		}
	}

	return byte(address)
}

func (m *Mapper) writeVRAM(address uint16, value byte) {
	if m.ntEnabled {
		// In extended-NT mode, PPUDATA uploads ignore the CHR and NT read
		// mappings. The raw PPU address is the low 16 bits of the WorkRam
		// destination. Registers $5092 and $5093 give the high segment number.
		offset := int(m.writeBank)<<writeBankShift | int(address)
		if offset < len(m.ram) {
			m.MarkFeature(feature.PPUWriteBanking)
			m.ram[offset] = value
		}
		return
	}

	if address < patternTableEnd {
		w := m.ppuWindows[address/chrBankSize]
		if w.writable {
			writeWrapped(w.data, w.offset+int(address&chrOffsetMask), value)
		}
		return
	}

	// Write the default four-screen and mirroring data directly. Do not use
	// the nametable hook of this mapper.
	slot := (address - nameTableStart) / ntBankSize % ntWindowCount
	m.NameTableMemory().Data()[slot][address&ntOffsetMask] = value
}
