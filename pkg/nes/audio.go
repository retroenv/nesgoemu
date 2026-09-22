package nes

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/apu/output"
	"github.com/retroenv/retrogolib/audio"
)

// audioSamplesPerCallback is the number of sample frames that the audio backend
// requests per callback. The SDL2 renderer requires a power of two.
const audioSamplesPerCallback = 1024

// AudioFormat returns the audio format of the system.
func (sys *System) AudioFormat() audio.Format {
	return audio.Format{
		SampleRate: apu.SampleRate,
		Channels:   1,
		Samples:    audioSamplesPerCallback,
		Format:     audio.FormatS16,
	}
}

// AudioCallback fills the buffer with the samples of the APU.
// The function writes silence when the APU has no samples queued. The playback
// worker calls this method, which can overlap the emulation goroutine.
func (sys *System) AudioCallback(buffer []byte) {
	sys.apu.FillSamples(buffer)
}

// AudioPaused reports whether audio output is paused. The system plays audio
// while the playback is active.
func (sys *System) AudioPaused() bool {
	return false
}

// AudioStats returns the APU queue counters. The SDL queue is separate.
func (sys *System) AudioStats() output.Stats {
	return sys.apu.AudioStats()
}

// audioEnabled reports whether the system opens an audio device. Audio output
// needs a registered renderer, the GUI, and an unmuted system.
func audioEnabled(opts *Options) bool {
	return audio.Setup != nil && !opts.noGui && !opts.noAudio
}

// startAudio opens the audio device and starts playback.
func startAudio(sys *System) (*audio.Playback, error) {
	playback, err := audio.Setup(sys)
	if err != nil {
		return nil, fmt.Errorf("setting up audio output: %w", err)
	}
	if err := playback.Start(); err != nil {
		playback.Stop()
		return nil, fmt.Errorf("starting audio playback: %w", err)
	}

	return playback, nil
}

// playbackErrors returns the error channel of the playback. A nil playback
// returns a nil channel, which never becomes ready in a select statement.
func playbackErrors(playback *audio.Playback) <-chan error {
	if playback == nil {
		return nil
	}

	return playback.Errors
}
