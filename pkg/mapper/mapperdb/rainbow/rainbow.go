// Package rainbow implements the Rainbow mapper (iNES 682) by Broke Studio.
// It provides PRG/CHR banking, 8 KB FPGA-RAM, flash commands, cartridge saves,
// mapper snapshots, scanline/CPU IRQs, and basic ESP message commands.
// Expansion audio, host networking, and ESP file commands remain incomplete.
//
// See the official Rainbow mapper specification:
// https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md
package rainbow

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
)

const (
	patternTableEnd      = 0x2000
	fpgaRAMSize          = 8 * 1024
	fpgaFixedRAMOffset   = 0x1800
	fpgaFixedStart       = 0x4800
	fpgaFixedEnd         = 0x4FFF
	fpgaBankedStart      = 0x5000
	fpgaBankedEnd        = 0x5FFF
	prgRAMStart          = 0x6000
	prgRAMEnd            = 0x7FFF
	prgROMStart          = 0x8000
	nmiVectorLower       = 0xFFFA
	nmiVectorUpper       = 0xFFFB
	irqVectorLower       = 0xFFFE
	irqVectorUpper       = 0xFFFF
	cartridgeRAMBankSize = 8 * 1024
	defaultRAMBlockSize  = 32 * 1024
	maximumPRGRAMSize    = 512 * 1024
	platformEmulatorV    = 0x21 // Emulator platform, mapper revision 1.
)

// Mapper implements the Rainbow mapper.
type Mapper struct {
	*mapperbase.Base

	// PRG banking: high banks cover $8000-$FFFF, low banks cover $6000-$7FFF.
	// Each 16-bit register combines upper (written via $4108-$410F / $4106-$4107)
	// and lower (written via $4118-$411F / $4116-$4117) bytes.
	prgMode    byte
	prgRAMMode byte
	highBanks  [prgHighBankCount]uint16 // $8000-$FFFF: registers $4108-$410F/$4118-$411F
	lowBanks   [prgLowBankCount]uint16  // $6000-$7FFF: registers $4106-$4107/$4116-$4117

	// CHR banking.
	chrMode       byte
	chrSource     byte
	spriteExtMode bool
	windowEnabled bool
	chrBanks      [chrBankCount]uint16 // combined upper ($4130-$413F) + lower ($4140-$414F)

	prgFlash flash
	chrFlash flash
	prgROM   []byte
	prgRAM   []byte
	chrROM   []byte
	chrRAM   []byte

	fpgaRAM        [fpgaRAMSize]byte
	fpgaBankSelect byte
	fpgaAutoAddr   uint16
	fpgaAutoInc    byte

	ntBank    [ntSlotCount]byte
	ntControl [ntSlotCount]byte
	fillTile  byte
	fillAttr  byte

	ppuBus   ppuBusState
	scanIRQ  scanlineIRQState
	cycleIRQ cycleIRQState

	nmiVectorEnabled bool
	irqVectorEnabled bool
	nmiAddr          uint16
	irqAddr          uint16

	spriteBankLower [spriteCount]byte
	spriteBankUpper byte
	oamSlowPage     byte
	oamExtPage      byte
	oamLimit        byte
	oamCode         [oamRoutineSize]byte
	oamCodeLocked   bool

	windowSplitRegs [windowSplitRegisterCount]byte // $4170-$4175
	ppuCycle        int                            // last PPU cycle from TickPPU, used for window routing
	ppuScanLine     int                            // last PPU scanline from TickPPU, used for window routing

	bgExtModeOffset byte // $4121

	activeSpriteIndex int    // -1 = no active sprite fetch; ≥0 = OAM index being evaluated
	activeSpriteSize  int    // sprite height (8 or 16) set by SetActiveSpriteExt
	bgExtActive       bool   // true when the current tile's nametable slot has BG ext mode enabled
	bgExtData         byte   // ext data byte for the current tile read from FPGA-RAM
	bgTileSlotIdx     int    // nametable slot index of the most recently cached tile fetch
	bgTileSlotOffset  uint16 // tile offset within bgTileSlotIdx for the cached fetch

	espEnabled    bool
	wifiIrqEnable bool
	wifiRegs      [espRegisterCount]byte // $4190-$4194
	esp           espState
}

