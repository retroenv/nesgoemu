package mxm1

import (
	"encoding/binary"
	"slices"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/renderstate"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

// graphicsModeFeatures are the features that markGraphicsModes marks.
var graphicsModeFeatures = []feature.ID{
	feature.BGExtendedMode,
	feature.ExtendedAttributes,
	feature.RowAttributes,
	feature.Metatiles,
	feature.GraphicsSets,
	feature.SpriteExtendedMode,
	feature.ScanlineRemap,
}

// graphicsModeTests enable each mode with register writes. Attribute modes 1
// and 2 and metatiles select a palette for each pixel row. Attribute mode 3
// gives no attributes.
var graphicsModeTests = []graphicsModeTest{
	{
		name:   "tile attributes",
		writes: []registerWrite{{regDescriptorMode, extendedModeEnable | attributeModeTile}},
		want:   []feature.ID{feature.BGExtendedMode, feature.ExtendedAttributes},
	},
	{
		name:   "fine-Y attributes",
		writes: []registerWrite{{regDescriptorMode, extendedModeEnable | attributeModeFineY}},
		want:   []feature.ID{feature.BGExtendedMode, feature.ExtendedAttributes, feature.RowAttributes},
	},
	{
		name:   "packed row attributes",
		writes: []registerWrite{{regDescriptorMode, extendedModeEnable | attributeModePackedRows}},
		want:   []feature.ID{feature.BGExtendedMode, feature.ExtendedAttributes, feature.RowAttributes},
	},
	{
		name:   "pattern-only attributes",
		writes: []registerWrite{{regDescriptorMode, extendedModeEnable | attributeModePatternOnly}},
		want:   []feature.ID{feature.BGExtendedMode},
	},
	{
		name:   "metatiles",
		writes: []registerWrite{{regMetatileMode, controlEnable}},
		want: []feature.ID{
			feature.BGExtendedMode,
			feature.ExtendedAttributes,
			feature.RowAttributes,
			feature.Metatiles,
		},
	},
	{
		name:   "two graphics sets",
		writes: []registerWrite{{regPlaneMode, planeBackgroundBit}, {regDescriptorMode, extendedModeEnable | attributeModeTile}},
		want:   []feature.ID{feature.BGExtendedMode, feature.ExtendedAttributes, feature.GraphicsSets},
	},
	{
		name:   "wide banks without extended background",
		writes: []registerWrite{{regPlaneMode, planeBackgroundBit}},
	},
	{
		name:   "sprite plane",
		writes: []registerWrite{{regPlaneMode, planeSpriteBit}},
		want:   []feature.ID{feature.SpriteExtendedMode},
	},
	{
		name:   "scanline remap",
		writes: []registerWrite{{regRemapMode, controlEnable}},
		want:   []feature.ID{feature.ScanlineRemap},
	},
	{
		name: "disabled modes",
		writes: []registerWrite{
			{regDescriptorMode, attributeModeFineY},
			{regMetatileMode, 0},
			{regRemapMode, 0},
		},
	},
}

type registerWrite struct {
	address uint16
	value   byte
}

type graphicsModeTest struct {
	name   string
	writes []registerWrite
	want   []feature.ID
}

func newTestMapper(t *testing.T) (*Mapper, *bus.Bus) {
	t.Helper()

	cart := &cartridge.Cartridge{
		Mapper: 800,
		PRG:    make([]byte, 4*1024*1024),
		CHR:    make([]byte, 4*1024*1024),
		Mirror: cartridge.Mirror4,
		NES2: &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{
			PRGVolatile: 1024 * 1024,
			CHRVolatile: 8 * 1024,
		}},
	}

	b := mappertest.NewBus(cart)

	mapper, err := New800(mapperbase.New(b))
	assert.NoError(t, err)

	m := mapper.(*Mapper)
	b.Mapper = m

	return m, b
}

func featureUsed(m *Mapper, id feature.ID) bool {
	for _, usage := range m.Features() {
		if usage.ID == id {
			return usage.Used
		}
	}

	return false
}

// A matching mapper number does not make the protocol of another submapper
// compatible. The constructor rejects it before it installs nametable hooks or
// memory mappings.
func TestRejectsOtherSubmapper(t *testing.T) {
	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			Mapper: 800,
			NES2:   &cartridge.NES2Metadata{Submapper: 1},
		},
	})

	mapper, err := New800(base)
	assert.Nil(t, mapper)
	assert.Error(t, err)
	assert.Equal(t, "MXM-1 submapper 1 is not supported", err.Error())
}

