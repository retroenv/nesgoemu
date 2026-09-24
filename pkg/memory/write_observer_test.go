package memory

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/assert"
)

func TestWriteObserver(t *testing.T) {
	memory := New(&bus.Bus{})
	memory.Write(0x123, 0x10)
	var event WriteEvent
	called := 0
	memory.ObserveWrites(func(value WriteEvent) {
		assert.Equal(t, byte(0x10), memory.OpenBus())
		event = value
		called++
	})
	memory.Write(0x123, 0x5a)
	assert.Equal(t, 1, called)
	assert.Equal(t, WriteEvent{
		Address: 0x123,
		Value:   0x5a,
	}, event)
	replaced := 0
	memory.ObserveWrites(func(WriteEvent) { replaced++ })
	memory.Write(0x123, 0x33)
	assert.Equal(t, 1, called)
	assert.Equal(t, 1, replaced)
	memory.ObserveWrites(nil)
	memory.Write(0x123, 0x44)
	assert.Equal(t, 1, replaced)
}
