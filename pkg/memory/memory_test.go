package memory

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/assert"
)

func TestPCMReadUsesOptionalMapper(t *testing.T) {
	apu := &pcmTestAPU{}
	mapper := &pcmTestMapper{}
	memory := New(&bus.Bus{
		APU:    apu,
		Mapper: mapper,
	})

	assert.Equal(t, byte(0xA5), memory.Read(0x4011))
	assert.Equal(t, 1, mapper.reads)
	assert.Equal(t, 0, apu.reads)
	assert.Equal(t, byte(0x5A), memory.Read(0x4010))
	assert.Equal(t, 1, apu.reads)
}

func TestPCMReadFallsBackToAPU(t *testing.T) {
	apu := &pcmTestAPU{}
	memory := New(&bus.Bus{APU: apu})

	assert.Equal(t, byte(0x5A), memory.Read(0x4011))
	assert.Equal(t, 1, apu.reads)
}

func TestInspectRAMReadsWithoutBusAccess(t *testing.T) {
	systemBus := &bus.Bus{}
	memory := New(systemBus)
	memory.Write(0x0807, 0xa5)

	value, ok := memory.InspectRAM(0x0807)

	assert.True(t, ok)
	assert.Equal(t, byte(0xa5), value)
	_, ok = memory.InspectRAM(0x2000)
	assert.False(t, ok)
}

func TestInternalRAMMirrorsEveryTwoKiB(t *testing.T) {
	memory := New(&bus.Bus{})
	memory.Write(0x0007, 0xa5)
	for _, address := range []uint16{0x0007, 0x0807, 0x1007, 0x1807} {
		assert.Equal(t, byte(0xa5), memory.Read(address))
	}
	memory.Write(0x0fff, 0x5a)
	value, ok := memory.InspectRAM(0x07ff)
	assert.True(t, ok)
	assert.Equal(t, byte(0x5a), value)
}

type pcmTestAPU struct {
	bus.APU
	reads int
}

func (a *pcmTestAPU) Read(uint16) byte {
	a.reads++
	return 0x5A
}

type pcmTestMapper struct {
	bus.Mapper
	reads int
}

func (m *pcmTestMapper) ReadPCM() byte {
	m.reads++
	return 0xA5
}
