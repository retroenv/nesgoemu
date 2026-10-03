package envelope

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestStartLoadsDecayAtMaximum(t *testing.T) {
	env := New()

	env.Write(0x00)
	env.Start()
	env.Clock()

	assert.Equal(t, byte(15), env.Output())
}

func TestDecayUsesVolumeAsDividerPeriod(t *testing.T) {
	env := New()
	env.Write(0x02)
	env.Start()

	env.Clock()
	assert.Equal(t, byte(15), env.Output())

	env.Clock()
	env.Clock()
	assert.Equal(t, byte(15), env.Output())

	env.Clock()
	assert.Equal(t, byte(14), env.Output())
}

func TestDecayStopsAtZeroWithoutLoop(t *testing.T) {
	env := New()
	env.Write(0x00)
	env.Start()

	for range 16 {
		env.Clock()
	}
	assert.Equal(t, byte(0), env.Output())

	env.Clock()
	assert.Equal(t, byte(0), env.Output())
}

func TestLoopReloadsDecay(t *testing.T) {
	env := New()
	env.Write(0x20)
	env.Start()

	for range 16 {
		env.Clock()
	}
	assert.Equal(t, byte(0), env.Output())

	env.Clock()
	assert.Equal(t, byte(15), env.Output())
}

func TestConstantVolumeIgnoresDecay(t *testing.T) {
	env := New()
	env.Write(0x1f)

	for range 32 {
		env.Clock()
		assert.Equal(t, byte(15), env.Output())
	}
}

func TestParameterWriteKeepsDecay(t *testing.T) {
	// Only a length-load write sets the start flag.
	// https://www.nesdev.org/wiki/APU_Envelope
	env := New()
	env.Write(0)
	env.Start()
	for range 6 {
		env.Clock()
	}
	assert.Equal(t, byte(10), env.Output())

	env.Write(0)
	env.Clock()
	assert.Equal(t, byte(9), env.Output())
}

func TestConstantVolumeKeepsEnvelopeRunning(t *testing.T) {
	env := New()
	env.Write(0x10)
	env.Start()
	for range 6 {
		env.Clock()
	}
	assert.Equal(t, byte(0), env.Output())

	env.Write(0)
	assert.Equal(t, byte(10), env.Output())
	env.Clock()
	assert.Equal(t, byte(9), env.Output())
}

func TestLoopFlagFollowsBitFive(t *testing.T) {
	env := New()

	env.Write(0x20)
	assert.True(t, env.Loop())

	env.Write(0x00)
	assert.False(t, env.Loop())
}

func TestReset(t *testing.T) {
	env := New()
	env.Write(0x3f)
	env.Clock()

	env.Reset()

	assert.True(t, env.Loop())
	assert.Equal(t, byte(15), env.Output())
	env.Write(0x2f)
	assert.Equal(t, byte(0), env.Output(), "reset clears the decay counter")
}
