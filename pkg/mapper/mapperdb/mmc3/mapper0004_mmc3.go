package mmc3

import (
	"fmt"
	"math"

	"github.com/retroenv/nesgoemu/internal/stateio"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

/*
Boards: TGROM, TKROM, TLROM, TSROM, TVROM, TR1ROM
Revision: Sharp MMC3, NES 2.0 submapper 0
PRG ROM capacity: 512K
PRG ROM window: 8K + 8K + 8K fixed + 8K fixed
PRG RAM capacity: 8K
CHR capacity: 256K
CHR window: 2K + 2K + 1K + 1K + 1K + 1K
*/

const (
	chrWindowSize = 0x0400 // 1K
	prgWindowSize = 0x2000 // 8K

	// The register address decodes bits 14, 13, and 0 of the written address.
	registerDecodeMask = 0xE001

	regBankSelect = 0x8000 // bank select and mode
	regBankData   = 0x8001 // bank data
	regMirroring  = 0xA000 // mirroring
	regPrgRAM     = 0xA001 // PRG RAM enable and protect
	regIRQLatch   = 0xC000 // IRQ latch
	regIRQReload  = 0xC001 // IRQ reload
	regIRQDisable = 0xE000 // IRQ disable
	regIRQEnable  = 0xE001 // IRQ enable

	bankSelectMask = 0x07 // $8000 bits 0-2
	prgModeMask    = 0x40 // $8000 bit 6
	chrModeMask    = 0x80 // $8000 bit 7

	chrPairBankMask      = 0xfe
	chrPairSecondBankBit = 0x01
	prgBankMask          = 0x3f

	wramEnableBit       = 0x80
	wramWriteProtectBit = 0x40

	chrPairBank0Register   = 0
	chrPairBank1Register   = 1
	chrSingleBank0Register = 2
	chrSingleBank1Register = 3
	chrSingleBank2Register = 4
	chrSingleBank3Register = 5
	prgBank0Register       = 6
	prgBank1Register       = 7
	bankRegisterCount      = 8

	ppuA12Mask = 0x1000

	// Hardware filters A12 with three falling edges of M2. This filter uses
	// nine elapsed PPU dots as an approximation. CPU/PPU phase is not modeled.
	// https://www.nesdev.org/wiki/MMC3#IRQ_Specifics
	// https://github.com/furrtek/SiliconRE/tree/master/Nintendo/MMC3C
	a12FilterDots = 9
)

// New returns a new MMC3 mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	if err := validateCartridge(base); err != nil {
		return nil, err
	}

	m := &mapperMMC3{Base: base}
	m.SetName("MMC3")
	m.DeclareFeature(feature.CHRBanking)
	m.DeclareFeature(feature.Mirroring)
	m.DeclareFeature(feature.PRGBanking)
	m.DeclareFeature(feature.ScanlineIRQ)
	m.SetChrWindowSize(chrWindowSize)
	m.SetPrgWindowSize(prgWindowSize)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}

	m.wramReadHook = m.AddReadHook(mapperbase.PRGRAMStart, mapperbase.PRGRAMEnd, m.readWram)
	m.wramWriteHook = m.AddWriteHook(mapperbase.PRGRAMStart, mapperbase.PRGRAMEnd, m.writeWram)
	m.AddReadHook(0, nes.PatternTableEndExclusive-1, m.observePatternRead).SetProxyOnly(true)
	m.AddWriteHook(0, nes.PatternTableEndExclusive-1, m.observePatternWrite).SetProxyOnly(true)
	m.AddWriteHook(nes.CodeBaseAddress, math.MaxUint16, m.writeRegister)

	// $A000 bit 0: 0 selects vertical, 1 selects horizontal mirroring.
	translation := mapperbase.MirrorModeTranslation{
		0: cartridge.MirrorVertical,
		1: cartridge.MirrorHorizontal,
	}
	m.SetMirrorModeTranslation(translation)
	m.NameTableMemory().SetReadHook(m.observeNameTableRead)
	m.NameTableMemory().SetWriteHook(m.observeNameTableWrite)

	m.applyPowerOnState()

	return m, nil
}

type mapperMMC3 struct {
	*mapperbase.Base

	registers  [bankRegisterCount]byte
	bankSelect byte
	prgMode    bool
	chrMode    bool

	wramEnabled      bool
	wramWriteProtect bool
	wramReadHook     mapperbase.Hook
	wramWriteHook    mapperbase.Hook

	irqLatch   byte
	irqCounter byte
	irqReload  bool
	irqEnabled bool

	a12Low     bool
	a12LowDots byte // Elapsed PPU dots with A12 low, up to a12FilterDots.
}

