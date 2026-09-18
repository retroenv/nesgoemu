package envelope

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestWriteStartsDecayAtMaximum(t *testing.T) {
	env := New()

	env.Write(0x00)
	env.Clock()

	assert.Equal(t, byte(15), env.Output())
}

func TestDecayUsesVolumeAsDividerPeriod(t *testing.T) {
	env := New()
	env.Write(0x02)

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

	assert.False(t, env.Loop())
	assert.Equal(t, byte(0), env.Output())
}
