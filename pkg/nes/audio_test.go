package nes

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/audio"
	"github.com/retroenv/retrogolib/gui"
)

// toneProgram enables pulse 1 with a period that plays a 440 Hz tone.
var toneProgram = []byte{
	0xa9, 0x01, 0x8d, 0x15, 0x40, // LDA #$01, STA $4015
	0xa9, 0xbf, 0x8d, 0x00, 0x40, // LDA #$BF, STA $4000
	0xa9, 0xfd, 0x8d, 0x02, 0x40, // LDA #$FD, STA $4002
	0xa9, 0x00, 0x8d, 0x03, 0x40, // LDA #$00, STA $4003
	0x4c, 0x14, 0x80, // JMP $8014
}

// loopProgram runs an endless loop without sound output.
var loopProgram = []byte{0x4c, 0x00, 0x80}

func TestAudioFormat(t *testing.T) {
	sys := newAudioTestSystem(t, loopProgram)

	format := sys.AudioFormat()

	assert.Equal(t, apu.SampleRate, format.SampleRate)
	assert.Equal(t, 1, format.Channels)
	assert.Equal(t, audioSamplesPerCallback, format.Samples)
	assert.Equal(t, audio.FormatS16, format.Format)
	assert.NoError(t, format.Validate())
}

func TestAudioPaused(t *testing.T) {
	sys := newAudioTestSystem(t, loopProgram)

	assert.False(t, sys.AudioPaused())
}

func TestAudioCallbackWritesSilenceWithoutSamples(t *testing.T) {
	sys := newAudioTestSystem(t, loopProgram)
	buffer := make([]byte, 4)

	sys.AudioCallback(buffer)

	assert.Equal(t, []byte{0, 0, 0, 0}, buffer)
}

func TestAudioCallbackPlaysEnabledPulse(t *testing.T) {
	sys := newAudioTestSystem(t, toneProgram)
	result := sys.Run(context.Background(), RunOptions{
		Frames:    5,
		MaxCycles: 200_000,
	})
	assert.Equal(t, ExitSuccess, result.Reason)

	samples := drainSamples(t, sys, 2048)

	assert.Greater(t, peak(samples), 1000, "the pulse channel is audible")
	measured := measureFrequency(samples[256:], apu.SampleRate)
	assert.LessOrEqual(t, math.Abs(measured-440.4), 15, "the period 0xFD plays 440 Hz")
}

func TestAudioCallbackIsSilentWithoutChannels(t *testing.T) {
	sys := newAudioTestSystem(t, loopProgram)
	result := sys.Run(context.Background(), RunOptions{
		Frames:    5,
		MaxCycles: 200_000,
	})
	assert.Equal(t, ExitSuccess, result.Reason)

	samples := drainSamples(t, sys, 2048)

	// The output filters remove the initial level of the triangle channel.
	for index := 1024; index < len(samples); index++ {
		assert.Equal(t, int16(0), samples[index])
	}
}

func TestAudioEnabled(t *testing.T) {
	tests := []struct {
		name     string
		options  *Options
		renderer bool
		expected bool
	}{
		{"gui mode", NewOptions(), true, true},
		{"without renderer", NewOptions(), false, false},
		{"console mode", NewOptions(WithDisabledGUI()), true, false},
		{"muted", NewOptions(WithDisabledAudio()), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.renderer {
				setTestRenderer(t, testRenderer(nil))
			} else {
				setTestRenderer(t, nil)
			}

			assert.Equal(t, tt.expected, audioEnabled(tt.options))
		})
	}
}

func TestStartAudioStartsPlayback(t *testing.T) {
	sys := newAudioTestSystem(t, loopProgram)
	started := false
	setTestRenderer(t, func(backend audio.Backend) (*audio.Playback, error) {
		assert.NoError(t, backend.AudioFormat().Validate())
		return testPlayback(&started), nil
	})

	playback, err := startAudio(sys)

	assert.NoError(t, err)
	assert.NotNil(t, playback)
	assert.True(t, started)
}

func TestStartAudioReportsSetupError(t *testing.T) {
	sys := newAudioTestSystem(t, loopProgram)
	setTestRenderer(t, func(audio.Backend) (*audio.Playback, error) {
		return nil, errors.New("no audio device")
	})

	playback, err := startAudio(sys)

	assert.Nil(t, playback)
	assert.ErrorContains(t, err, "no audio device")
	assert.ErrorContains(t, err, "setting up audio output")
}

func TestStartAudioReportsStartError(t *testing.T) {
	sys := newAudioTestSystem(t, loopProgram)
	stopped := false
	setTestRenderer(t, func(audio.Backend) (*audio.Playback, error) {
		return &audio.Playback{
			Start:  func() error { return errors.New("device is busy") },
			Stop:   func() { stopped = true },
			Errors: make(chan error, 1),
		}, nil
	})

	playback, err := startAudio(sys)

	assert.Nil(t, playback)
	assert.ErrorContains(t, err, "device is busy")
	assert.True(t, stopped, "a failed start releases the device")
}

func TestPlaybackErrorsWithoutPlayback(t *testing.T) {
	assert.True(t, playbackErrors(nil) == nil)
}