// TickPPU advances the A12 pulse filter by one PPU dot.
func (m *mapperMMC3) TickPPU(_, _ int, _ bool) {
	if !m.a12Low || m.a12LowDots >= a12FilterDots {
		return
	}

	m.a12LowDots++
}

// SnapshotFormat identifies the direct mapper state format.
func (*mapperMMC3) SnapshotFormat() string { return "mmc3-mapperMMC3-v1" }

// TransferState transfers registers and mutable cartridge memory.
func (m *mapperMMC3) TransferState(archive *stateio.Archive) error {
	if err := m.Base.TransferState(archive); err != nil {
		return fmt.Errorf("transferring mmc3: %w", err)
	}

	if err := archive.Values(&m.registers, &m.bankSelect, &m.prgMode, &m.chrMode, &m.wramEnabled, &m.wramWriteProtect, &m.irqLatch, &m.irqCounter, &m.irqReload, &m.irqEnabled, &m.a12Low, &m.a12LowDots); err != nil {
		return fmt.Errorf("transferring mmc3: %w", err)
	}
	if archive.Reading() {
		m.wramReadHook.SetProxyOnly(m.wramEnabled)
		m.wramWriteHook.SetProxyOnly(m.wramEnabled && !m.wramWriteProtect)
	}

	return nil
}

func (m *mapperMMC3) writeRegister(address uint16, value uint8) error {
	switch address & registerDecodeMask {
	case regBankSelect:
		// Bits 0-2 select the bank register, bit 6 selects the PRG mode,
		// and bit 7 selects the CHR mode.
		m.bankSelect = value & bankSelectMask
		m.prgMode = value&prgModeMask != 0
		m.chrMode = value&chrModeMask != 0
		m.applyPRGMapping()
		m.applyCHRMapping()

	case regBankData:
		m.writeBankRegister(value)

	case regMirroring:
		if m.Cartridge().Mirror == cartridge.Mirror4 {
			return nil
		}

		m.MarkFeature(feature.Mirroring)
		if err := m.SetNameTableMirrorModeIndex(value & 1); err != nil {
			return fmt.Errorf("setting MMC3 mirror mode: %w", err)
		}

	case regPrgRAM:
		// Bit 7 enables the 8K PRG RAM, bit 6 protects it from writes.
		m.wramEnabled = value&wramEnableBit != 0
		m.wramWriteProtect = value&wramWriteProtectBit != 0
		m.wramReadHook.SetProxyOnly(m.wramEnabled)
		m.wramWriteHook.SetProxyOnly(m.wramEnabled && !m.wramWriteProtect)

	case regIRQLatch:
		m.MarkFeature(feature.ScanlineIRQ)
		m.irqLatch = value

	case regIRQReload:
		m.MarkFeature(feature.ScanlineIRQ)
		m.irqCounter = 0
		m.irqReload = true

	case regIRQDisable:
		m.MarkFeature(feature.ScanlineIRQ)
		m.irqEnabled = false
		m.SetMapperIRQ(false)

	case regIRQEnable:
		m.MarkFeature(feature.ScanlineIRQ)
		m.irqEnabled = true
	}

	return nil
}

func (m *mapperMMC3) writeBankRegister(value uint8) {
	// Registers 0 and 1 select 2 KiB CHR banks, which can only be even
	// numbered, so their low bit is ignored.
	// https://www.nesdev.org/wiki/MMC3#CHR_Banks
	if m.bankSelect <= chrPairBank1Register {
		value &= chrPairBankMask
	} else if m.bankSelect >= prgBank0Register {
		value &= prgBankMask
	}

	m.registers[m.bankSelect] = value

	if m.bankSelect <= chrSingleBank3Register {
		m.MarkFeature(feature.CHRBanking)
		m.applyCHRMapping()
		return
	}

	m.MarkFeature(feature.PRGBanking)
	m.applyPRGMapping()
}

func (m *mapperMMC3) applyPRGMapping() {
	if m.prgMode {
		// $8000: bank fixed to the second-last bank.
		m.SetPrgWindow(0, -2)
		m.SetPrgWindow(1, int(m.registers[prgBank1Register]))
		m.SetPrgWindow(2, int(m.registers[prgBank0Register]))
	} else {
		m.SetPrgWindow(0, int(m.registers[prgBank0Register]))
		m.SetPrgWindow(1, int(m.registers[prgBank1Register]))
		m.SetPrgWindow(2, -2)
	}

	// $E000: bank fixed to the last bank.
	m.SetPrgWindow(3, -1)
}

