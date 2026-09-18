package triangle

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestSequencerAdvancesAfterPeriod(t *testing.T) {
	triangle := enabled()
	triangle.Write(0x400a, 0x02) // period 2, so the sequencer advances every 3 cycles

	initial := triangle.sequence
	for range 3 {
		triangle.Clock()
	}

	assert.Equal(t, (initial+1)%32, triangle.sequence)
}

func TestSequencerHoldsWhileSilenced(t *testing.T) {
	triangle := enabled()
	triangle.Write(0x400a, 0x02)
	triangle.Clock()
	held := triangle.sequence

	triangle.SetEnabled(false)
	for range 10 {
		triangle.Clock()
	}

	assert.Equal(t, held, triangle.sequence)
}

func TestOutputFollowsSequence(t *testing.T) {
	triangle := enabled()

	triangle.sequence = 0
	assert.Equal(t, byte(15), triangle.Output())

	triangle.sequence = 15
	assert.Equal(t, byte(0), triangle.Output())

	triangle.sequence = 16
	assert.Equal(t, byte(0), triangle.Output())

	triangle.sequence = 31
	assert.Equal(t, byte(15), triangle.Output())
}

func TestLinearCounterReloadsAndCountsDown(t *testing.T) {
	triangle := enabled()
	triangle.Write(0x4008, 0x02) // reload value 2, control clear
	triangle.Write(0x400b, 0x00) // sets the reload flag

	triangle.ClockQuarterFrame()
	assert.Equal(t, byte(2), triangle.linear)

	triangle.ClockQuarterFrame()
	assert.Equal(t, byte(1), triangle.linear)

	triangle.ClockQuarterFrame()
	triangle.ClockQuarterFrame()
	assert.Equal(t, byte(0), triangle.linear)
}

func TestLinearCounterControlKeepsReloads(t *testing.T) {
	triangle := enabled()
	triangle.Write(0x4008, 0x81) // control set, reload value 1
	triangle.Write(0x400b, 0x00)

	for range 4 {
		triangle.ClockQuarterFrame()
		assert.Equal(t, byte(1), triangle.linear)
	}
}

func TestLengthCounterSilencesSequencer(t *testing.T) {
	triangle := New()
	triangle.Write(0x4008, 0x7f)
	triangle.Write(0x400a, 0x02)
	triangle.Clock()

	sequence := triangle.sequence
	for range 10 {
		triangle.Clock()
	}

	assert.Equal(t, sequence, triangle.sequence, "the length counter is zero")
}

func TestTimerHighSetsHighBits(t *testing.T) {
	triangle := enabled()

	triangle.Write(0x400a, 0xff)
	triangle.Write(0x400b, 0x07)

	assert.Equal(t, uint16(0x7ff), triangle.timer)
}

func TestReset(t *testing.T) {
	triangle := enabled()
	triangle.Write(0x4008, 0xff)
	triangle.Write(0x400b, 0x07)

	triangle.Reset()

	assert.False(t, triangle.LengthActive())
	assert.Equal(t, byte(0), triangle.linear)
	assert.Equal(t, byte(0), triangle.reloadValue)
	assert.Equal(t, uint16(0), triangle.timer)
}

// enabled returns a triangle channel with an enabled length counter and a
// loaded linear counter.
func enabled() *Triangle {
	triangle := New()
	triangle.SetEnabled(true)
	triangle.Write(0x4008, 0x7f)
	triangle.Write(0x400b, 0x00)
	triangle.ClockQuarterFrame()

	return triangle
}