func TestRunRendererReturnsAudioError(t *testing.T) {
	sys := newAudioTestSystem(t, loopProgram)
	audioErrors := make(chan error, 1)
	audioErrors <- errors.New("audio device lost")

	starter := func(gui.Backend) (func() (bool, error), func(), error) {
		return func() (bool, error) { return true, nil }, func() {}, nil
	}

	err := sys.runRenderer(t.Context(), NewOptions(), starter, audioErrors)

	assert.ErrorContains(t, err, "audio device lost")
}

func TestWithDisabledAudio(t *testing.T) {
	opts := NewOptions(WithDisabledAudio())

	assert.True(t, opts.noAudio)
}

func TestIRQSourcesRemainIndependent(t *testing.T) {
	for _, clearAPU := range []bool{false, true} {
		sys := newAudioTestSystem(t, loopProgram)
		sys.Bus.SetMapperIRQ(true)
		sys.Bus.APU.Step(29828)

		if clearAPU {
			sys.Bus.APU.Read(0x4015)
		} else {
			sys.Bus.SetMapperIRQ(false)
			sys.Bus.APU.Step(1)
		}
		sys.CPU.Flags.I = 0
		assert.NoError(t, sys.CPU.Step())
		assert.True(t, sys.CPU.CheckInterrupts())

		sys.Bus.SetMapperIRQ(false)
		sys.Bus.APU.Read(0x4015)
		sys.CPU.Flags.I = 0
		assert.False(t, sys.CPU.CheckInterrupts())
	}
}

func TestUltrasonicTriangleDoesNotAlias(t *testing.T) {
	for _, period := range []byte{0, 1} {
		sys := newAudioTestSystem(t, loopProgram)
		sys.Bus.APU.Write(0x4015, 4)
		sys.Bus.APU.Write(0x4008, 0xff)
		sys.Bus.APU.Write(0x400a, period)
		sys.Bus.APU.Write(0x400b, 0)
		sys.Bus.APU.Write(0x4017, 0xc0)
		sys.Bus.APU.Step(400_000)

		samples := drainSamples(t, sys, 2048)
		for _, sample := range samples {
			assert.LessOrEqual(t, math.Abs(float64(sample)), 4.0, "ultrasonic output must not become an audible tone")
		}
	}
}

// newAudioTestSystem returns a system that runs the given program from $8000.
func newAudioTestSystem(t *testing.T, program []byte) *System {
	t.Helper()

	cart := cartridge.New()
	cart.PRG = make([]byte, 32*1024)
	cart.CHR = make([]byte, 8*1024)
	copy(cart.PRG, program)
	cart.PRG[0x7ffc] = 0x00
	cart.PRG[0x7ffd] = 0x80

	sys, err := NewSystem(NewOptions(WithCartridge(cart), WithSavePath("")))
	assert.NoError(t, err)

	return sys
}

// setTestRenderer replaces the audio renderer for one test.
func setTestRenderer(t *testing.T, setup audio.Initializer) {
	t.Helper()

	previous := audio.Setup
	t.Cleanup(func() { audio.Setup = previous })
	audio.Setup = setup
}

// testRenderer returns a renderer that reports a working playback.
func testRenderer(started *bool) audio.Initializer {
	return func(audio.Backend) (*audio.Playback, error) {
		return testPlayback(started), nil
	}
}

// testPlayback returns a playback that records the start request.
func testPlayback(started *bool) *audio.Playback {
	return &audio.Playback{
		Start: func() error {
			if started != nil {
				*started = true
			}
			return nil
		},
		Stop:   func() {},
		Errors: make(chan error, 1),
	}
}

// drainSamples returns the samples that the playback callback receives.
func drainSamples(t *testing.T, sys *System, frames int) []int16 {
	t.Helper()

	buffer := make([]byte, frames*2)
	sys.AudioCallback(buffer)

	samples := make([]int16, frames)
	for index := range frames {
		samples[index] = int16(binary.LittleEndian.Uint16(buffer[index*2:]))
	}

	return samples
}

// measureFrequency finds the strongest frequency in a windowed spectrum.
// Filter ringing can add zero crossings without changing the tone frequency.
func measureFrequency(samples []int16, sampleRate int) float64 {
	var best, frequency float64
	for bin := 1; bin < len(samples)/2; bin++ {
		var realPart, imaginaryPart float64
		for index, sample := range samples {
			window := 0.5 - 0.5*math.Cos(2*math.Pi*float64(index)/float64(len(samples)-1))
			phase := 2 * math.Pi * float64(bin*index) / float64(len(samples))
			value := float64(sample) * window
			realPart += value * math.Cos(phase)
			imaginaryPart += value * math.Sin(phase)
		}
		power := realPart*realPart + imaginaryPart*imaginaryPart
		if power > best {
			best = power
			frequency = float64(bin*sampleRate) / float64(len(samples))
		}
	}
	return frequency
}

// peak returns the highest sample value.
func peak(samples []int16) int {
	highest := 0
	for _, sample := range samples {
		highest = max(highest, int(sample))
	}

	return highest
}