func selectCPU(m *Mapper, flags byte, bank uint16) {
	m.Write(regPRGSelect, flags)
	m.Write(regPRGBankHigh, byte(bank>>8))
	m.Write(regPRGBankLow, byte(bank))
}

func TestPRGBanksReadbackAndInterruptContext(t *testing.T) {
	const (
		windowOffset     = 7
		firstBank        = 0x120
		normalBank       = 0x124
		alternateRAMBank = 3
		alternateAddress = 0xc000 + windowOffset
	)
	m, _ := newTestMapper(t)

	m.prgROM[len(m.prgROM)-6] = 0x2d
	assert.Equal(t, byte(0x2d), m.Read(cpu6502.NMIAddress))

	for slot := range byte(4) {
		bank := uint16(firstBank) + uint16(slot)
		value := 0x80 + slot
		selectCPU(m, slot, bank)
		m.prgROM[int(bank)*prgBankSize+windowOffset] = value

		assert.Equal(t, value, m.Read(cpuWindowBase+windowOffset+uint16(slot)*prgBankSize))
		assert.Equal(t, slot, m.Read(regPRGReadback+uint16(slot)*prgReadbackStride))
		assert.Equal(t, byte(1), m.Read(regPRGReadback+1+uint16(slot)*prgReadbackStride))
		assert.Equal(t, byte(bank), m.Read(regPRGReadback+2+uint16(slot)*prgReadbackStride))
	}

	assert.False(t, featureUsed(m, feature.AlternateBank))
	m.Write(regAlternateMode, controlEnable)
	assert.True(t, featureUsed(m, feature.AlternateBank))
	m.Write(regAlternateRAM, bankSourceRAM)
	m.Write(regAlternateHigh, 0)
	m.Write(regAlternateLow, alternateRAMBank)
	m.Write(alternateAddress, 0x55)
	assert.Equal(t, byte(0x55), m.ram[alternateRAMBank*prgBankSize+windowOffset])

	selectCPU(m, alternateSlot, normalBank)
	m.prgROM[normalBank*prgBankSize+windowOffset] = 0x91
	assert.Equal(t, byte(0x55), m.Read(alternateAddress))

	m.Write(regAlternateMode, 0)
	assert.Equal(t, byte(0x91), m.Read(alternateAddress))
}

func TestGraphicsCPUWindowReadHideAndDeferredMapping(t *testing.T) {
	const (
		windowOffset  = 0x23
		windowAddress = nes.CodeBaseAddress + windowOffset
		firstPRGSlot  = 1
		ramBank       = 4
		chrROMBank    = 5
	)
	m, _ := newTestMapper(t)

	selectCPU(m, bankSourceRAM|prgGraphicsLatch|firstPRGSlot, ramBank)
	m.Write(windowAddress, 0x94)
	assert.Equal(t, byte(0x94), m.ram[ramBank*prgBankSize+windowOffset])
	assert.False(t, featureUsed(m, feature.GraphicsReadLatch))
	assert.Equal(t, byte(0), m.Read(windowAddress))
	assert.Equal(t, byte(0x94), m.Read(regGraphicsLatch))
	assert.True(t, featureUsed(m, feature.GraphicsReadLatch))

	// A change of the selection flags alone does not apply a new mapping.
	m.Write(regPRGSelect, firstPRGSlot)
	assert.Equal(t, byte(0x94), m.Read(windowAddress))

	selectCPU(m, prgGraphicsLatch|firstPRGSlot, chrROMBank)
	m.chrROM[chrROMBank*prgBankSize+windowOffset] = 0x67
	assert.Equal(t, byte(0), m.Read(windowAddress))
	assert.Equal(t, byte(0x67), m.Read(regGraphicsLatch))

	m.Write(windowAddress, 0xff)
	assert.Equal(t, byte(0x67), m.chrROM[chrROMBank*prgBankSize+windowOffset])
}

