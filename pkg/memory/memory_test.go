package memory

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/controller"
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

func TestWriteFrameCounterDoesNotStrobeController(t *testing.T) {
	apu := &pcmTestAPU{}
	systemBus := &bus.Bus{
		APU:         apu,
		Controller2: controller.New(),
	}
	memory := New(systemBus)

	systemBus.Controller2.SetButtonState(controller.A, true)
	systemBus.Controller2.SetStrobeMode(1)
	systemBus.Controller2.SetStrobeMode(0)
	assert.Equal(t, byte(1), systemBus.Controller2.Read())
	memory.Write(0x4017, 1)

	assert.Equal(t, []apuWrite{{address: 0x4017, value: 1}}, apu.writes)
	assert.Equal(t, byte(0), systemBus.Controller2.Read(), "a frame-counter write must not restart controller reads")
}

func TestControllerStrobeAndAdjacentReads(t *testing.T) {
	systemBus := &bus.Bus{
		Controller1: controller.New(),
		Controller2: controller.New(),
	}
	memory := New(systemBus)
	for _, device := range []bus.Controller{systemBus.Controller1, systemBus.Controller2} {
		device.SetButtonState(controller.A, true)
	}
	memory.Write(0x4016, 1)
	memory.Write(0x4016, 0)
	memory.Write(0, 0x40)
	for _, address := range []uint16{0x4016, 0x4017} {
		for range 3 {
			memory.BeginCycle()
			assert.Equal(t, byte(0x41), memory.Read(address), "adjacent reads keep output enable active")
		}
		memory.BeginCycle()
		memory.Read(0)
		memory.BeginCycle()
		assert.Equal(t, byte(0x40), memory.Read(address), "a bus access between reads permits a new button bit")
	}
}

func TestWriteOtherAPURegistersSkipsController(t *testing.T) {
	apu := &pcmTestAPU{}
	systemBus := &bus.Bus{
		APU:         apu,
		Controller2: controller.New(),
	}
	memory := New(systemBus)

	memory.Write(0x4000, 0xbf)

	assert.Equal(t, []apuWrite{{address: 0x4000, value: 0xbf}}, apu.writes)
}

type apuWrite struct {
	address uint16
	value   byte
}

type pcmTestAPU struct {
	bus.APU
	reads  int
	writes []apuWrite
}

func (a *pcmTestAPU) Read(uint16) byte {
	a.reads++
	return 0x5A
}

func (a *pcmTestAPU) Write(address uint16, value byte) {
	entry := apuWrite{
		address: address,
		value:   value,
	}
	a.writes = append(a.writes, entry)
}

type pcmTestMapper struct {
	bus.Mapper
	reads int
}

func (m *pcmTestMapper) ReadPCM() byte {
	m.reads++
	return 0xA5
}
