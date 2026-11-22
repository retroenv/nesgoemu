package apu

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestSnapshotKeepsIRQAndChannelState(t *testing.T) {
	const (
		channelsEnabled    = 0x0f
		firstFrameIRQCycle = 29828
	)
	a, _ := newTestAPU()
	a.Write(register.APU_SND_CHN, channelsEnabled)
	a.Write(register.APU_PL1_HI, 0)
	a.Step(firstFrameIRQCycle)
	before := a.Snapshot()
	assert.True(t, before.Frame.IRQ)
	assert.Equal(t, byte(channelsEnabled), before.Registers[register.APU_SND_CHN-register.APU_PL1_VOL])
	assert.Equal(t, before, a.Snapshot())
	before.Registers[0] = 255
	assert.Equal(t, byte(0), a.Snapshot().Registers[0])
}
