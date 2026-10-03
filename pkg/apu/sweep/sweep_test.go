package sweep

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestClockAdjustsPeriodWhenDividerExpires(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x81) // enabled, divider period 0, shift 1

	assert.Equal(t, uint16(150), s.Clock(100), "a reload does not cancel a due sweep")
	assert.Equal(t, uint16(150), s.Clock(100), "the target period is applied")
}

func TestDividerPeriodDelaysAdjustment(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x91) // enabled, divider period 1, shift 1

	assert.Equal(t, uint16(150), s.Clock(100))
	assert.Equal(t, uint16(100), s.Clock(100))
	assert.Equal(t, uint16(150), s.Clock(100))
}

func TestReloadWithRunningDividerDoesNotSweep(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x91)
	assert.Equal(t, uint16(150), s.Clock(100))

	s.Write(0x91)
	assert.Equal(t, uint16(100), s.Clock(100))
	assert.Equal(t, uint16(100), s.Clock(100))
	assert.Equal(t, uint16(150), s.Clock(100))
}

func TestDisabledUnitKeepsPeriod(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x01) // disabled, shift 1

	for range 4 {
		assert.Equal(t, uint16(100), s.Clock(100))
	}
}

func TestZeroShiftKeepsPeriod(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x80) // enabled, shift 0

	for range 4 {
		assert.Equal(t, uint16(100), s.Clock(100))
	}
}

func TestNegateModesUseDifferentChangeAmounts(t *testing.T) {
	ones := New(OnesComplement)
	ones.Write(0x89) // enabled, negate, shift 1

	twos := New(TwosComplement)
	twos.Write(0x89)

	ones.Clock(100)
	twos.Clock(100)

	assert.Equal(t, uint16(49), ones.Clock(100))
	assert.Equal(t, uint16(50), twos.Clock(100))
}

func TestMutedWhenPeriodIsBelowEight(t *testing.T) {
	s := New(OnesComplement)

	assert.True(t, s.Muted(7))
	assert.False(t, s.Muted(8))
}

func TestMutedWhenTargetPeriodIsTooLarge(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x03) // disabled, shift 3

	assert.True(t, s.Muted(0x7f0))
	assert.False(t, s.Muted(0x100))
}

func TestMutedWithoutShift(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x00) // negate clear, shift 0 doubles the period

	assert.True(t, s.Muted(0x400))
	assert.False(t, s.Muted(0x100))
}

func TestNegateAvoidsMutingWithoutShift(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x08) // negate, shift 0

	assert.False(t, s.Muted(0x7ff))
}

func TestClockDoesNotUpdatePeriodWhileMuted(t *testing.T) {
	s := New(OnesComplement)
	s.Write(0x83) // enabled, shift 3

	s.Clock(0x7f0)
	s.Clock(0x7f0)
	assert.Equal(t, uint16(0x7f0), s.Clock(0x7f0))
}

func TestReset(t *testing.T) {
	s := New(TwosComplement)
	s.Write(0xf9)

	s.Reset()

	assert.True(t, s.enabled)
	assert.True(t, s.negate)
	assert.Equal(t, byte(1), s.shift, "reset keeps the shift register setting")
	assert.Equal(t, TwosComplement, s.mode, "the negation mode is kept")
	assert.Equal(t, uint16(50), s.Clock(100))
}