// New creates and initializes a new Rainbow mapper instance.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	cart := base.Cartridge()

	// Derive PRG-RAM size from iNES header (cart.RAM = number of 8 KB banks).
	// Use a size from 32 KB through 512 KB, aligned to 32 KB.
	prgRAMSize := max(int(cart.RAM)*cartridgeRAMBankSize, defaultRAMBlockSize)
	prgRAMSize = min(prgRAMSize, maximumPRGRAMSize)
	prgRAMSize = (prgRAMSize + defaultRAMBlockSize - 1) / defaultRAMBlockSize * defaultRAMBlockSize
	chrRAMSize := defaultRAMBlockSize
	if metadata := cart.NES2; metadata != nil {
		sizes := metadata.RAMSizes
		prgRAMSize = sizes.PRGVolatile + sizes.PRGNonvolatile
		chrRAMSize = sizes.CHRVolatile + sizes.CHRNonvolatile
	}

	m := &Mapper{
		Base:   base,
		prgROM: append([]byte(nil), cart.PRG...),
		chrROM: append([]byte(nil), cart.CHR...),
		prgRAM: make([]byte, prgRAMSize),
		chrRAM: make([]byte, chrRAMSize),
	}
	m.SetName("Rainbow")
	m.declareFeatures()
	m.Base.Initialize()

	m.applyPowerUpDefaults()
	base.NameTableMemory().SetReadHook(m.readNametable)
	base.NameTableMemory().SetWriteHook(m.writeNametable)

	return m, nil
}

// Read returns a byte from the given address.
func (m *Mapper) Read(address uint16) uint8 {
	switch {
	case address < patternTableEnd:
		return m.readCHR(address)

	case address >= oamRoutineStart && address < fpgaFixedStart:
		return m.readOAMRoutine(address)

	case address >= registerStart && address <= registerEnd:
		return m.readRegister(address)

	case address >= fpgaFixedStart && address <= fpgaFixedEnd:
		m.MarkFeature(feature.FPGARAM)
		return m.fpgaRAM[fpgaFixedRAMOffset+(address-fpgaFixedStart)]

	case address >= fpgaBankedStart && address <= fpgaBankedEnd:
		return m.readFPGA(address)

	case address >= prgRAMStart && address <= prgRAMEnd:
		return m.readPRGRAM(address)

	case address >= nmiVectorLower:
		return m.readVector(address)

	case address >= prgROMStart:
		return m.readPRG(address)

	default:
		return 0
	}
}

// Write stores a byte at the given address.
func (m *Mapper) Write(address uint16, value uint8) {
	switch {
	case address < patternTableEnd:
		m.writeCHR(address, value)

	case address >= registerStart && address <= registerEnd:
		m.writeRegister(address, value)

	case address >= fpgaFixedStart && address <= fpgaFixedEnd:
		m.MarkFeature(feature.FPGARAM)
		m.fpgaRAM[fpgaFixedRAMOffset+(address-fpgaFixedStart)] = value

	case address >= fpgaBankedStart && address <= fpgaBankedEnd:
		m.writeFPGA(address, value)

	case address >= prgRAMStart && address <= prgRAMEnd:
		m.writePRGRAM(address, value)

	case address >= prgROMStart:
		m.writePRG(address, value)
	}
}

// MemorySizes returns the mapper PRG RAM and CHR RAM sizes in bytes.
func (m *Mapper) MemorySizes() (prgRAM, chrRAM int) {
	return len(m.prgRAM), len(m.chrRAM)
}

// Specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#vector-redirection-416b-416f-write-only
func (m *Mapper) readVector(address uint16) uint8 {
	if address == nmiVectorLower || address == nmiVectorUpper {
		m.scanIRQ.inFrame = false
		m.scanIRQ.counter = 0
		m.scanIRQ.pending = false
		m.scanIRQ.readyToFire = false
		// NMI ends the frame and breaks the repeated-read detection sequence.
		m.ppuBus.lastAddress, m.ppuBus.repeated = 0, 0
		m.updateIRQStatus()
	}

	switch address {
	case nmiVectorLower:
		if m.nmiVectorEnabled {
			return byte(m.nmiAddr)
		}
	case nmiVectorUpper:
		if m.nmiVectorEnabled {
			return byte(m.nmiAddr >> registerByteShift)
		}
	case irqVectorLower:
		if m.irqVectorEnabled {
			return byte(m.irqAddr)
		}
	case irqVectorUpper:
		if m.irqVectorEnabled {
			return byte(m.irqAddr >> registerByteShift)
		}
	}

	return m.readPRG(address)
}

func (m *Mapper) applyPowerUpDefaults() {
	// $4100: PRG mode 0, PRG-RAM mode 0.
	m.prgMode = 0
	m.prgRAMMode = 0

	// $4120: CHR mode 0, CHR-ROM source.
	m.chrMode = 0
	m.chrSource = 0
	m.ntBank = [ntSlotCount]byte{0, 0, 1, 1, 0}
	m.ntControl[ntWindowSlot] = ntWindowSource
	m.oamSlowPage = oamDefaultSlowPage
	m.oamExtPage = oamDefaultExtendedPage
	m.oamLimit = oamDefaultLimit

	// $4153: scanline IRQ offset 135.
	m.scanIRQ.offset = scanIRQDefaultOffset

	// Default FPGA auto-increment to 1.
	m.fpgaAutoInc = 1

	// No active sprite fetch at power-up.
	m.activeSpriteIndex = -1
}
