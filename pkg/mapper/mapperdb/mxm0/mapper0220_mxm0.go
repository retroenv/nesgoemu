// Package mxm0 implements mapper 220, submapper 1, for the N8 PRO release of
// House of Truth (2022). It supports the layout of that release: 32 KiB PRG ROM
// and 24 KiB CHR ROM.
//
// Memory:
//
//	$0000-$3FFF of CHR ROM: four pattern banks of 4 KiB.
//	$4000-$5FFF of CHR ROM: attribute bytes, 1 KiB for each fine Y value.
//
// The mapper gives each background tile a different attribute on each pixel row.
// A nametable fetch latches its address. The next attribute fetch reads
//
//	$4000 + fineY*$400 + nametable address
//
// Bits 1-0 of that byte select the pattern bank for the next pattern fetches,
// and PPU address bit 12 has no effect. Bits 7-6 or bits 5-4 are the palette
// number, as $6000 selects. The game changes $6000 on each frame to mix colors.
// Thus the bit of $6000 is the blend phase. The phase also selects the palette
// table. The decompiled Verilog of the bittrace project calls this bit "field".
// The original names are not known.
//
// The game has two brightness modes. In dim mode, the NMI handler sets all three
// color emphasis bits of PPUMASK and writes 1 to $6002. Then palette entry 1 of
// phase 0 is $1C, not $11. The game starts in dim mode. The mapper also starts
// with $1C, because it stores the bit inverted and has no reset for it.
//
// Registers. The mapper uses bit 0 of the written value and full address decoding:
//
//	$6000  W  blend phase
//	$6001  W  1: start the ROM check
//	$6002  W  1: dim mode, entry 1 of phase 0 is $1C. 0: entry 1 is $11
//	$6010-$601F  R  palette bytes
package mxm0

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
)

const (
	name = "MXM-0"

	prgSize = 0x8000
	chrSize = 0x6000

	regPhase   = 0x6000
	regCheck   = 0x6001
	regDimMode = 0x6002

	paletteRegFirst  = 0x6010
	paletteRegLast   = 0x601F
	paletteIndexMask = 0x0F

	dimEntryIndex  = 1
	dimEntry1Color = 0x1C

	patternTableEnd = 0x2000
	paletteRAMStart = 0x3F00
	// Hardware decodes an attribute fetch from PPU A9-A6 only. PPU A13 selects
	// the nametable space.
	attributeMask = 0x03C0
	nameLatchMask = 0x03FF

	patternOffsetMask = 0x0FFF
	patternBankShift  = 12
	bankMask          = 0x03
	attributeBase     = 0x4000
	fineYShift        = 10
	fineYMask         = 0x07

	// phase0Shift and phase1Shift are the positions of the palette number in
	// an attribute byte. attributeCopies repeats the number in all four
	// quadrants of the byte for the PPU.
	phase0Shift       = 6
	phase1Shift       = 4
	paletteNumberMask = 0x03
	attributeCopies   = 0x55

	// fetchesPerLine is the count of PPU reads on a scanline with rendering on.
	fetchesPerLine = 170
)

// paletteTable holds the palette bytes for phases 0 and 1.
var paletteTable = [2][16]byte{
	{0x0F, 0x11, 0x2C, 0x3A, 0x0F, 0x18, 0x28, 0x39, 0x0F, 0x16, 0x28, 0x37, 0x0F, 0x00, 0x26, 0x37},
	{0x0F, 0x11, 0x2C, 0x3B, 0x0F, 0x19, 0x29, 0x38, 0x0F, 0x17, 0x27, 0x39, 0x0F, 0x17, 0x10, 0x39},
}

var declaredFeatures = []feature.ID{
	feature.CHRBanking,
	feature.CRCCheck,
	feature.ExtendedAttributes,
	feature.PPUBusTiming,
	feature.PPURegisterCapture,
	feature.RowAttributes,
}

// Mapper supplies the pattern banks, the attributes of each row, and the palette
// registers.
type Mapper struct {
	*mapperbase.Base

	chr []byte

	dimMode bool
	phase   int // 0 or 1

	fetchCount  int
	fineY       uint16
	nameLatch   uint16
	patternBank uint16
}

// New constructs the mapper. It rejects other submappers, other PRG ROM sizes,
// and CHR ROM smaller than 24 KiB.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	cart := base.Cartridge()
	if cart.NES2 != nil && cart.NES2.Submapper != 1 {
		return nil, fmt.Errorf("mapper 220 submapper %d is not supported", cart.NES2.Submapper)
	}
	if err := mapperbase.ValidateSize("MXM-0 PRG ROM", len(cart.PRG), prgSize, prgSize, prgSize); err != nil {
		return nil, fmt.Errorf("checking memory size: %w", err)
	}
	if len(cart.CHR) < chrSize {
		return nil, fmt.Errorf("checking memory size: unsupported MXM-0 CHR ROM size %d", len(cart.CHR))
	}

	m := &Mapper{
		Base:    base,
		chr:     cart.CHR,
		dimMode: true,
	}
	m.SetName(name)
	m.SetPrgRAM(nil)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}
	if err := m.SetNameTableMirrorMode(cartridge.MirrorVertical); err != nil {
		return nil, fmt.Errorf("setting mirror mode: %w", err)
	}

	for _, id := range declaredFeatures {
		m.DeclareFeature(id)
	}

	m.NameTableMemory().SetReadHook(m.readNameTable)
	m.NameTableMemory().SetWriteHook(m.writeNameTable)

	return m, nil
}