// applyCHRMapping maps the eight 1K CHR windows from the bank registers.
// In mode 0, registers 0 and 1 select two 2K windows at $0000-$0FFF, and
// registers 2 to 5 select four 1K windows at $1000-$1FFF. Mode 1 swaps the
// two regions.
func (m *mapperMMC3) applyCHRMapping() {
	switch m.chrMode {
	case false:
		m.SetChrWindow(0, int(m.registers[chrPairBank0Register]&chrPairBankMask))
		m.SetChrWindow(1, int(m.registers[chrPairBank0Register]|chrPairSecondBankBit))
		m.SetChrWindow(2, int(m.registers[chrPairBank1Register]&chrPairBankMask))
		m.SetChrWindow(3, int(m.registers[chrPairBank1Register]|chrPairSecondBankBit))
		m.SetChrWindow(4, int(m.registers[chrSingleBank0Register]))
		m.SetChrWindow(5, int(m.registers[chrSingleBank1Register]))
		m.SetChrWindow(6, int(m.registers[chrSingleBank2Register]))
		m.SetChrWindow(7, int(m.registers[chrSingleBank3Register]))

	default:
		m.SetChrWindow(0, int(m.registers[chrSingleBank0Register]))
		m.SetChrWindow(1, int(m.registers[chrSingleBank1Register]))
		m.SetChrWindow(2, int(m.registers[chrSingleBank2Register]))
		m.SetChrWindow(3, int(m.registers[chrSingleBank3Register]))
		m.SetChrWindow(4, int(m.registers[chrPairBank0Register]&chrPairBankMask))
		m.SetChrWindow(5, int(m.registers[chrPairBank0Register]|chrPairSecondBankBit))
		m.SetChrWindow(6, int(m.registers[chrPairBank1Register]&chrPairBankMask))
		m.SetChrWindow(7, int(m.registers[chrPairBank1Register]|chrPairSecondBankBit))
	}
}

func (m *mapperMMC3) observePatternRead(address uint16) (uint8, error) {
	m.observeA12(address&ppuA12Mask != 0)
	return 0, nil
}

func (m *mapperMMC3) observePatternWrite(address uint16, _ uint8) error {
	m.observeA12(address&ppuA12Mask != 0)
	return nil
}

func (m *mapperMMC3) observeNameTableRead(address uint16) (uint8, bool) {
	m.observeA12(address&ppuA12Mask != 0)
	return 0, false
}

func (m *mapperMMC3) observeNameTableWrite(address uint16, _ byte) bool {
	m.observeA12(address&ppuA12Mask != 0)
	return false
}

func (m *mapperMMC3) observeA12(high bool) {
	if !high {
		if !m.a12Low {
			m.a12Low = true
			m.a12LowDots = 0
		}

		return
	}

	if m.a12Low && m.a12LowDots >= a12FilterDots {
		m.clockIRQ()
	}

	m.a12Low = false
	m.a12LowDots = 0
}

// clockIRQ clocks the scanline counter. The counter reloads from the latch
// when it is zero or a reload was requested, and it decrements otherwise.
// The IRQ asserts when the counter is zero and IRQs are enabled. This is the
// Sharp revision behavior.
// https://www.nesdev.org/wiki/MMC3#IRQ_Specifics
func (m *mapperMMC3) clockIRQ() {
	if m.irqCounter == 0 || m.irqReload {
		m.irqCounter = m.irqLatch
	} else {
		m.irqCounter--
	}

	if m.irqCounter == 0 && m.irqEnabled {
		m.SetMapperIRQ(true)
	}

	m.irqReload = false
}

// readWram handles a read while the WRAM is disabled. The read returns the
// CPU open bus value.
func (m *mapperMMC3) readWram(_ uint16) (uint8, error) {
	return m.OpenBus(), nil
}

// writeWram handles a write while the WRAM is disabled or protected. The
// write is ignored.
func (m *mapperMMC3) writeWram(_ uint16, _ uint8) error {
	return nil
}

// applyPowerOnState applies the register state at power on. The MMC3 power-on
// values are unspecified, so this uses zero as a deterministic default. The
// reset vector must point into $E000-$FFFF, and code must initialize the
// mapper before it leaves that bank.
// https://www.nesdev.org/wiki/MMC3#PRG_Banks
func (m *mapperMMC3) applyPowerOnState() {
	m.bankSelect = 0
	m.prgMode = false
	m.chrMode = false
	m.registers = [bankRegisterCount]byte{}

	m.wramEnabled = false
	m.wramWriteProtect = false
	m.wramReadHook.SetProxyOnly(false)
	m.wramWriteHook.SetProxyOnly(false)

	m.irqLatch = 0
	m.irqCounter = 0
	m.irqReload = false
	m.irqEnabled = false
	m.SetMapperIRQ(false)

	m.applyPRGMapping()
	m.applyCHRMapping()
	mode := cartridge.MirrorVertical
	if m.Cartridge().Mirror == cartridge.Mirror4 {
		mode = cartridge.Mirror4
	}

	m.NameTableMemory().SetMirrorMode(mode)
}
