package mxm1_test

import (
	"encoding/binary"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	prgBankSize = 0x2000
	chrBankSize = 0x0200

	nametableSize      = 0x0400
	nameTableBase      = 0x2000
	attributeTableBase = 0x23c0
	tilesPerRow        = 32

	patternPageSize  = 0x1000
	tilePatternBytes = 16
	tileHeight       = 8

	descriptorSize     = 2
	descriptorPageSize = 2 * nametableSize

	extendedModeEnable       = 0x80
	attributeModePackedRows  = 2
	attributeModePatternOnly = 3

	reg801PRGBankLow           = 0x5010
	reg801IgnoredPRGWrite      = 0x5012
	reg801PRGSelectionReadback = 0x501c
	reg801PRGBankHigh          = 0x5020
	reg801PRGSelect            = 0x5022

	reg801AlternateEnable  = 0x5014
	reg801AlternateSource  = 0x5015
	reg801AlternateBankLow = 0x5017

	reg801CHRBankHigh     = 0x5011
	reg801CHRBankLow      = 0x5013
	reg801CHRSelect       = 0x5021
	reg801IgnoredCHRWrite = 0x5023

	regDescriptorMode    = 0x5040
	regDescriptorSelect  = 0x5041
	regDescriptorBankLow = 0x5043

	reg801NametableMode    = 0x5050
	reg801NametableSelect  = 0x5051
	reg801NametableBankLow = 0x5053

	reg801RemapMode          = 0x50e0
	reg801RemapData          = 0x50e1
	reg801RemapOffset        = 0x50e3
	reg801RemapPositionAlias = 0x50e6
)

func TestMapper801PRGBanks(t *testing.T) {
	const (
		lowWindow       = 0x6000
		windowOffset    = 7
		alternateWindow = 0xc000
		firstPRGSlot    = 1
		ramSource       = 0x80
		bankLow         = 0xfc
		bankHigh        = 7 // Only bit 0 remains in the effective bank.
		ramBank         = 3
		ramData         = 0xa5
		resetVectorLow  = 0x73
	)
	b, cart := newMapper801Bus(t)
	m := b.Mapper
	cart.PRG[0x1fc*prgBankSize+windowOffset] = 0x41
	cart.PRG[0x0fc*prgBankSize+windowOffset] = 0x42
	cart.PRG[len(cart.PRG)-(0x10000-cpu6502.ResetAddress)] = resetVectorLow

	m.Write(reg801PRGSelect, 0)
	m.Write(reg801PRGBankLow, bankLow)
	assert.Equal(t, byte(0x42), m.Read(lowWindow+windowOffset))
	m.Write(reg801PRGBankHigh, bankHigh)
	m.Write(reg801IgnoredPRGWrite, 2)
	assert.Equal(t, byte(0x41), m.Read(lowWindow+windowOffset))
	assert.Equal(t, byte(bankLow), m.Read(reg801PRGBankHigh))
	assert.Equal(t, byte(bankHigh), m.Read(reg801PRGSelect))
	assert.Equal(t, byte(resetVectorLow), m.Read(cpu6502.ResetAddress))

	m.Write(reg801PRGSelect, ramSource|firstPRGSlot)
	m.Write(reg801PRGBankHigh, 0)
	m.Write(reg801PRGBankLow, ramBank)
	m.Write(nes.CodeBaseAddress+windowOffset, ramData)
	assert.Equal(t, byte(ramSource|firstPRGSlot), m.Read(reg801PRGSelectionReadback))
	assert.Equal(t, byte(ramData), m.Read(nes.CodeBaseAddress+windowOffset))
	m.Write(reg801PRGSelect, firstPRGSlot)
	assert.Equal(t, byte(0), m.Read(nes.CodeBaseAddress+windowOffset))
	m.Write(reg801PRGSelect, ramSource|firstPRGSlot)
	assert.Equal(t, byte(ramData), m.Read(nes.CodeBaseAddress+windowOffset))

	m.Write(reg801AlternateEnable, 1)
	m.Write(reg801AlternateBankLow, 1)
	m.Write(reg801AlternateSource, ramSource)
	assert.Equal(t, byte(ramData), m.Read(alternateWindow+windowOffset))
	value, ok := m.(bus.CPUMemoryInspector).InspectCPU(alternateWindow + windowOffset)
	assert.True(t, ok)
	assert.Equal(t, byte(ramData), value)

	m.(bus.MapperResetter).Reset()
	assert.Equal(t, byte(0), m.Read(lowWindow+windowOffset))
	assert.Equal(t, byte(resetVectorLow), m.Read(cpu6502.ResetAddress))
}

func TestMapper801CHRBanks(t *testing.T) {
	const (
		patternSlot  = 8
		windowOffset = 7
		bankLow      = 0x34
	)
	b, cart := newMapper801Bus(t)
	m := b.Mapper
	cart.CHR[0x2134*chrBankSize+windowOffset] = 0xa5
	cart.CHR[windowOffset] = 0x41

	m.Write(reg801CHRSelect, patternSlot)
	m.Write(reg801CHRBankHigh, 0xe1) // The bank high byte uses bits 5 through 0.
	m.Write(reg801CHRBankLow, bankLow)
	m.Write(reg801IgnoredCHRWrite, 0x51)
	m.Write(reg801CHRSelect, patternSlot+16) // This slot is out of range.
	m.Write(reg801CHRBankLow, 0x52)
	assert.Equal(t, byte(0xa5), m.Read(patternSlot*chrBankSize+windowOffset))
	assert.Equal(t, byte(0x41), m.Read((patternSlot+1)*chrBankSize+windowOffset))
}

