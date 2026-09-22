package openbus

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestSetRefreshesAllBits(t *testing.T) {
	t.Parallel()

	register := New()
	register.Set(0xAB)

	assert.Equal(t, byte(0xAB), register.Value())
}

func TestBitDecaysAfterDecayTime(t *testing.T) {
	t.Parallel()

	register := New()
	register.Set(0xFF)
	register.Tick(DecayCycles - 1)

	assert.Equal(t, byte(0xFF), register.Value(), "a bit keeps its value within the decay time")

	register.Tick(1)

	assert.Equal(t, byte(0), register.Value(), "a bit that is not refreshed with a one decays to zero")
}

func TestReadDoesNotRefreshBits(t *testing.T) {
	t.Parallel()

	register := New()
	register.Set(0xFF)
	register.Tick(DecayCycles - 1)

	for range 4 {
		assert.Equal(t, byte(0xFF), register.Value())
	}

	register.Tick(1)

	assert.Equal(t, byte(0), register.Value())
}

func TestRefreshRestartsDecayTime(t *testing.T) {
	t.Parallel()

	register := New()
	register.Set(0x01)
	register.Tick(DecayCycles - 1)
	register.Set(0x01)
	register.Tick(DecayCycles - 1)

	assert.Equal(t, byte(0x01), register.Value())
}

func TestSetBitsKeepsOtherBits(t *testing.T) {
	t.Parallel()

	register := New()
	register.Set(0xFF)
	register.Tick(DecayCycles - 1)
	register.SetBits(0b0000_1111, 0b0000_1111)
	register.Tick(1)

	assert.Equal(t, byte(0b0000_1111), register.Value(), "only the refreshed bits keep their value")
}

func TestSetZeroClearsBits(t *testing.T) {
	t.Parallel()

	register := New()
	register.Set(0xFF)
	register.Set(0x00)

	assert.Equal(t, byte(0), register.Value())
}
