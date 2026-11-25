package mxm1

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/renderstate"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
)

const (
	// The NES 2.0 header of the N8 Pro release specifies 2 MiB of PRG ROM
	// and 256 KiB of CHR ROM. See https://somethingnerdy.com/downloads/.
	mapper128CHRSize = 256 * 1024
	mapper128PRGSize = 2 * 1024 * 1024

	descriptorPaletteOffset = 1
	descriptorPaletteShift  = 6

	framePaletteOffset = 0x3c00
	framePaletteSize   = 16
	framePCMOffset     = framePaletteOffset + framePaletteSize
	frameSize          = 0x8000
	pcmSamplesPerFrame = 735

	frameSlotCount    = 4
	mapper128WorkSize = frameSlotCount*frameSize + ntWindowCount*ntBankSize

	reg128FIFO           = 0x40f0
	reg128FIFOStatus     = 0x40f1
	reg128TransferMode   = 0x5095
	reg128TransferStatus = 0x5096
	reg128TransferEnable = 0x5097
	reg128MediaSlot      = 0x5098
	reg128Palette        = 0x5099
	reg128PCMControl     = 0x50b0
	reg128PCMData        = 0x50b9

	transferReady = 0x11

	// Simulation of the release FPGA configuration gives an IRQ every 1131
	// ticks of the 50 MHz clock. Each pulse lasts approximately 300 ticks.
	n8ProClockHz      = 50_000_000
	pcmIRQPeriodTicks = 1131
	pcmIRQWidthTicks  = 300

	// Use integer phase units to keep the fractional part of each period.
	pcmIRQIncrement = n8ProClockHz * nes.NTSCCPUClockDenominator
	pcmIRQPeriod    = pcmIRQPeriodTicks * nes.NTSCCPUClockNumerator
	pcmIRQWidth     = pcmIRQWidthTicks * nes.NTSCCPUClockNumerator

	pcmCaptureEnable = 0x20
	pcmIdleSample    = 0x20
)

// New128 constructs the NES 2.0 mapper 128, submapper 0, with file streaming.
func New128(base *mapperbase.Base) (bus.Mapper, error) {
	if err := validate128Layout(base.Cartridge()); err != nil {
		return nil, err
	}

	openMedia := base.CartridgeMediaOpener()
	if openMedia == nil {
		return nil, errors.New("mapper 128 requires a cartridge media opener")
	}

	m := &mapper128{
		Base: base,

		work: make([]byte, mapper128WorkSize),

		openMedia: openMedia,
	}

	m.SetName(name)
	m.SetPrgRAM(nil)
	m.SetChrRAM(nil)
	m.SetPrgWindowSize(prgBankSize)
	m.SetChrWindowSize(chrBankSize)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper 128: %w", err)
	}

	m.NameTableMemory().SetReadHook(m.readNameTable)
	m.NameTableMemory().SetWriteHook(m.writeNameTable)
	m.Reset()

	return m, nil
}

// mapper128 implements the registers used by the N8 Pro release of Bad Apple
// Ultimate NES Demake. The release ROM and FPGA configuration are the
// references for register behavior. See https://somethingnerdy.com/downloads/.
type mapper128 struct {
	*mapperbase.Base

	work []byte

	prgLow    byte
	prgSelect byte

	chrHigh   byte
	chrSelect byte

	backgroundFetch bool
	descriptor      graphicsBanks
	nameAddress     uint16
	nametable       graphicsBanks

	paletteCursor byte
	paletteSlot   byte

	pcm  pcmPlayback
	slot byte

	fifo      mediaFIFO
	openMedia bus.CartridgeMediaOpener
}

// pcmPlayback holds the sample cursor and the IRQ timer state.
type pcmPlayback struct {
	cursor    int
	enabled   bool
	irqActive bool
	irqPhase  uint64
	sample    byte
}

// Reset restores the bank, FIFO, and PCM registers.
func (m *mapper128) Reset() {
	m.prgSelect = 0
	m.prgLow = 0
	m.chrSelect = 0
	m.chrHigh = 0
	m.nametable = graphicsBanks{}
	m.descriptor = graphicsBanks{}
	m.backgroundFetch = false
	m.nameAddress = 0

	m.paletteCursor = 0
	m.paletteSlot = 0

	m.slot = frameSlotCount
	m.pcm = pcmPlayback{
		irqPhase: pcmIRQWidth,
		sample:   pcmIdleSample,
	}
	m.SetMapperIRQ(false)

	m.fifo = mediaFIFO{}
	clear(m.work)

	for window := range prgWindowCount - 1 {
		m.SetPrgWindow(window, window)
	}

	m.SetPrgWindow(prgWindowCount-1, -1)

	for window := range chrWindowCount {
		m.SetChrWindow(window, window)
	}
}

