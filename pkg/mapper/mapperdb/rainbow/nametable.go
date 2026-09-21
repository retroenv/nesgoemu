package rainbow

import "github.com/retroenv/nesgoemu/pkg/feature"

// Nametable specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#nametables-control-412a-412d-412f-write-only
// Window specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#window-split-mode-4170-4175

const (
	ntSourceCIRAM   = 0 // standard NES CIRAM
	ntSourceCHRRAM  = 1 // The installed CHR-RAM size limits the bank bits.
	ntSourceFPGARAM = 2 // FPGA-RAM, bank[1:0] selects 1 KB page
	ntSourceCHRROM  = 3 // CHR-ROM, bank[7:0] selects 1 KB page

	ntBaseAddress    = 0x2000 // start of PPU nametable space
	ntEndAddress     = 0x3F00 // start of PPU palette space
	ntMirrorSize     = 0x1000
	ntSlotSize       = 0x400 // 1 KB per nametable slot
	ntSlotShift      = 10
	ntAttrOffset     = 0x3C0 // attribute table starts at offset $3C0 within a slot
	ciramAddressMask = 0x7FF
	ciramBankMask    = 0x01

	ntAttrExtBit         = 0x01 // ntControl bit 0: attribute extended mode
	ntBGExtBit           = 0x02 // ntControl bit 1: background extended mode
	ntExtPageMask        = 0x03 // ntControl bits [3:2]: FPGA-RAM page
	ntExtPageShift       = 2
	ntFillBit            = 0x20 // ntControl bit 5: fill mode
	ntSrcShift           = 6    // ntControl bits [7:6]: source select
	ntSrcMask            = 0xC0
	ntPaletteMask        = 0x03
	ntAttributeBlockSize = 4
	ntWindowSource       = ntSourceFPGARAM << ntSrcShift

	ntSlotCount  = 5  // The array has four screen slots and one window slot.
	ntWindowSlot = 4  // The window nametable uses this ntBank and ntControl index.
	ntTileCols   = 32 // tile columns per row in a nametable
	ntTileRows   = 30 // visible tile rows per nametable

	ppuTileSize           = 8
	ppuVisibleHeight      = ntTileRows * ppuTileSize
	ppuVisibleFetchStart  = 1
	ppuVisibleFetchEnd    = 256
	ppuPrefetchStart      = 321
	ppuPrefetchEnd        = 336
	ppuScanlineCount      = 262
	ppuWindowColumnOffset = 2

	windowSplitXStartIndex   = 0
	windowSplitXEndIndex     = 1
	windowSplitYStartIndex   = 2
	windowSplitYEndIndex     = 3
	windowSplitXScrollIndex  = 4
	windowSplitYScrollIndex  = 5
	windowSplitRegisterCount = 6
)

// readNametable is the PPU read hook for nametable addresses $2000–$3EFF.
// It returns (value, true) if the Rainbow mapper handles this read;
// (0, false) to fall through to the default CIRAM read.
func (m *Mapper) readNametable(address uint16) (uint8, bool) {
	if address < ntBaseAddress {
		return 0, false
	}

	m.observePPURead(address)

	// Normalize $3000–$3EFF mirrors down to $2000–$2EFF.
	normalized := ntBaseAddress + (address-ntBaseAddress)%ntMirrorSize
	offset := normalized - ntBaseAddress
	slot := int(offset >> ntSlotShift)      // 0–3
	slotOffset := offset & (ntSlotSize - 1) // 0x000–0x3FF

	if m.windowEnabled && (!m.ppuBus.enabled || m.scanIRQ.inFrame) {
		if v, ok := m.readWindowNametable(slotOffset); ok {
			return v, true
		}
	}

	return m.readSlot(slot, slotOffset)
}