func TestPPUBankingAndBankedWrites(t *testing.T) {
	const (
		patternSlot    = 1
		patternBank    = 3
		patternOffset  = 1
		patternAddress = patternSlot*chrBankSize + patternOffset

		nametableSlot = 2
		nametableBank = 0x21
		tileOffset    = 5
		nameAddress   = nameTableStart + nametableSlot*ntBankSize + tileOffset
		readOffset    = nametableBank*ntBankSize + tileOffset
		uploadBank    = 0xa
		uploadOffset  = uploadBank*(64*1024) + nameAddress
	)
	m, b := newTestMapper(t)

	m.Write(regCHRSelect, bankSourceRAM|patternSlot)
	m.Write(regCHRBankHigh, 0)
	m.Write(regCHRBankLow, patternBank)
	m.Write(patternAddress, 0x6b)
	assert.Equal(t, byte(0x6b), m.ram[patternBank*chrBankSize+patternOffset])
	assert.Equal(t, byte(0x6b), m.Read(patternAddress))

	m.Write(regNametableMode, extendedModeEnable)
	m.Write(regNametableSelect, bankSourceRAM|nametableSlot)
	m.Write(regNametableHigh, 0)
	m.Write(regNametableLow, nametableBank)
	m.ram[readOffset] = 0x42
	assert.Equal(t, byte(0x42), b.NameTable.Read(nameAddress))

	m.Write(regWriteBankHigh, 0)
	m.Write(regWriteBankLow, uploadBank)
	assert.False(t, featureUsed(m, feature.PPUWriteBanking))
	b.NameTable.Write(nameAddress, 0x86)
	assert.Equal(t, byte(0x86), m.ram[uploadOffset])
	assert.True(t, featureUsed(m, feature.PPUWriteBanking))
	assert.Equal(t, byte(0x42), m.ram[readOffset])

	m.Write(regNametableMode, 0)
	b.NameTable.Write(nameAddress, 0x34)
	assert.Equal(t, byte(0x34), b.NameTable.Read(nameAddress))
}

func TestBlockCopySourcesAndForwardOverlap(t *testing.T) {
	const (
		markerOffset = 7
		prgData      = 0x11
		chrData      = 0x22
		ramData      = 0x33
	)
	m, _ := newTestMapper(t)

	// A copy outside the memory does not happen and does not mark the feature.
	m.Write(regCopyToHigh, 0xff)
	m.Write(regCopyCount, 0xff)
	assert.False(t, featureUsed(m, feature.BlockCopy))

	for _, test := range []struct{ source, want byte }{
		{copySourcePRG, prgData},
		{copySourceRAM, ramData},
		{copySourceCHR, chrData},
		{copySourceRAMAlias, ramData},
	} {
		m.prgROM[markerOffset] = prgData
		m.chrROM[markerOffset] = chrData
		m.ram[markerOffset] = ramData
		m.Write(regCopySource, test.source)
		m.Write(regCopyDestination, 2) // The copy goes to WorkRam with this selector too.
		m.Write(regCopyFromLow, 0)
		m.Write(regCopyFromHigh, 0)
		m.Write(regCopyToLow, 1)
		m.Write(regCopyToHigh, 0)
		m.Write(regCopyCount, 1)

		assert.Equal(t, test.want, m.ram[prgBankSize+markerOffset])
	}

	assert.True(t, featureUsed(m, feature.BlockCopy))

	m.ram[2*prgBankSize+markerOffset] = 0xff
	m.Write(regCopySource, copySourceRAM)
	m.Write(regCopyCount, 2)
	assert.Equal(t, byte(ramData), m.ram[2*prgBankSize+markerOffset])

	m.Write(regCopyToHigh, 0xff)
	m.Write(regCopyCount, 0xff) // An out-of-range copy is ignored. It does not panic.
}

