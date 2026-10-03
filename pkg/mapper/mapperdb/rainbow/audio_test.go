package rainbow

import (
	"math"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestAudioPowerUpRoutesBothPhysicalOutputs(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	assert.Equal(t, byte(audioOutputEXP6|audioOutputEXP9), m.audio.outputControl)
	assert.Equal(t, byte(audioMasterVolumeMask), m.audio.masterVolume)
}

func TestAudioPulseDutyAndPeriod(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regAudioPulse1Control, 0x07) // duty 0: one high step; volume 7
	m.Write(regAudioPulse1Low, 1)
	m.Write(regAudioPulse1High, audioEnableBit)

	output := make([]byte, 0, 32)
	for range 32 {
		output = append(output, m.audio.pulse1.output())
		m.ClockCPU(1)
	}

	high := 0
	for _, value := range output {
		if value == 7 {
			high++
		}
	}
	assert.Equal(t, 2, high, "period 1 clocks 16 sequencer steps in 32 CPU cycles")
}

func TestAudioPulseModeIgnoresDuty(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regAudioPulse2Control, audioPulseModeBit|0x0B)
	m.Write(regAudioPulse2High, audioEnableBit)

	for range 32 {
		assert.Equal(t, byte(11), m.audio.pulse2.output())
		m.ClockCPU(1)
	}
}

func TestAudioSawAccumulatorSequence(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regAudioSawRate, 8)
	m.Write(regAudioSawHigh, audioEnableBit)

	want := []byte{0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 0}
	for index, expected := range want {
		m.ClockCPU(1)
		assert.Equal(t, expected, m.audio.saw.output(), "step %d", index+1)
	}
}

func TestAudioOutputRoutingAndMasterVolume(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regAudioPulse1Control, audioPulseModeBit|audioPulseVolumeMask)
	m.Write(regAudioPulse1High, audioEnableBit)

	full := m.ExpansionAudioOutput()
	assert.True(t, math.Abs(full-audioPulseFullScale) < 0.000001)

	m.Write(regAudioMasterVolume, 7)
	assert.True(t, math.Abs(m.ExpansionAudioOutput()-full*7.0/15.0) < 0.000001)

	m.Write(regAudioOutputControl, audioOutputIPCM)
	assert.Equal(t, 0.0, m.ExpansionAudioOutput(), "IPCM alone does not drive a physical expansion pin")
	assert.Equal(t, byte(14), m.ReadPCM(), "IPCM returns the master-volume-scaled six-bit DAC value times two")
}

func TestAudioDisableResetsChannelPhase(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regAudioPulse1Control, 0x0F)
	m.Write(regAudioPulse1High, audioEnableBit)
	m.ClockCPU(3)
	m.Write(regAudioPulse1High, 0)

	assert.Equal(t, byte(15), m.audio.pulse1.sequence)
	assert.Equal(t, byte(0), m.audio.pulse1.output())

	m.Write(regAudioSawRate, 8)
	m.Write(regAudioSawHigh, audioEnableBit)
	m.ClockCPU(4)
	m.Write(regAudioSawHigh, 0)
	assert.Equal(t, byte(0), m.audio.saw.step)
	assert.Equal(t, byte(0), m.audio.saw.accumulator)
}

func TestAudioRegisterObserverUsesMapperCycles(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	var writes []ExpansionAudioWrite
	m.ObserveExpansionAudioWrites(func(write ExpansionAudioWrite) { writes = append(writes, write) })
	m.ClockCPU(12)
	m.Write(regAudioSawRate, 0x7F)

	assert.Equal(t, []ExpansionAudioWrite{{Cycle: 12, Address: regAudioSawRate, Value: 0x7F}}, writes)
	assert.Equal(t, byte(audioSawRateMask), m.audio.saw.rate)
}
