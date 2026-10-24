package mmc1

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

// cycleCPU reports a CPU cycle count that the test sets.
type cycleCPU struct {
	cycles uint64
}

func (c *cycleCPU) Cycles() uint64       { return c.cycles }
func (c *cycleCPU) StallCycles(uint16)   {}
func (c *cycleCPU) State() cpu6502.State { return cpu6502.State{} }
func (c *cycleCPU) SetIRQ(bool)          {}
func (c *cycleCPU) TriggerIrq()          {}
func (c *cycleCPU) TriggerNMI()          {}

func TestMapperMMC1(t *testing.T) {
	chr := make([]byte, 0x1000*3) // 4K banks
	prg := make([]byte, 0x4000*3) // 16K banks

	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			CHR: chr,
			PRG: prg,
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	m, err := New(base)
	assert.NoError(t, err)

	chr[0x0000] = 0x01
	chr[0x2000] = 0x02
	prg[0x0000] = 0x03
	prg[0x8000] = 0x04

	m.Write(0x8000, 0)
	m.Write(0x8000, 1) // select bank 2
	m.Write(0x8000, 0)
	m.Write(0x8000, 0)
	m.Write(0xE000, 0) // set prg bank

	m.Write(0x8000, 0)
	m.Write(0x8000, 1) // select bank 2
	m.Write(0x8000, 0)
	m.Write(0x8000, 0)
	m.Write(0xC000, 0) // set chr 1 bank

	m.Write(0x8000, 1) // mirror mode 1
	m.Write(0x8000, 0)
	m.Write(0x8000, 0)
	m.Write(0x8000, 1) // prg mode 2
	m.Write(0x9000, 1) // chr mode 1, set control

	assert.Equal(t, 0x01, m.Read(0x0000))
	assert.Equal(t, 0x02, m.Read(0x1000))
	assert.Equal(t, 0x03, m.Read(0x8000))
	assert.Equal(t, 0x04, m.Read(0xC000))

	mode := m.MirrorMode()
	assert.Equal(t, cartridge.MirrorSingle1, mode)
	assert.True(t, featureUsed(base, feature.SingleScreenMirroring))
	assert.False(t, featureUsed(base, feature.Mirroring))

	writeShiftRegister(m, 0x8000, 0b0_11_11) // horizontal mirroring
	assert.Equal(t, cartridge.MirrorHorizontal, m.MirrorMode())
	assert.True(t, featureUsed(base, feature.Mirroring))
}

func TestMapperMMC1ResetFixesLastBank(t *testing.T) {
	// A reset write sets PRG mode 3 before the mapper applies the control
	// register. Golf Grand Slam writes $80 and then jumps to $C11E.
	m := newBankTestMapper(t, nil)

	writeShiftRegister(m, 0x8000, 0b0_10_00) // PRG mode 2
	writeShiftRegister(m, 0xE000, 5)
	assert.Equal(t, 0, m.Read(0x8000))
	assert.Equal(t, 5, m.Read(0xC000))

	m.Write(0x8000, 0x80)

	assert.Equal(t, 5, m.Read(0x8000))
	assert.Equal(t, 7, m.Read(0xC000))
}

func TestMapperMMC1KeepsPRGMode(t *testing.T) {
	// A completed register write does not change the PRG mode.
	m := newBankTestMapper(t, nil)

	writeShiftRegister(m, 0x8000, 0b0_10_00) // PRG mode 2
	writeShiftRegister(m, 0xE000, 5)

	assert.Equal(t, 0, m.Read(0x8000))
	assert.Equal(t, 5, m.Read(0xC000))
}

func TestMapperMMC1PRG32KIgnoresLowBit(t *testing.T) {
	m := newBankTestMapper(t, nil)

	writeShiftRegister(m, 0x8000, 0b0_00_00) // PRG mode 0
	writeShiftRegister(m, 0xE000, 3)

	assert.Equal(t, 2, m.Read(0x8000))
	assert.Equal(t, 3, m.Read(0xC000))
}

func TestMapperMMC1IgnoresConsecutiveWrites(t *testing.T) {
	// A read-modify-write instruction writes on two consecutive cycles. The
	// MMC1 ignores the second write. Bill & Ted's Excellent Video Game
	// Adventure resets the mapper with INC on a ROM byte that contains $FF.
	cpu := &cycleCPU{}
	m := newBankTestMapper(t, cpu)
	writeShiftRegister(m, 0x8000, 0b0_10_00) // PRG mode 2

	cpu.cycles = 100
	m.Write(0x8000, 0xFF)
	cpu.cycles = 101
	m.Write(0x8000, 0x00) // ignored
	for bit, cycle := range []uint64{110, 120, 130, 140, 150} {
		cpu.cycles = cycle
		m.Write(0xE000, 6>>bit&1)
	}

	assert.Equal(t, 6, m.Read(0x8000))
	assert.Equal(t, 7, m.Read(0xC000))
}

// newBankTestMapper returns an MMC1 mapper with eight 16K PRG banks. The first
// byte of each bank holds the bank number.
func newBankTestMapper(t *testing.T, cpu bus.CPU) bus.Mapper {
	t.Helper()
	const banks = 8
	prg := make([]byte, 0x4000*banks)
	for bank := range banks {
		prg[bank*0x4000] = byte(bank)
	}

	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			CHR: make([]byte, 0x2000),
			PRG: prg,
		},
		CPU:       cpu,
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	m, err := New(base)
	assert.NoError(t, err)

	return m
}

// writeShiftRegister writes the five low bits of value, lowest bit first.
func writeShiftRegister(m bus.Mapper, address uint16, value byte) {
	for bit := range 5 {
		m.Write(address, value>>bit&1)
	}
}

func featureUsed(user feature.User, id feature.ID) bool {
	for _, usage := range user.Features() {
		if usage.ID == id {
			return usage.Used
		}
	}

	return false
}
