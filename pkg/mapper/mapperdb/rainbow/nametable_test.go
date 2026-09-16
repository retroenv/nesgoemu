package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestNametableWritesAndCIRAMBanks(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	nt := m.NameTableMemory()
	nt.Write(0x2800, 0xAB)
	assert.Equal(t, byte(0xAB), nt.Read(0x2C00))
	assert.Equal(t, byte(0), nt.Read(0x2000))
	m.Write(regNTBankStart, 1)
	assert.Equal(t, byte(0xAB), nt.Read(0x2000))
	for source := byte(1); source <= 2; source++ {
		m.Write(regNTControlStart, source<<6)
		m.Write(regNTBankStart, 2)
		nt.Write(0x3012, 0xCD)
		assert.Equal(t, byte(0xCD), nt.Read(0x2012))
		if source == 1 {
			assert.Equal(t, byte(0xCD), m.chrRAM[0x812])
		} else {
			assert.Equal(t, byte(0xCD), m.fpgaRAM[0x812])
		}
	}
	m.Write(regNTControlStart, 0xC0)
	nt.Write(0x2012, 0xEF)
	assert.Equal(t, byte(0), m.chrROM[0x812])
}

func TestCIRAMPatternSource(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regCHRControl, 0xC4)
	m.Write(regCHRBankLowerStart, 2)
	m.Write(0x12, 0xAB)
	assert.Equal(t, byte(0xAB), m.NameTableMemory().ReadCIRAM(0x12))
	assert.Equal(t, byte(0xAB), m.Read(0x12))
}

func TestWindowSourceForcedToFPGA(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	assert.Equal(t, byte(0x80), m.Read(regNTWindowControl))
	for _, value := range []byte{0, 0x40, 0xC0, 0xFF} {
		m.Write(regNTWindowControl, value)
		assert.Equal(t, (value&0x3F)|0x80, m.Read(regNTWindowControl))
	}
}

func TestCIRAMIgnoresPatternBanks(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	for mode := byte(0); mode < 8; mode++ {
		m.Write(regCHRControl, 0xC0|mode)
		for i := range uint16(16) {
			m.Write(regCHRBankLowerStart+i, byte(i+3))
		}
		for _, address := range []uint16{0x12, 0x412, 0x812, 0x1412, 0x1C12} {
			m.Write(address, byte(address>>8)+1)
			expected := m.NameTableMemory().ReadCIRAM(address & 0x7FF)
			assert.Equal(t, expected, m.Read(address))
			m.bgExtActive = true
			m.bgExtData = 0xFF
			assert.Equal(t, expected, m.Read(address))
			m.spriteExtMode = true
			m.activeSpriteIndex = 3
			m.spriteBankLower[3] = 0xFF
			assert.Equal(t, expected, m.Read(address))
			m.activeSpriteIndex = -1
		}
	}
}

func TestFillAttributeOverridesExtendedPalette(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regNTControlStart, 0x23)
	m.Write(regFillAttribute, 2)
	m.fpgaRAM[0] = 0xC0
	m.NameTableMemory().Read(0x2000)
	assert.Equal(t, byte(0xAA), m.NameTableMemory().Read(0x23C0))
}
