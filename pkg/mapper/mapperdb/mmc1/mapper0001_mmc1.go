// Package mmc1 implements MMC1 cartridge boards.
package mmc1

import (
	"errors"
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
Boards: SKROM, SLROM, SNROM, others
PRG ROM capacity: 256K (512K)
PRG ROM window: 16K + 16K fixed or 32K
PRG RAM capacity: 32K
PRG RAM window:8K
CHR capacity: 128K
CHR window: 4K + 4K or 8K
*/

const (
	prgRAMSize    = 0x8000
	chrWindowSize = 0x1000

	chrBank0RegisterStart = 0xa000
	chrBank1RegisterStart = 0xc000
	prgBankRegisterStart  = 0xe000

	shiftRegisterBits = 5
	shiftResetBit     = 0x80

	mirrorModeMask        = 0x03
	singleScreenModeCount = 2
	prgModeShift          = 2
	prgModeMask           = 0x03
	prgModeFixedFirst     = 2
	prgModeFixedLast      = 3
	chrModeShift          = 4
	chrModeMask           = 0x01

	// prgModeFixLast is the control register value of PRG mode 3, which fixes the
	// last bank at $C000.
	prgModeFixLast = 0x0C

	prgBankMask = 0x0f
)

// New returns a new MMC1 mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	m := &mapperMMC1{
		Base:    base,
		ram:     make([]byte, prgRAMSize),
		control: prgModeFixLast, // the MMC1 powers on with the last bank at $C000
	}
	m.SetName("MMC1")
	m.DeclareFeature(feature.CHRBanking)
	m.DeclareFeature(feature.Mirroring)
	m.DeclareFeature(feature.PRGBanking)
	m.DeclareFeature(feature.ShiftRegister)
	m.DeclareFeature(feature.SingleScreenMirroring)
	m.SetChrWindowSize(chrWindowSize)
	m.SetPrgRAM(m.ram)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}

	m.AddWriteHook(nes.CodeBaseAddress, math.MaxUint16, m.writeShiftBit)

	m.SetPrgWindow(1, -1)

	translation := mapperbase.MirrorModeTranslation{
		0: cartridge.MirrorSingle0,
		1: cartridge.MirrorSingle1,
		2: cartridge.MirrorVertical,
		3: cartridge.MirrorHorizontal,
	}
	m.SetMirrorModeTranslation(translation)

	// TODO support mmc1 variants

	return m, nil
}

type mapperMMC1 struct {
	*mapperbase.Base

	ram []byte

	shiftCount    byte
	shiftRegister byte

	control  byte
	chrBank0 int
	chrBank1 int
	prgBank  int

	lastWriteCycle uint64
	lastWriteValid bool // lastWriteCycle holds the cycle of a write
}

// SnapshotFormat identifies the direct mapper state format.
func (*mapperMMC1) SnapshotFormat() string { return "mmc1-mapperMMC1-v1" }

// TransferState transfers registers and mutable cartridge memory.
func (m *mapperMMC1) TransferState(archive *stateio.Archive) error {
	if err := m.Base.TransferState(archive); err != nil {
		return fmt.Errorf("transferring mmc1: %w", err)
	}

	if err := archive.Values(&m.shiftCount, &m.shiftRegister, &m.control, &m.chrBank0, &m.chrBank1, &m.prgBank, &m.lastWriteCycle, &m.lastWriteValid); err != nil {
		return fmt.Errorf("transferring mmc1: %w", err)
	}
	if m.shiftCount >= shiftRegisterBits || m.shiftRegister >= 1<<shiftRegisterBits {
		return errors.New("invalid MMC1 snapshot shift state")
	}
	return nil
}

// resetShift clears the shift register and applies the control register.
func (m *mapperMMC1) resetShift() error {
	m.shiftCount = 0
	m.shiftRegister = 0
	return m.applyControl()
}

// writeShiftBit writes one bit of a serial register write. A write with bit 7
// set resets the shift register and sets PRG mode 3. The MMC1 ignores a write
// on the cycle after a write, except a reset. Read-modify-write instructions
// write on two consecutive cycles. https://www.nesdev.org/wiki/MMC1
func (m *mapperMMC1) writeShiftBit(address uint16, value uint8) error {
	m.MarkFeature(feature.ShiftRegister)

	cycle, ok := m.CPUCycles()
	consecutive := ok && m.lastWriteValid && cycle == m.lastWriteCycle+1
	m.lastWriteCycle = cycle
	m.lastWriteValid = ok

	if value&shiftResetBit != 0 {
		m.control |= prgModeFixLast
		return m.resetShift()
	}
	if consecutive {
		return nil
	}

	// the shift register gets written from lowest to highest bit
	bit := (value & 1) << m.shiftCount
	m.shiftRegister |= bit

	m.shiftCount++
	if m.shiftCount < shiftRegisterBits {
		return nil
	}

	switch {
	case address < chrBank0RegisterStart: // $8000-$9FFF
		m.control = m.shiftRegister

	case address < chrBank1RegisterStart: // $A000-$BFFF
		m.chrBank0 = int(m.shiftRegister)

	case address < prgBankRegisterStart: // $C000-$DFFF
		m.chrBank1 = int(m.shiftRegister)

	case address >= prgBankRegisterStart: // $E000-$FFFF
		m.prgBank = int(m.shiftRegister) & prgBankMask
	}

	return m.resetShift()
}

func (m *mapperMMC1) applyControl() error {
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.CHRBanking)

	// Control bits 1-0: 0 and 1 select one-screen A and B, 2 and 3 select
	// vertical and horizontal mirroring. https://www.nesdev.org/wiki/MMC1
	mirrorMode := m.control & mirrorModeMask
	if mirrorMode < singleScreenModeCount {
		m.MarkFeature(feature.SingleScreenMirroring)
	} else {
		m.MarkFeature(feature.Mirroring)
	}

	if err := m.SetNameTableMirrorModeIndex(mirrorMode); err != nil {
		return fmt.Errorf("setting MMC1 mirror mode: %w", err)
	}

	prgMode := (m.control >> prgModeShift) & prgModeMask
	switch prgMode {
	case 0, 1:
		// switch 32 KB at $8000, ignoring low bit of bank number
		m.SetPrgWindow(0, m.prgBank&^1)
		m.SetPrgWindow(1, m.prgBank|1)

	case prgModeFixedFirst:
		// fix first bank at $8000 and switch 16 KB bank at $C000
		m.SetPrgWindow(0, 0)
		m.SetPrgWindow(1, m.prgBank)

	case prgModeFixedLast:
		// fix last bank at $C000 and switch 16 KB bank at $8000
		m.SetPrgWindow(0, m.prgBank)
		m.SetPrgWindow(1, -1)
	}

	m.SetChrWindow(0, m.chrBank0)
	chrMode := (m.control >> chrModeShift) & chrModeMask
	if chrMode == 0 {
		// switch 8 KB at a time
		m.SetChrWindow(1, m.chrBank0+1)
	} else {
		// switch two separate 4 KB banks
		m.SetChrWindow(1, m.chrBank1)
	}

	return nil
}