func TestMapper801RemapFetch(t *testing.T) {
	const (
		sourceRow              = 80
		remapTableRow          = 240
		rowOffset              = 6
		wrappedFineY           = 6
		lowerNametable         = 2
		nametableBank          = 1
		descriptorBank         = 1
		tileColumn             = 5
		tileIndex              = 2
		descriptorPatternBank  = 0x123
		lowBytePatternBank     = 0x23
		palette2InAllQuadrants = 0xaa
	)
	b, cart := newMapper801Bus(t)
	m := b.Mapper

	// Row 240 plus 6 wraps to row 6 and selects the lower nametable.
	m.Write(reg801NametableMode, extendedModeEnable)
	m.Write(reg801NametableSelect, lowerNametable)
	m.Write(reg801NametableBankLow, nametableBank)
	cart.CHR[nametableBank*nametableSize+tileColumn] = tileIndex
	m.Write(regDescriptorSelect, lowerNametable)
	m.Write(regDescriptorBankLow, descriptorBank)
	descriptorOffset := descriptorBank*descriptorPageSize + tileColumn*descriptorSize
	binary.BigEndian.PutUint16(cart.CHR[descriptorOffset:], 2<<14|descriptorPatternBank)

	// The source row selects palette 3. The wrapped row selects palette 2.
	packedRowsOffset := descriptorOffset + descriptorPageSize
	cart.CHR[packedRowsOffset] = 0b11_10_01_00   // Rows 0 through 3.
	cart.CHR[packedRowsOffset+1] = 0b00_01_10_11 // Rows 4 through 7.

	patternOffset := tileIndex*tilePatternBytes + wrappedFineY
	cart.CHR[descriptorPatternBank*patternPageSize+patternOffset] = 0x31
	cart.CHR[descriptorPatternBank*patternPageSize+patternOffset+tileHeight] = 0x32
	cart.CHR[lowBytePatternBank*patternPageSize+patternOffset] = 0x33
	cart.CHR[lowBytePatternBank*patternPageSize+patternOffset+tileHeight] = 0x34

	m.Write(reg801RemapMode, 1)
	m.Write(reg801RemapPositionAlias, sourceRow)
	m.Write(reg801RemapOffset, rowOffset)
	uploadRemapRow(m, sourceRow, remapTableRow)

	const (
		nameAddress      = nameTableBase + 2*tilesPerRow + tileColumn
		lowPlaneAddress  = tileIndex * tilePatternBytes
		highPlaneAddress = lowPlaneAddress + tileHeight
	)
	for _, test := range []struct {
		name            string
		mode, low, high byte
	}{
		{name: "tile attributes", mode: 0, low: 0x31, high: 0x32},
		{name: "packed row attributes", mode: attributeModePackedRows, low: 0x31, high: 0x32},
		{name: "low descriptor byte for patterns", mode: attributeModePatternOnly, low: 0x33, high: 0x34},
	} {
		t.Run(test.name, func(t *testing.T) {
			m.Write(regDescriptorMode, extendedModeEnable|test.mode)
			assert.Equal(t, byte(tileIndex), b.NameTable.Read(nameAddress))
			assert.Equal(t, byte(palette2InAllQuadrants), b.NameTable.Read(attributeTableBase))
			assert.Equal(t, test.low, m.Read(lowPlaneAddress))
			assert.Equal(t, test.high, m.Read(highPlaneAddress))
		})
	}
}

func TestMapper801RejectsUnsupportedLayout(t *testing.T) {
	b, cart := newMapper801Bus(t)
	cart.NES2.Submapper = 1
	_, err := mapper.New(b)
	assert.Error(t, err)

	cart.NES2.Submapper = 0
	cart.CHR = nil
	_, err = mapper.New(b)
	assert.Error(t, err)
}

func uploadRemapRow(m bus.Mapper, sourceRow, remappedRow byte) {
	const bytesPerRow = 2
	_ = m.Read(reg801RemapMode)
	for range bytesPerRow * int(sourceRow) {
		m.Write(reg801RemapData, 0)
	}
	m.Write(reg801RemapData, 0) // Keep the nametable selected by the row.
	m.Write(reg801RemapData, remappedRow)
}

func newMapper801Bus(t *testing.T) (*bus.Bus, *cartridge.Cartridge) {
	t.Helper()
	cart := &cartridge.Cartridge{
		Mapper: 801,
		PRG:    make([]byte, 4*1024*1024),
		CHR:    make([]byte, 8*1024*1024),
		Mirror: cartridge.Mirror4,
		NES2: &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{
			PRGVolatile: 2 * prgBankSize,
		}},
	}

	b := mappertest.NewBus(cart)
	m, err := mapper.New(b)
	assert.NoError(t, err)
	b.Mapper = m
	return b, cart
}
