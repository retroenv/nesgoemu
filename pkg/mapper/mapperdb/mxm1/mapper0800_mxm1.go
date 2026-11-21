// Package mxm1 implements mapper 800 for Former Dawn Demo v3.2, mapper 801
// for Former Dawn Hover Boots Demo, and mapper 128 for Bad Apple Ultimate NES
// Demake. All three use submapper 0. Mapper 128 reads graphics and PCM data
// from a file that the ROM selects.
// The package does not implement expansion audio or CRT display effects.
//
// The older MXM-0 board (220, submapper 1, the House of Truth minimapper) uses
// a different register protocol and graphics packing. Its flashcart FPGA
// configurations are evidence for that board. They are not a specification for
// this mapper.
//
// On mappers 800 and 801, the CPU banks PRG ROM and WorkRam. It uploads
// graphics descriptors and predecoded tiles into WorkRam. The mapper detects
// the PPU sequence of name, attribute, and pattern fetches. It replaces the
// bytes of these fetches with bytes from the tables. The PPU uses those bytes
// to render the graphics.
//
// The mapper bitstream in the Hover Boots release is the reference for the
// mapper 801 bank ports and descriptor fetches. This implementation updates the
// vertical counter when the CPU writes to it. The PPU fetch sequence controls
// the scanline timing.
package mxm1

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/renderstate"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
)

var declaredFeatures = []feature.ID{
	feature.PRGBanking,
	feature.CHRBanking,
	feature.CHRSourceSelect,
	feature.LowBankMapping,
	feature.AlternateBank,
	feature.GraphicsReadLatch,
	feature.BlockCopy,
	feature.NameTableBanking,
	feature.PPUWriteBanking,
	feature.ExtendedAttributes,
	feature.RowAttributes,
	feature.BGExtendedMode,
	feature.Metatiles,
	feature.MetatileAnimation,
	feature.GraphicsSets,
	feature.SpriteExtendedMode,
	feature.ScanlineRemap,
	feature.ScanlineIRQ,
	feature.PPUBusTiming,
	feature.PPURegisterCapture,
}

// window caches an effective mapping. It is separate from the register
// readback. A write to a source selector can change the register state before
// a write to a bank byte rebuilds the mapping. Do not mix the two.
type window struct {
	data     []byte
	offset   int
	writable bool
	prg      bool
}

type registers struct {
	prgBanks  [cpuWindowCount]uint16
	prgFlags  [cpuWindowCount]byte
	prgMode   byte
	prgSelect byte
	readLatch byte

	altBank    uint16
	altEnabled bool
	altRAM     bool

	chrBanks  [chrWindowCount]uint16
	chrMode   byte
	chrRAM    [chrWindowCount]bool
	chrSelect byte

	irqEnabled      bool
	irqOffset       byte
	irqPending      bool
	irqPhase        byte
	irqTarget       byte
	previousPending bool
	scanline        byte

	attributeMode byte
	descriptor    graphicsBanks
	descriptorRAM [ntWindowCount]bool
	patternRAM    [ntWindowCount]bool

	nametable graphicsBanks
	ntMode    byte
	ntRAM     [ntWindowCount]bool

	secondPlane       bool
	spriteMode        bool
	spriteSecondPlane bool
	wideBanks         bool

	readInhibit byte
	writeBank   uint16

	copyDestination byte
	copySource      byte
	copyFrom        uint16
	copyTo          uint16

	metatileEnabled bool
	metatileFrame   byte
	metatileStride  byte
	metatileTables  [ntWindowCount]byte

	remapCursor  uint16
	remapEnabled bool
	remapPage    byte
	remapTable   [remapTableSize]byte

	remapNTFlip801   bool
	remapOffset801   byte
	remapPosition801 byte

	streamCursor byte

	manualScroll     byte
	ppuRegisters     [ppuRegisterCount]byte
	scrollDirty      bool
	verticalPosition byte
}

// fetchState tracks PPU fetch sequences and the data latched for each tile.
type fetchState struct {
	dummyAttribute    bool
	frameStart        bool
	inFrame           bool
	lastAddress       uint16
	nameReads         uint32
	previousNameReads byte
	repeatCount       byte

	metatileFetchesLeft byte
	metatileHigh        byte
	metatileLow         byte
	metatilePalette     byte

	descriptorFetchesLeft byte
	tileFineY             byte
	tileOffset            uint16
	tileTable             byte
}

// Mapper supplies the CPU windows, PPU fetch substitutions, and scanline IRQ
// of MXM.
type Mapper struct {
	*mapperbase.Base
	isMapper801 bool

	registers
	fetch fetchState

	prgROM, chrROM, ram, chrMemory []byte

	cpuWindows [cpuWindowCount]window
	ppuWindows [chrWindowCount]window
	ntWindows  [ntWindowCount]window
}

// New800 constructs mapper 800, submapper 0. It uses the NES 2.0 RAM sizes
// when the header is available.
func New800(base *mapperbase.Base) (bus.Mapper, error) {
	return newMapper(base, false)
}