// writeNametable writes to the selected memory. CHR-ROM writes are ignored.
func (m *Mapper) writeNametable(address uint16, value byte) bool {
	if address < ntBaseAddress || address >= ntEndAddress {
		return false
	}
	offset := (address - ntBaseAddress) % ntMirrorSize
	slot := offset / ntSlotSize
	bank := m.ntBank[slot]
	offset %= ntSlotSize
	switch m.ntControl[slot] >> ntSrcShift {
	case ntSourceCIRAM:
		m.NameTableMemory().WriteCIRAM(uint16(bank&ciramBankMask)*ntSlotSize+offset, value)
	case ntSourceCHRRAM:
		m.writeToRAM(m.chrRAM, int(bank)*ntSlotSize+int(offset), value)
	case ntSourceFPGARAM:
		m.fpgaRAM[uint16(bank&ntExtPageMask)*ntSlotSize+offset] = value
	}
	return true
}

// readAttrExtMode handles attribute extended mode (ntControl bit 0).
// When enabled, the ext data byte for the tile (cached in bgTileSlotOffset) provides
// a per-tile 2-bit palette that replaces the normal grouped attribute.
// Returns (0, false) when attribute ext mode is disabled or the slot doesn't match.
func (m *Mapper) readAttrExtMode(bankIdx int, ctrl byte) (uint8, bool) {
	if ctrl&ntAttrExtBit == 0 || m.bgTileSlotIdx != bankIdx {
		return 0, false
	}

	fpgaSrcPage := (ctrl >> ntExtPageShift) & ntExtPageMask
	extOffset := uint32(fpgaSrcPage)*ntSlotSize + uint32(m.bgTileSlotOffset)
	if extOffset >= fpgaRAMSize {
		return 0, false
	}

	extData := m.fpgaRAM[extOffset]
	palette := (extData >> ntSrcShift) & ntPaletteMask
	m.MarkFeature(feature.ExtendedAttributes)
	return replicateAttr(palette), true
}

// readSlot reads from nametable slot bankIdx, applying fill mode and source routing
// from ntBank[bankIdx]/ntControl[bankIdx].
// Each slot selects its own memory source and bank.
func (m *Mapper) readSlot(bankIdx int, slotOffset uint16) (uint8, bool) {
	ctrl := m.ntControl[bankIdx]
	if m.ppuBus.enabled && !m.scanIRQ.inFrame {
		ctrl &= ntSrcMask
	}
	bank := m.ntBank[bankIdx]

	if slotOffset < ntAttrOffset {
		m.updateBGExtState(bankIdx, slotOffset, ctrl)
	}

	if ctrl&ntFillBit != 0 {
		m.MarkFeature(feature.NameTableFill)

		if slotOffset >= ntAttrOffset {
			return replicateAttr(m.fillAttr & ntPaletteMask), true
		}
		return m.fillTile, true
	}

	if slotOffset >= ntAttrOffset {
		if v, ok := m.readAttrExtMode(bankIdx, ctrl); ok {
			return v, true
		}
	}

	switch ctrl >> ntSrcShift {
	case ntSourceCIRAM:
		return m.NameTableMemory().ReadCIRAM(uint16(bank&ciramBankMask)*ntSlotSize + slotOffset), true

	case ntSourceCHRRAM:
		if len(m.chrRAM) == 0 {
			return 0, true
		}
		addr := (int(bank)*ntSlotSize + int(slotOffset)) % len(m.chrRAM)
		return m.chrRAM[addr], true

	case ntSourceFPGARAM:
		addr := uint32(bank&ntExtPageMask)*ntSlotSize + uint32(slotOffset)
		if addr >= fpgaRAMSize {
			return 0, true
		}
		return m.fpgaRAM[addr], true

	case ntSourceCHRROM:
		addr := uint32(bank)*ntSlotSize + uint32(slotOffset)
		if int(addr) >= len(m.chrROM) {
			return 0, true
		}
		return m.chrROM[addr], true
	}

	return 0, false
}