// Read handles pattern fetches, palette registers, and PRG ROM.
func (m *Mapper) Read(address uint16) byte {
	switch {
	case address < patternTableEnd:
		return m.readPattern(address)
	case address >= paletteRegFirst && address <= paletteRegLast:
		return m.paletteByte(address)
	}

	return m.Base.Read(address)
}

// Write handles the registers. CHR ROM and PRG ROM ignore writes.
func (m *Mapper) Write(address uint16, value byte) {
	set := value&1 != 0

	switch address {
	case regPhase:
		m.phase = int(value & 1)

	case regCheck:
		if set {
			m.MarkFeature(feature.CRCCheck)
		}

	case regDimMode:
		m.dimMode = set
	}
}

// ObservePPURegisterWrite restarts the row count on a write to $2005. The next
// completed scanline of fetches changes fine Y from 7 to 0.
func (m *Mapper) ObservePPURegisterWrite(address uint16, _ byte) {
	if address == register.PPU_SCROLL {
		m.MarkFeature(feature.PPURegisterCapture)
		m.fineY = fineYMask
		m.fetchCount = 0
	}
}

// InspectCHR returns a pattern byte as the mapper currently maps it. It does not
// change the mapper state.
func (m *Mapper) InspectCHR(address uint16) byte {
	return m.chr[m.patternOffset(address)]
}

// InspectPRGRAM reports false because the board has no PRG RAM.
func (*Mapper) InspectPRGRAM(int) (byte, bool) {
	return 0, false
}

// InspectMemory returns the pattern and attribute bytes that the mapper supplies.
// It reports false for nametable bytes and palette RAM.
func (m *Mapper) InspectMemory(address uint16) (byte, bool) {
	switch {
	case address < patternTableEnd:
		return m.InspectCHR(address), true
	case address >= paletteRAMStart || !isAttribute(address):
		return 0, false
	}

	return m.paletteAttribute(m.rowAttribute()), true
}

// PPUReadMapping reports the physical byte of a pattern fetch.
func (m *Mapper) PPUReadMapping(address uint16) (bus.GraphicsMapping, bool) {
	if address >= patternTableEnd {
		return bus.GraphicsMapping{}, false
	}

	return bus.GraphicsMapping{
		Memory: bus.GraphicsCHRROM,
		Offset: m.patternOffset(address),
	}, true
}

// PPUWriteMapping reports false because the board has no CHR RAM.
func (*Mapper) PPUWriteMapping(uint16) (bus.GraphicsMapping, bool) {
	return bus.GraphicsMapping{}, false
}

func (m *Mapper) readPattern(address uint16) byte {
	m.MarkFeature(feature.CHRBanking)

	value := m.chr[m.patternOffset(address)]
	m.countFetch()

	return value
}

func (m *Mapper) patternOffset(address uint16) int {
	return int(m.patternBank<<patternBankShift | address&patternOffsetMask)
}

// readNameTable reads an attribute byte from CHR ROM. The caller reads other
// nametable bytes from console memory. In the logic, the pattern bank latches
// before the fetch count changes fine Y. The PPU samples the data at the end of
// the fetch, after that change. Thus the bank uses the old fine Y and the returned
// byte uses the new fine Y on the last fetch of a scanline.
func (m *Mapper) readNameTable(address uint16) (byte, bool) {
	if !isAttribute(address) {
		m.nameLatch = address & nameLatchMask
		m.countFetch()

		return 0, false
	}

	m.MarkFeature(feature.ExtendedAttributes)
	m.MarkFeature(feature.RowAttributes)

	m.patternBank = uint16(m.rowAttribute() & bankMask)
	m.countFetch()

	return m.paletteAttribute(m.rowAttribute()), true
}

// writeNameTable discards attribute writes. The logic disables console memory for
// these addresses. It does not use the PPU write signal, so this rule follows from
// that output. Hardware tests have not confirmed it.
func (*Mapper) writeNameTable(address uint16, _ byte) bool {
	return isAttribute(address)
}

func (m *Mapper) attributeOffset() int {
	return int(attributeBase | m.fineY<<fineYShift | m.nameLatch)
}

func (m *Mapper) rowAttribute() byte {
	return m.chr[m.attributeOffset()]
}

func (m *Mapper) paletteAttribute(value byte) byte {
	shift := phase0Shift
	if m.phase == 1 {
		shift = phase1Shift
	}

	return (value >> shift & paletteNumberMask) * attributeCopies
}

func (m *Mapper) countFetch() {
	m.fetchCount++
	if m.fetchCount == fetchesPerLine {
		m.MarkFeature(feature.PPUBusTiming)
		m.fetchCount = 0
		m.fineY = (m.fineY + 1) & fineYMask
	}
}

func (m *Mapper) paletteByte(address uint16) byte {
	index := address & paletteIndexMask
	if m.phase == 0 && m.dimMode && index == dimEntryIndex {
		return dimEntry1Color
	}

	return paletteTable[m.phase][index]
}

func isAttribute(address uint16) bool {
	return address&attributeMask == attributeMask
}