// Read returns PRG, PPU pattern data, or a register value.
func (m *mapper128) Read(address uint16) byte {
	if value, ok := m.readPort(address, false); ok {
		return value
	}

	switch {
	case m.streamedPattern(address):
		return m.work[m.backgroundPatternOffset(address)]

	case address >= cpuWindowBase && address <= mapperbase.PRGRAMEnd:
		return m.Cartridge().PRG[m.lowPRGOffset(address)]

	default:
		return m.Base.Read(address)
	}
}

// Write handles the registers used by the release.
func (m *mapper128) Write(address uint16, value byte) {
	switch address & registerGroupMask {
	case regPRGMode, regCHRMode:
		m.writeBanks(address, value)

	case regDescriptorMode:
		m.descriptor.write(address, value)

	case regNametableMode:
		m.nametable.write(address, value)

	default:
		m.writeMedia(address, value)
	}
}

// TickPPU identifies the background fetch dots. The NES still supplies the
// nametable address, fine Y, and pattern plane for each fetch.
func (m *mapper128) TickPPU(cycle, scanLine int, rendering bool) {
	visibleDot := cycle >= renderstate.FirstVisibleDot && cycle <= renderstate.LastVisibleDot
	prefetchDot := cycle >= renderstate.FirstBackgroundPrefetchDot && cycle <= renderstate.LastBackgroundPrefetchDot
	renderLine := scanLine < renderstate.VisibleScanlines || scanLine == renderstate.PreRenderScanline
	m.backgroundFetch = rendering && renderLine && (visibleDot || prefetchDot)
}

// ClockCPU runs the PCM IRQ timer. The release ROM's IRQ handler reads
// the mapper sample register at $50B9 and writes the sample to the APU
// DAC register at $4011.
func (m *mapper128) ClockCPU(cycles uint64) {
	if !m.pcm.enabled {
		return
	}
	for range cycles {
		m.pcm.irqPhase += pcmIRQIncrement
		if m.pcm.irqPhase >= pcmIRQPeriod {
			m.pcm.irqPhase -= pcmIRQPeriod
			if m.slot < frameSlotCount && m.pcm.cursor < pcmSamplesPerFrame {
				m.pcm.sample = m.work[int(m.slot)*frameSize+framePCMOffset+m.pcm.cursor]
				m.pcm.cursor++
			}
		}

		active := m.pcm.irqPhase < pcmIRQWidth
		if active != m.pcm.irqActive {
			m.pcm.irqActive = active
			m.SetMapperIRQ(active)
		}
	}
}

// InspectCPU returns the CPU data without changing the FIFO or palette cursor.
func (m *mapper128) InspectCPU(address uint16) (byte, bool) {
	if value, ok := m.readPort(address, true); ok {
		return value, true
	}

	if offset, ok := m.PRGROMOffset(address); ok {
		return m.Cartridge().PRG[offset], true
	}
	return 0, false
}

// PRGROMOffset returns the ROM offset of the current CPU mapping.
func (m *mapper128) PRGROMOffset(address uint16) (int, bool) {
	if address >= cpuWindowBase && address <= mapperbase.PRGRAMEnd {
		return m.lowPRGOffset(address), true
	}
	return m.Base.PRGROMOffset(address)
}

// PRGLowBank returns the number of the PRG ROM bank in the $6000 window.
func (m *mapper128) PRGLowBank() int {
	return int(m.prgLow)
}

// MapperError returns an error from the companion file transfer.
func (m *mapper128) MapperError() error {
	return m.fifo.transferErr
}

// PPUReadMapping reports the offset of the current pattern byte.
func (m *mapper128) PPUReadMapping(address uint16) (bus.GraphicsMapping, bool) {
	if m.streamedPattern(address) {
		return bus.GraphicsMapping{
			Memory: bus.GraphicsCHRRAM,
			Offset: m.backgroundPatternOffset(address),
		}, true
	}
	return m.Base.PPUReadMapping(address)
}

// MemorySizes includes the four slots of streamed graphics RAM.
func (m *mapper128) MemorySizes() (int, int) {
	return 0, len(m.work)
}

// InspectMemory reads the current graphics data without a PPU fetch.
func (m *mapper128) InspectMemory(address uint16) (byte, bool) {
	switch {
	case address < nes.PatternTableEndExclusive:
		return m.Base.Read(address), true

	case address < register.PALETTE_START:
		return m.inspectNameTable(address)

	default:
		return 0, false
	}
}