func TestDescriptorSourcesAndEightScanlinePalettes(t *testing.T) {
	const (
		tileColumn           = 5
		nameAddress          = nameTableStart + tileColumn
		tileDescriptorOffset = tileColumn * descriptorSize
		patternBank          = 3
		lowPlaneAddress      = 0x420
		highPlaneAddress     = lowPlaneAddress + tileHeight
	)
	m, b := newTestMapper(t)

	m.Write(regDescriptorMode, extendedModeEnable|attributeModeTile)

	for _, flags := range []byte{0, descriptorPatternRAM, bankSourceRAM, descriptorPatternRAM | bankSourceRAM} {
		m.Write(regDescriptorSelect, flags)
		descriptors := m.chrROM
		patterns := m.chrROM

		if flags&bankSourceRAM != 0 {
			descriptors = m.ram
		}

		if flags&descriptorPatternRAM != 0 {
			patterns = m.ram
		}

		binary.BigEndian.PutUint16(descriptors[tileDescriptorOffset:], 3<<14|patternBank)
		patterns[patternBank*patternPageSize+lowPlaneAddress] = 0x95
		patterns[patternBank*patternPageSize+highPlaneAddress] = 0x59

		_ = b.NameTable.Read(nameAddress)
		assert.Equal(t, byte(0xff), b.NameTable.Read(attributeTableBase))
		assert.Equal(t, byte(0x95), m.Read(lowPlaneAddress))
		assert.Equal(t, byte(0x59), m.Read(highPlaneAddress))
	}

	m.Write(regDescriptorSelect, 0)
	m.Write(regDescriptorMode, extendedModeEnable|attributeModePackedRows)
	m.Write(regManualScroll, controlEnable)
	packedRowsOffset := descriptorBankSize + tileDescriptorOffset
	m.chrROM[packedRowsOffset] = 0b00_01_10_11   // Rows 0 through 3.
	m.chrROM[packedRowsOffset+1] = 0b11_10_01_00 // Rows 4 through 7.

	for fine, expected := range []byte{0, 0x55, 0xaa, 0xff, 0xff, 0xaa, 0x55, 0} {
		m.Write(regVerticalPosition, byte(fine))
		_ = b.NameTable.Read(nameAddress)
		assert.Equal(t, expected, b.NameTable.Read(attributeTableBase))
		_ = m.Read(lowPlaneAddress)
		_ = m.Read(highPlaneAddress)
	}
}

func TestMetatilesAndAnimationIndirection(t *testing.T) {
	const (
		tileColumn       = 5
		animationIndex   = 2
		animationFrame   = 7
		animationStride  = 8
		resolvedID       = 3
		animationOffset  = descriptorSize * (animationIndex*animationStride + animationFrame)
		lowPlaneAddress  = 0x420
		highPlaneAddress = lowPlaneAddress + tileHeight
	)
	m, b := newTestMapper(t)

	m.Write(regMetatileMode, controlEnable)
	m.Write(regMetatileStride, 0)
	m.Write(regMetatileFrame, animationFrame)
	binary.BigEndian.PutUint16(m.ram[metatileTable0+tileColumn*descriptorSize:], metatileAnimationMask|animationIndex)
	binary.BigEndian.PutUint16(m.ram[metatileAnimationTable+animationOffset:], resolvedID)
	copy(m.ram[metatilePlaneTable+resolvedID*metatileRecordSize:], []byte{0x10, 0x20, 0x30, 0x40})
	m.ram[metatilePaletteTable+resolvedID/2] = 0b00_00_10_11 // The odd ID uses the low nibble.

	for planeSet := range byte(2) {
		m.Write(regPlaneMode, planeBackgroundBit)
		m.Write(regPlaneSelect, planeSet)

		assert.Equal(t, byte(0), b.NameTable.Read(nameTableStart+tileColumn))
		assert.Equal(t, []byte{0xaa, 0xff}[planeSet], b.NameTable.Read(attributeTableBase))
		assert.Equal(t, []byte{0x10, 0x30}[planeSet], m.Read(lowPlaneAddress))
		assert.Equal(t, []byte{0x20, 0x40}[planeSet], m.Read(highPlaneAddress))
	}

	assert.True(t, featureUsed(m, feature.MetatileAnimation))
}

// The test checks all graphics mode features after each set of register writes.
func TestGraphicsModeFeatures(t *testing.T) {
	for _, test := range graphicsModeTests {
		t.Run(test.name, func(t *testing.T) {
			m, _ := newTestMapper(t)
			for _, write := range test.writes {
				m.Write(write.address, write.value)
			}

			for _, id := range graphicsModeFeatures {
				assert.Equal(t, slices.Contains(test.want, id), featureUsed(m, id), id)
			}
		})
	}
}

