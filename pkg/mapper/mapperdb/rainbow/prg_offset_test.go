package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestPRGROMOffsetAllWindowsAndAliases(t *testing.T) {
	m := newTestMapper(t, 8*1024*1024, 0x2000)
	fillBankTestData(m.prgROM, 0x35)
	for mode := byte(0); mode < 8; mode++ {
		m.Write(regPRGControl, mode)
		for register := range uint16(8) {
			bank := uint16(0x703) + register
			m.Write(regHighBankUpperStart+register, byte(bank>>8)|0x78)
			m.Write(regHighBankLowerStart+register, byte(bank))
		}
		for address := 0x8000; address < 0x10000; address++ {
			physical, ok := m.PRGROMOffset(uint16(address))
			assert.True(t, ok)
			assert.Equal(t, m.prgROM[physical], m.Read(uint16(address)))
		}
	}
}

func TestPRGROMOffsetRejectsNonROMAndHasNoSideEffects(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	for _, address := range []uint16{0, 0x6000, 0x7fff} {
		_, ok := m.PRGROMOffset(address)
		assert.False(t, ok)
	}
	m.Write(regHighBankUpperStart, 0x80)
	_, ok := m.PRGROMOffset(0x8000)
	assert.False(t, ok)
	m.Write(regHighBankUpperStart, 0)
	m.prgFlash.ID = true
	_, ok = m.PRGROMOffset(0x8000)
	assert.False(t, ok)
	m.prgFlash.ID = false
	before := *m
	_, ok = m.PRGROMOffset(0xffff)
	assert.True(t, ok)
	assert.Equal(t, before, *m)
	m.prgROM = nil
	_, ok = m.PRGROMOffset(0x8000)
	assert.False(t, ok)
}
