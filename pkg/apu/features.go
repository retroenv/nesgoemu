package apu

import (
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
)

// Features returns the APU feature inventory in declaration order.
func (a *APU) Features() []feature.Usage {
	return a.features.Features()
}

func (a *APU) declareFeatures() {
	a.features.Declare(feature.Pulse1)
	a.features.Declare(feature.Pulse2)
	a.features.Declare(feature.Triangle)
	a.features.Declare(feature.Noise)
	a.features.Declare(feature.DMC)
	a.features.Declare(feature.PulseSweep)
	a.features.Declare(feature.NoiseShortMode)
	a.features.Declare(feature.DMCLoop)
	a.features.Declare(feature.DMCIRQ)
	a.features.Declare(feature.DMCDirectLoad)
	a.features.Declare(feature.DMCSamples)
	a.features.Declare(feature.FrameFiveStep)
}

func (a *APU) markFeatures(address uint16, value byte) {
	switch address {
	case register.APU_SND_CHN:
		channels := [...]feature.ID{feature.Pulse1, feature.Pulse2, feature.Triangle, feature.Noise, feature.DMC}
		for bit, id := range channels {
			if value&(1<<bit) != 0 {
				a.features.Mark(id)
			}
		}

	case register.APU_PL1_SWEEP, register.APU_PL2_SWEEP:
		if value&0x80 != 0 {
			a.features.Mark(feature.PulseSweep)
		}

	case register.APU_NOISE_LO:
		if value&0x80 != 0 {
			a.features.Mark(feature.NoiseShortMode)
		}

	case register.APU_DMC_FREQ:
		if value&0x40 != 0 {
			a.features.Mark(feature.DMCLoop)
		}

	case register.APU_DMC_RAW:
		// A zero write is common at start-up and does not show PCM playback.
		// This is an emulator choice.
		if value&0x7f != 0 {
			a.features.Mark(feature.DMCDirectLoad)
		}

	case register.APU_FRAME:
		if value&0x80 != 0 {
			a.features.Mark(feature.FrameFiveStep)
		}
	}
}

// markDMCFetch marks the DMC features that a sample DMA read shows. The read
// shows sample playback. The interrupt flag shows that the last byte of a
// sample ended with an enabled interrupt.
func (a *APU) markDMCFetch() {
	a.features.Mark(feature.DMCSamples)

	if a.dmc.IRQ() {
		a.features.Mark(feature.DMCIRQ)
	}
}
