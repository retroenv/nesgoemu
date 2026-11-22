package rainbow

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/assert"
)

func TestInspectCHRMatchesReadWithoutEffects(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.ppuBus.enabled = true

	m.chrSource = chrSourceROM
	m.chrROM[0x10] = 0x5a
	assert.Equal(t, byte(0x5a), m.InspectCHR(0x10))
	assert.Equal(t, 0, m.ppuBus.reads)

	assert.Equal(t, byte(0x5a), m.Read(0x10))
	assert.Equal(t, 1, m.ppuBus.reads)

	m.chrSource = chrSourceRAM
	m.chrRAM = make([]byte, 0x2000)
	m.chrRAM[0x20] = 0x77
	assert.Equal(t, byte(0x77), m.InspectCHR(0x20))
	assert.Equal(t, m.Read(0x20), m.InspectCHR(0x20))
	assert.Equal(t, 2, m.ppuBus.reads)
}

func TestInspectPRGRAM(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.prgRAM = make([]byte, 16)
	m.prgRAM[5] = 0x42

	value, ok := m.InspectPRGRAM(5)
	assert.True(t, ok)
	assert.Equal(t, byte(0x42), value)

	_, ok = m.InspectPRGRAM(16)
	assert.False(t, ok)
	_, ok = m.InspectPRGRAM(-1)
	assert.False(t, ok)
}

// Banked inspection must use the active low window and must keep IRQ state.
func TestInspectCPUUsesCurrentBanks(t *testing.T) {
	const (
		lowRAMAddress  = mapperbase.PRGRAMStart
		lowFPGAAddress = lowRAMAddress + prgWindowSize4K
		ramOffset      = 0x2000
		romValue       = 0x11
		ramValue       = 0x22
		fpgaValue      = 0x33
		updatedValue   = 0x44
	)
	m := newTestMapper(t, 0x10000, 0x2000)
	m.prgRAM = make([]byte, 0x8000)
	m.prgROM[0x4000] = romValue
	m.prgRAM[ramOffset] = ramValue
	m.fpgaRAM[0x1000] = fpgaValue
	m.prgRAMMode = prgRAMMode4K
	m.lowBanks[0] = uint16(prgLowBankSourceRAM<<prgLowBankSourceShift) | 2
	m.lowBanks[1] = uint16(prgLowBankSourceFPGA<<prgLowBankSourceShift) | 1
	before := m.State()
	value, ok := m.InspectCPU(lowRAMAddress)
	assert.True(t, ok)
	assert.Equal(t, byte(ramValue), value)
	value, ok = m.InspectCPU(lowFPGAAddress)
	assert.True(t, ok)
	assert.Equal(t, byte(fpgaValue), value)
	assert.Equal(t, before, m.State())
	offset, ok := m.CPURAMOffset(lowRAMAddress)
	assert.True(t, ok)
	assert.Equal(t, ramOffset, offset)
	assert.True(t, m.WritePRGRAM(offset, updatedValue))
	assert.Equal(t, byte(updatedValue), m.prgRAM[ramOffset])
	_, ok = m.CPURAMOffset(lowFPGAAddress)
	assert.False(t, ok)
	m.lowBanks[0] = 4
	value, ok = m.InspectCPU(lowRAMAddress)
	assert.True(t, ok)
	assert.Equal(t, byte(romValue), value)
	_, ok = m.CPURAMOffset(lowRAMAddress)
	assert.False(t, ok)
	_, ok = m.InspectCPU(registerStart)
	assert.False(t, ok)
}