func TestVerticalRemapAndReadInhibit(t *testing.T) {
	const (
		sourceRow           = 13
		remapPages          = 4
		rowsPerPage         = 256
		remappedRow         = 57
		tileColumn          = 5
		nameAddress         = nameTableStart + tileColumn
		remappedNameAddress = nameAddress + (remappedRow/tileHeight)*tilesPerRow
	)
	m, b := newTestMapper(t)

	m.Write(regRemapMode, controlEnable)
	_ = m.Read(regRemapMode)

	for i := range remapPages * rowsPerPage {
		m.Write(regRemapData, byte(i))
	}

	assert.Equal(t, uint16(1024), m.remapCursor)

	for page := range byte(remapPages) {
		m.Write(regRemapPage, page)
		m.Write(regManualScroll, controlEnable)
		m.Write(regVerticalPosition, sourceRow)
		m.remapTable[int(page)*rowsPerPage+sourceRow] = remappedRow
		b.NameTable.Write(remappedNameAddress, 0x72)

		assert.Equal(t, byte(0x72), b.NameTable.Read(nameAddress))
		_ = m.Read(0x420)
	}

	m.Write(regReadInhibit, 2)
	assert.Equal(t, byte(0), b.NameTable.Read(nameAddress))

	m.ClockCPU(1)
	assert.Equal(t, byte(0), b.NameTable.Read(nameAddress))

	m.ClockCPU(1)
	assert.Equal(t, byte(0x72), b.NameTable.Read(nameAddress))
}

// irqCPU records the IRQ line. The embedded nil interface panics on all other
// CPU methods. The tests do not call them.
type irqCPU struct {
	bus.CPU
	active bool
}

var _ bus.CPU = (*irqCPU)(nil)

func (c *irqCPU) SetIRQ(active bool) { c.active = active }

func TestIRQFetchDetectorAcknowledgeAndNMIVector(t *testing.T) {
	m, b := newTestMapper(t)
	cpu := &irqCPU{}
	b.CPU = cpu

	m.Write(regIRQEnable, controlEnable)
	m.Write(regIRQTarget, 1)

	for range 3 {
		_ = b.NameTable.Read(nameTableStart)
	}

	assert.Equal(t, byte(0), m.Read(regScanline))

	_ = b.NameTable.Read(nameTableStart + 1)
	for range 3 {
		_ = b.NameTable.Read(nameTableStart)
	}

	assert.True(t, cpu.active)

	m.ClockCPU(1)
	assert.Equal(t, byte(3), m.Read(regIRQAcknowledge))
	assert.False(t, cpu.active)

	m.ClockCPU(2)
	assert.Equal(t, byte(5), m.Read(regIRQAcknowledge))

	_ = m.Read(cpu6502.NMIAddress)
	assert.Equal(t, byte(0), m.Read(regScanline))
	assert.False(t, m.fetch.inFrame)

	// Inspector queries do not start another scanline.
	state := m.registers
	_ = m.InspectCHR(0)
	_, _ = m.InspectMemory(nameTableStart)
	assert.Equal(t, state, m.registers)
}

func TestPPUWriteTapAndReset(t *testing.T) {
	m, _ := newTestMapper(t)

	m.ObservePPURegisterWrite(register.PPU_SCROLL, 17)
	assert.False(t, featureUsed(m, feature.PPURegisterCapture))
	_ = m.Read(0)
	assert.Equal(t, byte(17), m.verticalPosition)
	assert.True(t, featureUsed(m, feature.PPURegisterCapture))

	m.ObservePPURegisterWrite(register.PPU_SCROLL+ppuRegisterCount, 99)
	_ = m.Read(0)
	assert.Equal(t, byte(17), m.verticalPosition)

	selectCPU(m, bankSourceRAM|1, 3)
	m.Write(nes.CodeBaseAddress, 0x42)
	m.Reset()
	assert.Equal(t, byte(0x42), m.ram[3*prgBankSize])
	assert.Equal(t, [5]uint16{0, 0, 1, 2, 0xffff}, m.prgBanks)
}