// Reset restores the mapper registers. It does not erase cartridge RAM.
func (m *Mapper) Reset() {
	m.fetch = fetchState{}
	m.registers = registers{
		prgBanks:        [cpuWindowCount]uint16{0, 0, 1, 2, lastPRGBank},
		metatileStride:  metatileDefaultStride,
		copySource:      copySourcePRG,
		copyDestination: copySourceRAM,
	}

	if !m.isMapper801 && len(m.ram) > ntBankSize {
		m.prgFlags[0] = bankSourceRAM
	}

	for slot, bank := range m.prgBanks {
		m.mapCPU(slot, bank, m.prgFlags[slot])
	}

	for slot := range m.ppuWindows {
		m.ppuWindows[slot] = window{data: m.chrROM}
		if !m.isMapper801 {
			m.ppuWindows[slot].offset = (initialCHRBank + slot) * chrBankSize
		}
	}

	if !m.isMapper801 {
		m.ppuWindows[initialCHRExceptionSlot].offset = initialCHRExceptionBank * chrBankSize
	}

	m.ntWindows = [ntWindowCount]window{}
	m.SetMapperIRQ(false)
}

// TickPPU marks the second dummy fetch in each sprite slot (dots 259, 267, ...,
// 315). The demo counts 42 tile-name reads for each scanline. nesgoemu makes a
// dummy nametable read at these dots, as the hardware does. The mapper treats
// it as an attribute read, so the phase gate keeps the timing that the demo
// expects. This applies only to this mapper.
func (m *Mapper) TickPPU(cycle, scanLine int, rendering bool) {
	renderLine := scanLine < renderstate.VisibleScanlines || scanLine == renderstate.PreRenderScanline
	m.fetch.dummyAttribute = rendering && renderLine &&
		cycle >= firstDummyAttribute && cycle <= lastDummyAttribute && (cycle-firstDummyAttribute)%spriteFetchSlotDots == 0
}

// Read handles CPU memory and pattern-table reads. Nametable reads use the
// nametable hook, so they are not confused with the CPU register space at
// $2000.
func (m *Mapper) Read(address uint16) byte {
	if address < patternTableEnd {
		return m.readVRAM(address)
	}

	if address == cpu6502.NMIAddress || address == cpu6502.NMIAddress+1 {
		// The demo uses reads of the NMI vector as the frame boundary for the
		// scanline detector. Debugger peeks must not cause this side effect.
		m.fetch.inFrame = false
		m.fetch.lastAddress = 0
		m.fetch.repeatCount = 0
		m.scanline = 0
		m.irqPending = false
		m.SetMapperIRQ(false)
	}

	if address < cpuWindowBase {
		return m.readRegister(address)
	}

	slot := int(address-cpuWindowBase) / prgBankSize
	w := m.cpuReadWindow(slot)
	value := readWrapped(w.data, w.offset+int(address&prgOffsetMask), m.OpenBus())
	if m.prgFlags[slot]&prgGraphicsLatch != 0 {
		// The graphics aperture hides the data of the original CPU read.
		// Software gets the byte with a later read of $50B0.
		m.MarkFeature(feature.GraphicsReadLatch)
		m.readLatch = value
		return 0
	}

	return value
}

// Write handles mapper registers, mapped cartridge RAM, and pattern writes.
func (m *Mapper) Write(address uint16, value byte) {
	if address < patternTableEnd {
		m.writeVRAM(address, value)
		return
	}

	if address < cpuWindowBase {
		m.writeRegister(address, value)
		return
	}

	w := m.cpuWindows[int(address-cpuWindowBase)/prgBankSize]
	if w.writable {
		m.MarkFeature(feature.PRGRAM)
		writeWrapped(w.data, w.offset+int(address&prgOffsetMask), value)
	}
}

// ObservePPURegisterWrite records writes to $2000-$2006. It ignores writes to
// $2007 and to the register mirrors at $2008-$3FFF.
func (m *Mapper) ObservePPURegisterWrite(address uint16, value byte) {
	if address >= register.PPU_CTRL && address <= register.PPU_ADDR {
		m.ppuRegisters[address&ppuRegisterMask] = value
		if address == register.PPU_SCROLL {
			m.scrollDirty = true
		}
	}
}

// ClockCPU clocks the IRQ-entry phase and the temporary PPU-read inhibit.
func (m *Mapper) ClockCPU(cycles uint64) {
	for range cycles {
		m.irqPhase = (m.irqPhase + 1) & irqPhaseMask
		if m.irqPending && !m.previousPending {
			m.irqPhase = irqAssertPhase
		}

		m.previousPending = m.irqPending

		if m.readInhibit != 0 {
			m.readInhibit--
		}
	}
}

// State reports the offsets of the effective ROM and RAM windows. It has no
// bus effects.
func (m *Mapper) State() bus.MapperState {
	state := bus.MapperState{
		ID:         m.Cartridge().Mapper,
		Name:       name,
		PrgWindows: make([]int, prgWindowCount),
		ChrWindows: make([]int, chrWindowCount),
	}

	for i := range state.PrgWindows {
		w := m.cpuReadWindow(i + 1)
		if len(w.data) != 0 {
			state.PrgWindows[i] = w.offset % len(w.data) / prgBankSize
		}
	}

	for i, w := range m.ppuWindows {
		state.ChrWindows[i] = w.offset / chrBankSize
	}

	return state
}

