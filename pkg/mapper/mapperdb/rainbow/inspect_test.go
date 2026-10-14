package rainbow

import (
	"testing"

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
