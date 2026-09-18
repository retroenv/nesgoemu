package lengthcounter

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestLoadTable(t *testing.T) {
	tests := []struct {
		name     string
		value    byte
		expected byte
	}{
		{"index 0", 0x00, 10},
		{"index 1", 0x08, 254},
		{"index 2", 0x10, 20},
		{"index 10", 0x50, 60},
		{"index 31", 0xf8, 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := New()
			counter.SetEnabled(true)

			counter.Load(tt.value)

			assert.Equal(t, tt.expected, counter.counter)
			assert.True(t, counter.Active())
		})
	}
}

func TestLoadIgnoresDisabledChannel(t *testing.T) {
	counter := New()

	counter.Load(0x08)

	assert.Equal(t, byte(0), counter.counter)
	assert.False(t, counter.Active())
}

func TestClockDecrementsToZero(t *testing.T) {
	counter := New()
	counter.SetEnabled(true)
	counter.Load(0x10)

	for range 20 {
		counter.Clock()
	}

	assert.False(t, counter.Active())
	assert.Equal(t, byte(0), counter.counter)
}

func TestClockStopsAtZero(t *testing.T) {
	counter := New()
	counter.SetEnabled(true)
	counter.Load(0x18)

	counter.Clock()
	assert.Equal(t, byte(1), counter.counter)

	counter.Clock()
	assert.Equal(t, byte(0), counter.counter)

	counter.Clock()
	assert.Equal(t, byte(0), counter.counter)
}

func TestHaltStopsClocking(t *testing.T) {
	counter := New()
	counter.SetEnabled(true)
	counter.Load(0x10)
	counter.SetHalt(true)

	for range 20 {
		counter.Clock()
	}

	assert.True(t, counter.Active())
}

func TestSetEnabledClearsCounter(t *testing.T) {
	counter := New()
	counter.SetEnabled(true)
	counter.Load(0x08)

	counter.SetEnabled(true)
	assert.True(t, counter.Active())

	counter.SetEnabled(false)
	assert.False(t, counter.Active())
}

func TestReset(t *testing.T) {
	counter := New()
	counter.SetEnabled(true)
	counter.Load(0x08)
	counter.SetHalt(true)

	counter.Reset()

	assert.False(t, counter.Active())
	counter.Load(0x08)
	assert.False(t, counter.Active(), "a reset counter does not load values")
}
