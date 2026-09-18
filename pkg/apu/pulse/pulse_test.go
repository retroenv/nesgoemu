package pulse

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/apu/sweep"
	"github.com/retroenv/retrogolib/assert"
)

func TestWriteDutyAndRegisters(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.SetEnabled(true)

	p.Write(0x4000, 0xbf) // duty 2, constant volume 15
	p.Write(0x4002, 0xfd)
	p.Write(0x4003, 0x00)

	assert.Equal(t, byte(2), p.duty)
	assert.Equal(t, uint16(0xfd), p.timer)
	assert.True(t, p.LengthActive())
}

func TestTimerHighSetsHighBits(t *testing.T) {
	p := New(sweep.OnesComplement)

	p.Write(0x4002, 0xff)
	p.Write(0x4003, 0x07)

	assert.Equal(t, uint16(0x7ff), p.timer)
}

func TestClockAdvancesSequencerAfterPeriod(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.Write(0x4002, 0x03) // period 3, so the sequencer advances every 4 cycles

	initial := p.sequence
	for range 4 {
		p.Clock()
	}

	assert.Equal(t, (initial+7)%8, p.sequence)
}

func TestClockCountsDownThroughPeriod(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.Write(0x4002, 0x03)

	p.Clock()
	assert.Equal(t, uint16(3), p.counter)

	p.Clock()
	assert.Equal(t, uint16(2), p.counter)
}

func TestOutputFollowsDutySequence(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.Write(0x4000, 0x1f) // duty 0, constant volume 15
	p.Write(0x4002, 0x0f)
	p.Write(0x4003, 0x00)
	p.SetEnabled(true)
	p.Write(0x4003, 0x00)

	outputs := make([]byte, 0, 8)
	for range 8 {
		outputs = append(outputs, p.Output())
		p.sequence = (p.sequence + 7) % 8
	}

	// Duty 0 reads the sequence 0, 7, 6, 5, 4, 3, 2, 1 and outputs one step
	// of 15 per period.
	assert.Equal(t, []byte{0, 15, 0, 0, 0, 0, 0, 0}, outputs)
}

func TestOutputIsZeroForLowPeriods(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.Write(0x4000, 0x1f)
	p.SetEnabled(true)
	p.Write(0x4003, 0x00)
	p.Write(0x4002, 0x07)

	assert.Equal(t, byte(0), p.Output())
}

func TestOutputIsZeroWithoutLength(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.Write(0x4000, 0x1f)
	p.Write(0x4002, 0x0f)

	assert.Equal(t, byte(0), p.Output())
}

func TestLengthLoadRestartsSequencer(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.SetEnabled(true)
	p.Write(0x4002, 0x0f)

	p.Clock()
	assert.NotEqual(t, byte(0), p.sequence)

	p.Write(0x4003, 0x00)
	assert.Equal(t, byte(0), p.sequence)
}

func TestLengthCounterHaltsWithEnvelopeLoop(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.SetEnabled(true)
	p.Write(0x4003, 0x08) // length index 1
	p.Write(0x4000, 0x20) // loop flag set, which halts the length counter

	for range 300 {
		p.ClockHalfFrame()
	}

	assert.True(t, p.LengthActive())
}

func TestReset(t *testing.T) {
	p := New(sweep.OnesComplement)
	p.SetEnabled(true)
	p.Write(0x4000, 0xff)
	p.Write(0x4002, 0xff)
	p.Write(0x4003, 0x07)

	p.Reset()

	assert.False(t, p.LengthActive())
	assert.Equal(t, byte(0), p.duty)
	assert.Equal(t, uint16(0), p.timer)
	assert.Equal(t, byte(0), p.Output())
}