// MemorySizes reports the sizes of the real backing memory, including the
// extra nametable page of MXM.
func (m *Mapper) MemorySizes() (int, int) { return len(m.ram), len(m.chrMemory) }

func (m *Mapper) readNameTable(address uint16) (byte, bool) {
	return m.readVRAM(address), true
}

func (m *Mapper) writeNameTable(address uint16, value byte) bool {
	m.writeVRAM(address, value)
	return true
}

func (m *Mapper) cpuReadWindow(slot int) window {
	if m.isMapper801 || slot != alternateSlot || !m.altEnabled {
		return m.cpuWindows[slot]
	}

	data := m.prgROM
	if m.altRAM {
		data = m.ram
	}

	return window{
		data:   data,
		offset: int(m.altBank) * prgBankSize,
		prg:    !m.altRAM,
	}
}

func (m *Mapper) mapCPU(slot int, bank uint16, flags byte) {
	if m.isMapper801 {
		// Only nine bits of the PRG bank register select an address.
		bank &= prgBankMask801
	}

	w := window{
		data: m.prgROM,
		prg:  true,
	}

	if flags&bankSourceRAM != 0 {
		w.data = m.ram
		if m.isMapper801 {
			w.data = w.data[:len(w.data)-ntBankSize]
		}

		w.writable = true
		w.prg = false
	}

	if flags&prgGraphicsLatch != 0 {
		// Graphics-aperture banks use unsigned offsets without wrap. A bank
		// after the end of the memory does not change the current mapping.
		if flags&bankSourceRAM == 0 {
			w.data = m.chrROM
			w.prg = false
		}

		w.offset = int(bank) * prgBankSize
		if w.offset+prgBankSize > len(w.data) {
			return
		}
	} else if len(w.data)/prgBankSize > 0 {
		// A bank value of $8000 or more is a negative 16-bit number. The bank
		// is that number as an unsigned 32-bit value, modulo the bank count.
		// With a power-of-two bank count, bank $FFFF selects the last bank.
		w.offset = int(uint32(int32(int16(bank)))%uint32(len(w.data)/prgBankSize)) * prgBankSize
	}

	m.cpuWindows[slot] = w
}

func (m *Mapper) mapSelectedCPU() {
	slot := int(m.prgSelect)
	if slot == alternateSlot && m.altEnabled {
		// Keep the normal-bank writes for later. Do not replace the active
		// alternate interrupt-side mapping.
		return
	}

	m.mapCPU(slot, m.prgBanks[slot], m.prgFlags[slot])
}

func (m *Mapper) mapAlternateCPU() {
	if m.altEnabled {
		var flags byte
		if m.altRAM {
			flags = bankSourceRAM
		}

		m.mapCPU(alternateSlot, m.altBank, flags)
	} else {
		m.mapCPU(alternateSlot, m.prgBanks[alternateSlot], m.prgFlags[alternateSlot])
	}
}

func newMapper(base *mapperbase.Base, isMapper801 bool) (bus.Mapper, error) {
	cart := base.Cartridge()
	if err := mapperbase.ValidateRAMSizes(cart); err != nil {
		return nil, fmt.Errorf("checking RAM sizes: %w", err)
	}
	if cart.NES2 != nil && cart.NES2.Submapper != 0 {
		return nil, fmt.Errorf("MXM-1 submapper %d is not supported", cart.NES2.Submapper)
	}
	if isMapper801 {
		if err := validateLayout801(cart); err != nil {
			return nil, err
		}
	} else if len(cart.PRG) < prgBankSize || len(cart.PRG)%prgBankSize != 0 {
		return nil, errors.New("MXM-1 requires PRG ROM in 8 KiB banks")
	}

	ramSize := mapperbase.PrgRAMSize(cart)
	chrSize := defaultCHRRAM
	if cart.NES2 != nil {
		chrSize = mapperbase.ChrRAMTotal(cart.NES2.RAMSizes)
	}

	m := &Mapper{
		Base:        base,
		isMapper801: isMapper801,
		prgROM:      cart.PRG,
		chrROM:      cart.CHR,
		ram:         make([]byte, ramSize+ntBankSize),
		chrMemory:   make([]byte, chrSize),
	}

	m.SetName(name)
	m.SetPrgRAM(m.ram)
	m.SetChrRAM(m.chrMemory)
	m.SetPrgWindowSize(prgBankSize)
	m.SetChrWindowSize(chrBankSize)
	if err := m.Base.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}

	for _, id := range declaredFeatures {
		m.DeclareFeature(id)
	}

	m.NameTableMemory().SetReadHook(m.readNameTable)
	m.NameTableMemory().SetWriteHook(m.writeNameTable)
	m.Reset()

	return m, nil
}

func readWrapped(data []byte, offset int, fallback byte) byte {
	if len(data) == 0 {
		return fallback
	}

	return data[offset%len(data)]
}

func writeWrapped(data []byte, offset int, value byte) {
	if len(data) > 0 {
		data[offset%len(data)] = value
	}
}