func (m *mapper128) lowPRGOffset(address uint16) int {
	return m.PRGLowBank()*prgBankSize + int(address&prgOffsetMask)
}

func (m *mapper128) readPort(address uint16, peek bool) (byte, bool) {
	switch address {
	case reg128FIFO:
		if peek {
			return m.fifo.response[0], true
		}
		return m.fifo.readResponse(), true

	case reg128FIFOStatus:
		if m.fifo.responseLength == 0 {
			return fifoStatusBase | fifoResponseEmptyBit, true
		}
		return fifoStatusBase, true

	case reg128TransferStatus:
		return transferReady, true

	case reg128Palette:
		value := m.paletteByte()
		if !peek {
			m.paletteCursor = (m.paletteCursor + 1) % framePaletteSize
			if m.paletteCursor == 0 {
				m.paletteSlot = (m.paletteSlot + 1) % frameSlotCount
			}
		}

		return value, true

	case reg128PCMData:
		return m.pcm.sample, true

	default:
		return read5000(address, n8ProCompatibilityReadback)
	}
}

func (m *mapper128) writeBanks(address uint16, value byte) {
	switch address {
	case regPRGSelect:
		m.prgSelect = value & prgSelectMask

	case regPRGBankLow:
		if m.prgSelect == 0 {
			m.prgLow = value
		} else {
			m.SetPrgWindow(int(m.prgSelect)-1, int(value))
		}

	case regCHRSelect:
		m.chrSelect = value & chrSelectMask

	case regCHRBankHigh:
		m.chrHigh = value

	case regCHRBankLow:
		m.SetChrWindow(int(m.chrSelect), int(m.chrHigh)<<8|int(value))
	}
}

func (m *mapper128) readNameTable(address uint16) (byte, bool) {
	offset := address & ntOffsetMask
	if m.backgroundFetch && m.descriptor.enabled {
		if offset < attributeTableOffset {
			m.nameAddress = address
		} else {
			tile := m.nameAddress & ntOffsetMask
			bank := m.descriptor.banks[nametableSlot(m.nameAddress)]
			index := descriptorOffset(bank, tile) + descriptorPaletteOffset
			if index < len(m.work) {
				return mapperbase.RepeatAttributePalette(m.work[index] >> descriptorPaletteShift), true
			}
		}
	}

	return m.inspectNameTable(address)
}

func (m *mapper128) inspectNameTable(address uint16) (byte, bool) {
	index, ok := m.nameTableOffset(address)
	if !ok {
		return 0, false
	}
	return m.work[index], true
}

func (m *mapper128) writeNameTable(address uint16, value byte) bool {
	index, ok := m.nameTableOffset(address)
	if ok {
		m.work[index] = value
	}

	return ok
}

func (m *mapper128) nameTableOffset(address uint16) (int, bool) {
	bank := m.nametable.banks[nametableSlot(address)]
	index := int(bank)*ntBankSize + int(address&ntOffsetMask)
	return index, m.nametable.enabled && index < len(m.work)
}

func (m *mapper128) streamedPattern(address uint16) bool {
	return address < nes.PatternTableEndExclusive && m.backgroundFetch && m.descriptor.enabled && m.slot < frameSlotCount
}

func (m *mapper128) backgroundPatternOffset(address uint16) int {
	// The demo streams its tiles in nametable order. The PPU pattern fetch
	// supplies the fine Y and the plane in the low four address bits.
	const tilePatternBytes = 16
	return int(m.slot)*frameSize +
		int(m.nameAddress&ntOffsetMask)*tilePatternBytes + int(address&(tilePatternBytes-1))
}

func (m *mapper128) paletteByte() byte {
	return m.work[int(m.paletteSlot)*frameSize+framePaletteOffset+int(m.paletteCursor)]
}

// validate128Layout checks the NES 2.0 cartridge layout for the supported release.
func validate128Layout(cart *cartridge.Cartridge) error {
	if cart.NES2 == nil || cart.NES2.Submapper != 0 || cart.Mirror != cartridge.Mirror4 ||
		len(cart.PRG) != mapper128PRGSize || len(cart.CHR) != mapper128CHRSize ||
		cart.NES2.RAMSizes != (cartridge.RAMSizes{}) || len(cart.Trainer) != 0 {

		return errors.New("mapper 128 requires the Bad Apple NES 2.0 submapper 0 layout")
	}
	return nil
}
