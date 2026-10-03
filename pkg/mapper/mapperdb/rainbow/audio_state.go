package rainbow

type audioPulse struct {
	period   uint16
	timer    uint16
	sequence byte
	duty     byte
	volume   byte
	mode     bool
	enabled  bool
}

type audioSaw struct {
	period      uint16
	timer       uint16
	rate        byte
	step        byte
	accumulator byte
	enabled     bool
}

type audioState struct {
	pulse1        audioPulse
	pulse2        audioPulse
	saw           audioSaw
	outputControl byte
	masterVolume  byte
	cycle         uint64
	observer      func(ExpansionAudioWrite)
}

func (p *audioPulse) clock() {
	if !p.enabled {
		return
	}
	if p.timer != 0 {
		p.timer--
		return
	}
	p.timer = p.period
	p.sequence = (p.sequence - 1) & 0x0F
}

func (p *audioPulse) output() byte {
	if !p.enabled || (!p.mode && p.sequence > p.duty) {
		return 0
	}
	return p.volume
}

func (p *audioPulse) writeControl(value byte) {
	p.mode = value&audioPulseModeBit != 0
	p.duty = (value & audioPulseDutyMask) >> audioPulseDutyShift
	p.volume = value & audioPulseVolumeMask
}

func (p *audioPulse) writeLow(value byte) {
	p.period = p.period&0x0F00 | uint16(value)
}

func (p *audioPulse) writeHigh(value byte) {
	p.period = p.period&0x00FF | uint16(value&audioPeriodHighMask)<<8
	p.enabled = value&audioEnableBit != 0
	if !p.enabled {
		p.sequence = 15
		p.timer = p.period
	}
}

func (s *audioSaw) clock() {
	if !s.enabled {
		return
	}
	if s.timer != 0 {
		s.timer--
		return
	}
	s.timer = s.period
	s.step++
	if s.step >= 14 {
		s.step = 0
		s.accumulator = 0
	} else if s.step&1 == 0 {
		s.accumulator += s.rate
	}
}

func (s *audioSaw) output() byte {
	if !s.enabled {
		return 0
	}
	return s.accumulator >> 3
}

func (s *audioSaw) writeLow(value byte) {
	s.period = s.period&0x0F00 | uint16(value)
}

func (s *audioSaw) writeHigh(value byte) {
	s.period = s.period&0x00FF | uint16(value&audioPeriodHighMask)<<8
	s.enabled = value&audioEnableBit != 0
	if !s.enabled {
		s.step = 0
		s.accumulator = 0
	}
}

func (a *audioState) combinedOutput() byte {
	return a.pulse1.output() + a.pulse2.output() + a.saw.output()
}

func (a *audioState) pcmOutput() byte {
	if a.outputControl&audioOutputIPCM == 0 {
		return 0
	}
	level := uint16(a.combinedOutput()) * uint16(a.masterVolume) / audioMasterVolumeMask
	return byte(level * 2)
}

func (a *audioState) valid() bool {
	return a.pulse1.period <= 0x0FFF && a.pulse2.period <= 0x0FFF && a.saw.period <= 0x0FFF &&
		a.pulse1.sequence <= 15 && a.pulse2.sequence <= 15 && a.saw.step < 14 &&
		a.masterVolume <= audioMasterVolumeMask &&
		a.outputControl&^(audioOutputEXP6|audioOutputEXP9|audioOutputIPCM|audioOutputEPSM) == 0
}
