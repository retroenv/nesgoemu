package rainbow

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/assert"
)

var _ bus.PPUMappingInspector = (*Mapper)(nil)

func TestPPUMappingAllCHRWindows(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x8000)
	for mode := range 8 {
		m.chrMode = byte(mode)
		size := 0x2000 >> min(mode, 4)
		for register := range 0x2000 / size {
			m.chrBanks[register] = uint16(0x103 + register)
		}
		for _, source := range []byte{chrSourceROM, chrSourceRAM} {
			m.chrSource = source
			memory, data := bus.GraphicsCHRROM, m.chrROM
			if source == chrSourceRAM {
				memory, data = bus.GraphicsCHRRAM, m.chrRAM
			}
			for window := range 0x2000 / size {
				for _, offset := range []int{0, 1, size - 1} {
					address := uint16(window*size + offset)
					physical := ((0x103+window)*size + offset) % len(data)
					want := bus.GraphicsMapping{
						Memory: memory,
						Offset: physical,
					}
					before := *m
					read, ok := m.PPUReadMapping(address)
					assert.True(t, ok)
					assert.Equal(t, want, read)
					write, ok := m.PPUWriteMapping(address)
					assert.True(t, ok)
					assert.Equal(t, want, write)
					assert.Equal(t, before, *m)
					data[physical] = 0xa5
					assert.Equal(t, byte(0xa5), m.Read(address))
				}
			}
		}
	}
}

func TestPPUMappingDirectSources(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	for _, test := range []struct {
		source byte
		memory bus.GraphicsMemory
		mask   int
	}{
		{chrSourceNT, bus.GraphicsCIRAM, 0x7ff},
		{chrSourceFPGA, bus.GraphicsFPGARAM, 0xfff},
	} {
		m.chrSource = test.source
		for _, address := range []uint16{0, 0x7ff, 0x1000, 0x1fff} {
			want := bus.GraphicsMapping{
				Memory: test.memory,
				Offset: int(address) & test.mask,
			}
			before := *m
			read, ok := m.PPUReadMapping(address)
			assert.True(t, ok)
			assert.Equal(t, want, read)
			write, ok := m.PPUWriteMapping(address)
			assert.True(t, ok)
			assert.Equal(t, want, write)
			assert.Equal(t, before, *m)
			m.Write(address, 0x5a)
			assert.Equal(t, byte(0x5a), m.Read(address))
		}
	}
}

func TestPPUMappingGraphicsContext(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x800000)
	m.EnableBusTiming()
	m.scanIRQ.inFrame = true
	m.scanIRQ.counter = 16
	m.windowEnabled = true
	m.windowSplitRegs = [6]byte{0, 31, 0, 239, 0, 5}
	m.bgExtActive = true
	m.bgExtData = 2
	m.bgExtModeOffset = 1
	assertCHRROMMapping(t, m, 0x40, 0x42045, 0x40)

	m.windowEnabled = false
	m.bgExtActive = false
	m.spriteExtMode = true
	m.activeSpriteIndex = 3
	m.activeSpriteSize = 8
	m.spriteBankUpper = 1
	m.spriteBankLower[3] = 2
	assertCHRROMMapping(t, m, 0x1045, 0x102045, 0x1045)
	m.activeSpriteSize = 16
	assertCHRROMMapping(t, m, 0x1045, 0x205045, 0x1045)
}

func TestPPUMappingRejectsInvalidAndAbsentMemory(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	for _, address := range []uint16{0x2000, 0x3fff, 0xffff} {
		_, ok := m.PPUReadMapping(address)
		assert.False(t, ok)
		_, ok = m.PPUWriteMapping(address)
		assert.False(t, ok)
	}
	m.chrFlash.ID = true
	_, ok := m.PPUReadMapping(0)
	assert.False(t, ok)
	_, ok = m.PPUWriteMapping(0)
	assert.True(t, ok)
	m.chrROM = nil
	_, ok = m.PPUWriteMapping(0)
	assert.False(t, ok)
	m.chrFlash.ID = false
	_, ok = m.PPUReadMapping(0)
	assert.False(t, ok)
	m.chrSource = chrSourceRAM
	m.chrRAM = nil
	_, ok = m.PPUReadMapping(0)
	assert.False(t, ok)
	_, ok = m.PPUWriteMapping(0)
	assert.False(t, ok)
}

func assertCHRROMMapping(t *testing.T, m *Mapper, address uint16, readOffset, writeOffset int) {
	t.Helper()
	before := *m
	read, ok := m.PPUReadMapping(address)
	assert.True(t, ok)
	assert.Equal(t, bus.GraphicsMapping{
		Memory: bus.GraphicsCHRROM,
		Offset: readOffset,
	}, read)
	write, ok := m.PPUWriteMapping(address)
	assert.True(t, ok)
	assert.Equal(t, bus.GraphicsMapping{
		Memory: bus.GraphicsCHRROM,
		Offset: writeOffset,
	}, write)
	assert.Equal(t, before, *m)
	m.chrROM[readOffset] = 0xa5
	assert.Equal(t, byte(0xa5), m.Read(address))
}
