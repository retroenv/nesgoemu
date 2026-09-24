package apu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestFeaturesTrackChannelEnablesAndModes(t *testing.T) {
	a, _ := newTestAPU()

	for _, usage := range a.Features() {
		assert.False(t, usage.Used, "%s must start unused", usage.Name)
	}

	a.Write(register.APU_SND_CHN, 0)
	a.Write(register.APU_PL1_SWEEP, 0)
	a.Write(register.APU_NOISE_LO, 0)
	a.Write(register.APU_DMC_FREQ, 0)
	a.Write(register.APU_FRAME, 0)
	for _, usage := range a.Features() {
		assert.False(t, usage.Used, "%s must stay unused after zero writes", usage.Name)
	}

	a.Write(register.APU_SND_CHN, 0x15)
	a.Write(register.APU_PL2_SWEEP, 0x80)
	a.Write(register.APU_NOISE_LO, 0x80)
	a.Write(register.APU_DMC_FREQ, 0xc0)
	a.Write(register.APU_FRAME, 0x80)

	used := make(map[feature.ID]bool)
	for _, usage := range a.Features() {
		used[usage.ID] = usage.Used
	}
	for _, id := range []feature.ID{
		feature.Pulse1, feature.Triangle, feature.DMC, feature.PulseSweep,
		feature.NoiseShortMode, feature.DMCLoop, feature.DMCIRQ, feature.FrameFiveStep,
	} {
		assert.True(t, used[id], "feature %d must be used", id)
	}
	assert.False(t, used[feature.Pulse2])
	assert.False(t, used[feature.Noise])

	a.Write(register.APU_SND_CHN, 0)
	assert.True(t, a.Features()[0].Used, "later writes must not erase usage")
	a.Write(register.APU_SND_CHN, 0x0a)
	usage := a.Features()
	assert.True(t, usage[1].Used, "pulse 2 must be used")
	assert.True(t, usage[3].Used, "noise must be used")
}
