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

			counter.Commit()

			assert.Equal(t, tt.expected, counter.counter)
			assert.True(t, counter.Active())
		})
	}
}

func TestLoadIgnoresDisabledChannel(t *testing.T) {
	counter := New()

	counter.Load(0x08)

	counter.Commit()

	assert.Equal(t, byte(0), counter.counter)
	assert.False(t, counter.Active())
}

func TestClockDecrementsToZero(t *testing.T) {
	counter := New()
	counter.SetEnabled(true)
	counter.Load(0x10)
	counter.Commit()

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
	counter.Commit()

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
	counter.Commit()
	counter.SetHalt(true)
	counter.Commit()

	for range 20 {
		counter.Clock()
	}

	assert.True(t, counter.Active())
}

func TestSetEnabledClearsCounter(t *testing.T) {
	counter := New()
	counter.SetEnabled(true)
	counter.Load(0x08)
	counter.Commit()

	counter.SetEnabled(true)
	assert.True(t, counter.Active())

	counter.SetEnabled(false)
	assert.False(t, counter.Active())
}

func TestReset(t *testing.T) {
	counter := New()
	counter.SetEnabled(true)
	counter.Load(0x08)
	counter.Commit()
	counter.SetHalt(true)
	counter.Commit()

	counter.Reset()

	assert.False(t, counter.Active())
	counter.Load(0x08)
	counter.Commit()
	assert.False(t, counter.Active(), "a reset counter does not load values")
}

func TestReloadOnLengthClock(t *testing.T) {
	// A decrement cancels the pending reload, including a decrement to zero.
	// A counter that was already zero accepts the reload.
	for _, initial := range []byte{0, 1, 6} {
		counter := New()
		counter.SetEnabled(true)
		counter.counter = initial
		counter.Load(0x18) // Reload value 2.
		counter.Clock()
		counter.Commit()
		want := byte(2)
		if initial > 0 {
			want = initial - 1
		}
		assert.Equal(t, want, counter.counter)
	}
}

func TestHaltWriteAppliesAfterLengthClock(t *testing.T) {
	for _, halted := range []bool{false, true} {
		counter := New()
		counter.SetEnabled(true)
		counter.Load(0x18)
		counter.SetHalt(halted)
		counter.Commit()

		counter.SetHalt(!halted)
		counter.Clock()
		counter.Commit()
		want := byte(1)
		if halted {
			want = 2
		}
		assert.Equal(t, want, counter.counter)
	}
}