func TestSpriteSlotDummyAttributeFetch(t *testing.T) {
	const (
		firstSpriteFetchDot   = renderstate.LastVisibleDot + 1
		dummyAttributeDot     = firstSpriteFetchDot + 2
		firstSpritePatternDot = firstSpriteFetchDot + 4
		visibleScanline       = 20
		spriteSlots           = 8
		dotsPerTile           = 8
		prefetchTiles         = 2
	)
	m, b := newTestMapper(t)

	// The 32 visible tiles each supply a name, attribute and two patterns.
	for tile := range tilesPerRow {
		m.TickPPU(renderstate.FirstVisibleDot+tile*dotsPerTile, visibleScanline, true)
		_ = b.NameTable.Read(nameTableStart + uint16(tile))
		_ = b.NameTable.Read(attributeTableBase)
		_ = m.Read(0)
		_ = m.Read(tileHeight)
	}

	assert.Equal(t, uint32(32), m.fetch.nameReads)

	// nesgoemu makes two nametable reads in each sprite slot. The demo expects
	// an attribute address in the second read. It counts only one name read.
	for slot := range spriteSlots {
		m.TickPPU(firstSpriteFetchDot+dotsPerTile*slot, visibleScanline, true)
		_ = b.NameTable.Read(nameTableStart)
		m.TickPPU(dummyAttributeDot+dotsPerTile*slot, visibleScanline, true)
		_ = b.NameTable.Read(nameTableStart)
		assert.Equal(t, uint16(attributeTableBase), m.fetch.lastAddress)
		m.TickPPU(firstSpritePatternDot+dotsPerTile*slot, visibleScanline, true)
		_ = m.Read(0)
		_ = m.Read(tileHeight)
	}

	assert.Equal(t, uint32(40), m.fetch.nameReads)

	for tile := range prefetchTiles {
		m.TickPPU(renderstate.FirstBackgroundPrefetchDot+tile*dotsPerTile, visibleScanline, true)
		_ = b.NameTable.Read(nameTableStart + uint16(tile))
		_ = b.NameTable.Read(attributeTableBase)
		_ = m.Read(0)
		_ = m.Read(tileHeight)
	}

	assert.Equal(t, uint32(42), m.fetch.nameReads)

	m.TickPPU(dummyAttributeDot, renderstate.VisibleScanlines+1, true)
	_ = b.NameTable.Read(nameTableStart)
	assert.Equal(t, uint16(nameTableStart), m.fetch.lastAddress)
}

func TestPhysicalMappingInspection(t *testing.T) {
	m, _ := newTestMapper(t)

	selectCPU(m, bankSourceRAM|1, 7)
	m.Write(nes.CodeBaseAddress+1, 0x42)
	value, ok := m.InspectCPU(nes.CodeBaseAddress + 1)
	assert.True(t, ok)
	assert.Equal(t, byte(0x42), value)

	selectCPU(m, prgGraphicsLatch, 1)
	m.readLatch = 0x71
	value, ok = m.InspectCPU(cpuWindowBase)
	assert.True(t, ok)
	assert.Equal(t, byte(0), value)
	assert.Equal(t, byte(0x71), m.readLatch)

	m.fetch.inFrame = true
	m.scanline = 42
	m.irqPending = true
	_, ok = m.InspectCPU(cpu6502.NMIAddress)
	assert.True(t, ok)
	assert.True(t, m.fetch.inFrame)
	assert.True(t, m.irqPending)
	assert.Equal(t, byte(42), m.scanline)

	_, ok = m.InspectCPU(cpuWindowBase - 1)
	assert.False(t, ok)

	m.Write(regCHRSelect, 0)
	m.Write(regCHRBankLow, 7)
	mapping, ok := m.PPUReadMapping(0x21)
	assert.True(t, ok)
	assert.Equal(t, bus.GraphicsMapping{
		Memory: bus.GraphicsCHRROM,
		Offset: 7*chrBankSize + 0x21,
	}, mapping)

	m.Write(regDescriptorMode, extendedModeEnable|attributeModeTile)
	_, ok = m.PPUReadMapping(0)
	assert.False(t, ok)

	_, ok = m.PPUWriteMapping(0)
	assert.False(t, ok)
}

func TestPRGROMOffsetFollowsActiveAlternateSource(t *testing.T) {
	const alternateWindowAddress = 0xc000
	m, _ := newTestMapper(t)
	m.prgROM[0] = 0xa5
	m.Write(regAlternateMode, controlEnable)
	offset, ok := m.PRGROMOffset(alternateWindowAddress)
	assert.True(t, ok)
	assert.Equal(t, 0, offset)
	assert.Equal(t, byte(0xa5), m.Read(alternateWindowAddress))

	m.Write(regAlternateRAM, bankSourceRAM)
	assert.Equal(t, byte(0), m.Read(alternateWindowAddress))
	_, ok = m.PRGROMOffset(alternateWindowAddress)
	assert.False(t, ok)
}