// updateBGExtState caches the current tile's slot and offset, then reads the ext
// data byte from FPGA-RAM when BG extended mode (ntControl bit 1) is active.
// Called for every tile-index fetch (slotOffset < ntAttrOffset).
func (m *Mapper) updateBGExtState(bankIdx int, slotOffset uint16, ctrl byte) {
	m.bgTileSlotIdx = bankIdx
	m.bgTileSlotOffset = slotOffset
	m.bgExtActive = ctrl&ntBGExtBit != 0

	if !m.bgExtActive {
		return
	}
	m.MarkFeature(feature.BGExtendedMode)

	fpgaSrcPage := (ctrl >> ntExtPageShift) & ntExtPageMask
	extOffset := uint32(fpgaSrcPage)*ntSlotSize + uint32(slotOffset)
	if extOffset < fpgaRAMSize {
		m.bgExtData = m.fpgaRAM[extOffset]
	} else {
		m.bgExtData = 0
	}
}

// readWindowNametable uses the screen position of the tile being fetched.
func (m *Mapper) readWindowNametable(slotOffset uint16) (uint8, bool) {
	column, line, inside := m.windowPosition()
	if !inside {
		return 0, false
	}
	tileCol := (column + int(m.windowSplitRegs[windowSplitXScrollIndex])) & (ntTileCols - 1)
	pixelY := (line + int(m.windowSplitRegs[windowSplitYScrollIndex])) % ppuVisibleHeight
	tileRow := pixelY / ppuTileSize
	ntOffset := uint16(tileRow*ntTileCols + tileCol)
	if slotOffset >= ntAttrOffset {
		ntOffset = ntAttrOffset + uint16(tileRow/ntAttributeBlockSize*ppuTileSize+
			tileCol/ntAttributeBlockSize)
	}
	value, _ := m.readSlot(ntWindowSlot, ntOffset)
	if slotOffset >= ntAttrOffset && m.ntControl[ntWindowSlot]&(ntFillBit|ntAttrExtBit) == 0 {
		shift := (tileRow&2)*2 + (tileCol & 2)
		value = replicateAttr((value >> shift) & ntPaletteMask)
	}
	return value, true
}

func (m *Mapper) windowPosition() (int, int, bool) {
	line := m.ppuScanLine
	column := -1
	switch {
	case m.ppuCycle >= ppuPrefetchStart && m.ppuCycle <= ppuPrefetchEnd:
		column = (m.ppuCycle - ppuPrefetchStart) / ppuTileSize
		line++
		if line == ppuScanlineCount {
			line = 0
		}
	case m.ppuCycle >= ppuVisibleFetchStart && m.ppuCycle <= ppuVisibleFetchEnd:
		column = (m.ppuCycle-ppuVisibleFetchStart)/ppuTileSize + ppuWindowColumnOffset
	}
	if m.ppuBus.enabled {
		if !m.scanIRQ.inFrame {
			return 0, 0, false
		}
		// Counts 49 and 50 fetch the next line's columns 0 and 1. The first
		// 48 reads comprise 32 background tiles and 16 unused sprite reads.
		// Modulo 50 aligns these prefetches with the window's screen coordinates.
		column = (m.ppuBus.tiles + 1) % ppuBusWindowFetchCount
		line = int(m.scanIRQ.counter)
		if m.ppuBus.tiles >= ppuBusLastCurrentLineFetch {
			line++
		}
	}
	inside := column >= 0 && line >= 0 && line < ppuVisibleHeight &&
		windowInRange(column, int(m.windowSplitRegs[windowSplitXStartIndex]), int(m.windowSplitRegs[windowSplitXEndIndex])) &&
		windowInRange(line, int(m.windowSplitRegs[windowSplitYStartIndex]), int(m.windowSplitRegs[windowSplitYEndIndex]))
	return column, line, inside
}

// replicateAttr fills all four 2-bit positions of an attribute byte with the
// same 2-bit palette value (e.g., 0x02 → 0xAA).
func replicateAttr(v byte) byte {
	return v | v<<2 | v<<4 | v<<6
}

// windowInRange includes both endpoints. Equal bounds select one coordinate.
// Reversed bounds select the two outer regions. This applies to both axes.
func windowInRange(val, lo, hi int) bool {
	if lo <= hi {
		return val >= lo && val <= hi
	}
	return val >= lo || val <= hi
}
