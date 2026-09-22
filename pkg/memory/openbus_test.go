package memory

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/retrogolib/assert"
)

func TestOpenBusRepeatsLastMemoryRead(t *testing.T) {
	t.Parallel()

	systemMemory := New(&bus.Bus{})
	systemMemory.Write(0x0000, 0x40)

	assert.Equal(t, byte(0x40), systemMemory.Read(0x0000))
	// $4018 to $401F are not mapped and repeat the value on the bus.
	assert.Equal(t, byte(0x40), systemMemory.Read(0x4018))
	assert.Equal(t, byte(0x40), systemMemory.OpenBus())
}

func TestOpenBusRepeatsLastMemoryWrite(t *testing.T) {
	t.Parallel()

	systemMemory := New(&bus.Bus{})
	systemMemory.Write(0x0000, 0x5A)

	assert.Equal(t, byte(0x5A), systemMemory.OpenBus())
	assert.Equal(t, byte(0x5A), systemMemory.Read(0x401F))
}

func TestControllerReadKeepsOpenBusBits(t *testing.T) {
	t.Parallel()

	controller1 := controller.New()
	controller2 := controller.New()
	systemMemory := New(&bus.Bus{
		Controller1: controller1,
		Controller2: controller2,
	})
	// The high byte $40 of an absolute address is the usual value on the bus.
	systemMemory.Write(0x0000, 0x40)
	controller1.SetButtonState(controller.A, true)
	controller2.SetButtonState(controller.A, true)

	// Games by Mindscape rely on $41 for a pressed button.
	assert.Equal(t, byte(0x41), systemMemory.Read(0x4016))
	assert.Equal(t, byte(0x41), systemMemory.Read(0x4017))
}

func TestUnmappedIOWriteIsIgnored(t *testing.T) {
	t.Parallel()

	systemMemory := New(&bus.Bus{})
	systemMemory.Write(0x4018, 0xA5)

	assert.Equal(t, byte(0xA5), systemMemory.OpenBus())
	assert.Equal(t, byte(0xA5), systemMemory.Read(0x4018))
}
