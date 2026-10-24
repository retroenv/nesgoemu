// Package mxm implements mapper 800, submapper 0, as the Former Dawn demo uses
// it. Register reads and writes are not symmetric. A state machine follows the
// PPU fetches. The package does not implement expansion audio.
//
// The older House of Truth minimapper (220, submapper 1) uses a different
// register protocol and graphics packing. Its flashcart FPGA configurations
// are evidence for that board. They are not a specification for this mapper.
//
// The CPU banks PRG ROM and WorkRam. It uploads graphics descriptors and
// predecoded tiles into WorkRam. On the PPU side, the mapper finds the normal
// name, attribute, and pattern fetch sequence. It replaces the bytes of these
// fetches with bytes from the tables. The PPU still renders ordinary NES
// pixels. There is no separate framebuffer. The extra graphics come from the
// data that each fetch sees.
//
// This mapper has no public test ROM and no public hardware test result. The
// tests check the register behavior that the code describes. They do not prove
// hardware timing.
package mxm

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
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
	prgMode, prgSelect byte
	prgFlags           [cpuWindowCount]byte
	prgBanks           [cpuWindowCount]uint16
	readLatch          byte
	altEnabled         bool
	altRAM             bool
	altBank            uint16

	chrMode, chrSelect byte
	chrRAM             [chrWindowCount]bool
	chrBanks           [chrWindowCount]uint16

	irqEnabled                     bool
	irqTarget, irqOffset, scanline byte
	irqPending, previousPending    bool
	irqPhase, inhibit              byte

	descriptorEnabled               bool
	attributeMode, descriptorSelect byte
	descriptorRAM, patternRAM       [ntWindowCount]bool
	descriptorBanks                 [ntWindowCount]uint16
	manualScroll, vertical          byte

	ntEnabled        bool
	ntMode, ntSelect byte
	ntRAM            [ntWindowCount]bool
	ntBanks          [ntWindowCount]uint16

	wide, spriteMode, second, spriteSecond bool

	writeBank uint16

	copySource, copyDestination byte
	copyFrom, copyTo            uint16

	metatileEnabled               bool
	metatileTables                [ntWindowCount]byte
	metatileStride, metatileFrame byte

	remapEnabled bool
	remapTable   [remapTableSize]byte
	remapCursor  uint16
	remapPage    byte

	streamCursor byte

	ppuRegisters [ppuRegisterCount]byte
	scrollDirty  bool

	lastAddress         uint16
	repeated            byte
	frameStart, inFrame bool
	nameReads           uint32
	previousNameReads   byte

	metatilePhase                              byte
	metatileLow, metatileHigh, metatilePalette byte

	descriptorPhase      byte
	tileOffset           uint16
	tileTable, tileFineY byte
}

// Mapper supplies the CPU windows, PPU fetch substitutions, and scanline IRQ
// of MXM.
type Mapper struct {
	*mapperbase.Base
	registers

	prgROM, chrROM, ram, chrMemory []byte

	cpuWindows [cpuWindowCount]window
	ppuWindows [chrWindowCount]window
	ntWindows  [ntWindowCount]window

	dummyAttribute bool
}

// New constructs submapper 0 of the demo with the RAM sizes from the NES 2.0
// header. It rejects other submappers before it initializes devices or installs
// PPU hooks.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	cart := base.Cartridge()
	if cart.NES2 != nil && cart.NES2.Submapper != 0 {
		return nil, fmt.Errorf("MXM submapper %d is not supported", cart.NES2.Submapper)
	}
	if len(cart.PRG) < prgBankSize || len(cart.PRG)%prgBankSize != 0 {
		return nil, errors.New("MXM requires PRG ROM in 8 KiB banks")
	}

	ramSize := mapperbase.PrgRAMSize(cart)
	chrSize := defaultCHRRAM
	if cart.NES2 != nil {
		chrSize = mapperbase.ChrRAMTotal(cart.NES2.RAMSizes)
	}

	m := &Mapper{
		Base:      base,
		prgROM:    cart.PRG,
		chrROM:    cart.CHR,
		ram:       make([]byte, ramSize+ntBankSize),
		chrMemory: make([]byte, chrSize),
	}

	m.SetName(name)
	m.SetPrgRAM(m.ram)
	m.SetChrRAM(m.chrMemory)
	m.SetPrgWindowSize(prgBankSize)
	m.SetChrWindowSize(chrBankSize)
	m.Base.Initialize()

	for _, id := range declaredFeatures {
		m.DeclareFeature(id)
	}

	m.NameTableMemory().SetReadHook(m.readNameTable)
	m.NameTableMemory().SetWriteHook(m.writeNameTable)
	m.Reset()

	return m, nil
}

