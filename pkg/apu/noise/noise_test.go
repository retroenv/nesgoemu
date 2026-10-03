package noise

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestWriteSelectsPeriodFromTable(t *testing.T) {
	tests := []struct {
		name     string
		value    byte
		expected uint16
	}{
		{"index 0", 0x00, 2},
		{"index 1", 0x01, 4},
		{"index 5", 0x05, 48},
		{"index 8", 0x08, 101},
		{"index 15", 0x0f, 2034},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noise := New()

			noise.Write(0x400e, tt.value)

			noise.CommitLengthWrites()

			assert.Equal(t, tt.expected, noise.timer)
		})
	}
}

func TestWriteModeFlag(t *testing.T) {
	n := New()

	n.Write(0x400e, 0x80)

	n.CommitLengthWrites()
	assert.True(t, n.mode)

	n.Write(0x400e, 0x00)

	n.CommitLengthWrites()
	assert.False(t, n.mode)
}

func TestClockReloadsPeriodAfterShift(t *testing.T) {
	n := New()
	n.Write(0x400e, 0x01) // period 4 in APU cycles
	n.CommitLengthWrites()

	n.Clock()
	assert.Equal(t, uint16(3), n.counter, "the shift register is clocked first")

	for range 3 {
		n.Clock()
	}
	assert.Equal(t, uint16(0), n.counter)

	n.Clock()
	assert.Equal(t, uint16(3), n.counter, "the period repeats")
}

func TestShiftRegisterShiftsTowardsBitFourteen(t *testing.T) {
	n := New()

	n.advance()

	assert.Equal(t, uint16(0x4000), n.shift, "the feedback moves into bit 14")
}

func TestShiftRegisterRepeatsAfterMaximumSequence(t *testing.T) {
	n := New()

	for range 32767 {
		n.advance()
	}

	assert.Equal(t, powerOnShiftRegister, n.shift, "the normal mode sequence is 32767 steps long")
}

func TestShiftRegisterUsesModeTap(t *testing.T) {
	n := New()
	n.shift = 0x40 // bit 6 set, bit 1 clear

	n.advance()

	assert.Equal(t, uint16(0x20), n.shift)

	modeChannel := New()
	modeChannel.shift = 0x40
	modeChannel.Write(0x400e, 0x80)
	modeChannel.CommitLengthWrites()
	modeChannel.advance()

	assert.Equal(t, uint16(0x4020), modeChannel.shift, "the short mode feeds back bit 6")
}

func TestOutputRequiresLengthAndShiftBitZero(t *testing.T) {
	n := New()
	n.SetEnabled(true)
	n.Write(0x400c, 0x1f) // constant volume 15
	n.CommitLengthWrites()
	n.Write(0x400f, 0x00)
	n.CommitLengthWrites()
	n.shift = 0

	assert.Equal(t, byte(15), n.Output())

	n.shift = 1
	assert.Equal(t, byte(0), n.Output(), "bit 0 of the shift register silences the channel")

	n.shift = 0
	n.SetEnabled(false)
	assert.Equal(t, byte(0), n.Output(), "the length counter is zero")
}

func TestLengthCounterHaltsWithEnvelopeLoop(t *testing.T) {
	n := New()
	n.SetEnabled(true)
	n.Write(0x400f, 0x08) // length index 1
	n.CommitLengthWrites()
	n.Write(0x400c, 0x20) // loop flag set, which halts the length counter
	n.CommitLengthWrites()

	for range 300 {
		n.ClockHalfFrame()
	}

	assert.True(t, n.LengthActive())
}

func TestReset(t *testing.T) {
	n := New()
	n.SetEnabled(true)
	n.Write(0x400c, 0xff)
	n.CommitLengthWrites()
	n.Write(0x400e, 0x8f)
	n.CommitLengthWrites()
	n.Write(0x400f, 0x00)
	n.CommitLengthWrites()

	n.Reset()

	assert.False(t, n.LengthActive())
	assert.True(t, n.mode)
	assert.Equal(t, powerOnShiftRegister, n.shift)
	assert.Equal(t, ntscPeriodTable[0], n.timer)
}
