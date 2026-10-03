package rainbow

import "github.com/retroenv/nesgoemu/pkg/feature"

// Rainbow expansion-audio register behavior follows the VRC6 channels, without
// VRC6's shared frequency-control register.
// https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#sound--audio-expansion-41a0-41af
// https://www.nesdev.org/wiki/VRC6_audio
const (
	audioEnableBit        = 0x80
	audioPulseModeBit     = 0x80
	audioPulseDutyMask    = 0x70
	audioPulseDutyShift   = 4
	audioPulseVolumeMask  = 0x0F
	audioSawRateMask      = 0x3F
	audioPeriodHighMask   = 0x0F
	audioOutputEXP6       = 0x01
	audioOutputEXP9       = 0x02
	audioOutputIPCM       = 0x04
	audioOutputEPSM       = 0x80
	audioMasterVolumeMask = 0x0F

	// One maximum Rainbow pulse is approximately as loud as one maximum 2A03
	// pulse. This is the 2A03 nonlinear mixer level for one pulse at volume 15.
	audioPulseFullScale = 95.88 / (8128.0/15.0 + 100.0)
)

// ExpansionAudioWrite is one cycle-stamped Rainbow audio-register write.
type ExpansionAudioWrite struct {
	Cycle   uint64
	Address uint16
	Value   byte
}

// ExpansionAudioOutput returns the current direct-pin audio level. Enabling
// both EXP6 and EXP9 selects two possible physical routes; it does not double
// the synthesized signal.
func (m *Mapper) ExpansionAudioOutput() float64 {
	if m.audio.outputControl&(audioOutputEXP6|audioOutputEXP9) == 0 {
		return 0
	}
	volume := float64(m.audio.masterVolume) / audioMasterVolumeMask
	return float64(m.audio.combinedOutput()) / 15 * audioPulseFullScale * volume
}

// ObserveExpansionAudioWrites replaces the optional Rainbow audio-write observer.
func (m *Mapper) ObserveExpansionAudioWrites(observer func(ExpansionAudioWrite)) {
	m.audio.observer = observer
}

func (m *Mapper) clockAudio() {
	m.audio.pulse1.clock()
	m.audio.pulse2.clock()
	m.audio.saw.clock()
	m.audio.cycle++
}

func (m *Mapper) writeAudioRegister(address uint16, value byte) {
	m.MarkFeature(feature.ExpansionAudio)
	if observer := m.audio.observer; observer != nil {
		observer(ExpansionAudioWrite{
			Cycle:   m.audio.cycle,
			Address: address,
			Value:   value,
		})
	}

	switch address {
	case regAudioPulse1Control:
		m.audio.pulse1.writeControl(value)
	case regAudioPulse1Low:
		m.audio.pulse1.writeLow(value)
	case regAudioPulse1High:
		m.audio.pulse1.writeHigh(value)
	case regAudioPulse2Control:
		m.audio.pulse2.writeControl(value)
	case regAudioPulse2Low:
		m.audio.pulse2.writeLow(value)
	case regAudioPulse2High:
		m.audio.pulse2.writeHigh(value)
	case regAudioSawRate:
		m.audio.saw.rate = value & audioSawRateMask
	case regAudioSawLow:
		m.audio.saw.writeLow(value)
	case regAudioSawHigh:
		m.audio.saw.writeHigh(value)
	case regAudioOutputControl:
		m.audio.outputControl = value & (audioOutputEXP6 | audioOutputEXP9 | audioOutputIPCM | audioOutputEPSM)
	case regAudioMasterVolume:
		m.audio.masterVolume = value & audioMasterVolumeMask
	}
}