// Reset restores the mapper registers. It does not erase cartridge RAM.
func (m *Mapper) Reset() {
	m.dummyAttribute = false
	m.registers = registers{
		prgBanks:        [cpuWindowCount]uint16{0, 0, 1, 2, lastPRGBank},
		metatileStride:  metatileDefaultStride,
		copySource:      copySourcePRG,
		copyDestination: copySourceRAM,
	}

	if len(m.ram) > ntBankSize {
		m.prgFlags[0] = bankSourceRAM
	}

	for slot, bank := range m.prgBanks {
		m.mapCPU(slot, bank, m.prgFlags[slot])
	}

	for slot := range m.ppuWindows {
		m.ppuWindows[slot] = window{
			data:   m.chrROM,
			offset: (initialCHRBank + slot) * chrBankSize,
		}
	}

	m.ppuWindows[initialCHRExceptionSlot].offset = initialCHRExceptionBank * chrBankSize
	m.ntWindows = [ntWindowCount]window{}
	m.SetMapperIRQ(false)
}

// TickPPU marks the second dummy fetch in each sprite slot (dots 259, 267, ...,
// 315). The demo counts 42 tile-name reads for each scanline. nesgoemu makes a
// dummy nametable read at these dots, as the hardware does. The mapper treats
// it as an attribute read, so the phase gate keeps the timing that the demo
// expects. This applies only to this mapper.
func (m *Mapper) TickPPU(cycle, scanLine int, rendering bool) {
	m.dummyAttribute = rendering && (scanLine < visibleScanlines || scanLine == preRenderScanline) &&
		cycle >= firstDummyAttribute && cycle <= lastDummyAttribute && (cycle-firstDummyAttribute)%spriteFetchSlotDots == 0
}

// Read handles CPU memory and pattern-table reads. Nametable reads use the
// nametable hook, so they are not confused with the CPU register space at
// $2000.
func (m *Mapper) Read(address uint16) byte {
	if address < patternTableEnd {
		return m.readVRAM(address)
	}

	if address == nmiVectorLow || address == nmiVectorHigh {
		// The demo uses reads of the NMI vector as the frame boundary for the
		// scanline detector. Debugger peeks must not cause this side effect.
		m.inFrame = false
		m.lastAddress = 0
		m.repeated = 0
		m.scanline = 0
		m.irqPending = false
		m.SetMapperIRQ(false)
	}

	if address < cpuWindowBase {
		return m.readRegister(address)
	}

	slot := int(address-cpuWindowBase) / prgBankSize
	w := m.cpuWindows[slot]
	if slot == alternateSlot && m.altEnabled {
		data := m.prgROM
		if m.altRAM {
			data = m.ram
		}

		w = window{
			data:   data,
			offset: int(m.altBank) * prgBankSize,
		}
	}

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

		if m.inhibit != 0 {
			m.inhibit--
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

	for i, w := range m.cpuWindows[1:] {
		state.PrgWindows[i] = w.offset / prgBankSize
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

func (m *Mapper) mapCPU(slot int, bank uint16, flags byte) {
	w := window{
		data: m.prgROM,
		prg:  true,
	}

	if flags&bankSourceRAM != 0 {
		w.data = m.ram
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
