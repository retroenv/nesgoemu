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
		if value&0x80 != 0 {
			a.features.Mark(feature.DMCIRQ)
		}
	case register.APU_FRAME:
		if value&0x80 != 0 {
			a.features.Mark(feature.FrameFiveStep)
		}
	}
}
