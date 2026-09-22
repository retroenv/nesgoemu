package nes

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestWAVRecordingIsIndependentOfChunkBoundaries(t *testing.T) {
	fullSystem := newAudioTestSystem(t, toneProgram)
	splitSystem := newAudioTestSystem(t, toneProgram)
	var full, first, second bytes.Buffer
	assert.NoError(t, fullSystem.WriteWAV(t.Context(), &full, 4096))
	assert.NoError(t, splitSystem.WriteWAV(t.Context(), &first, 1234))
	assert.NoError(t, splitSystem.WriteWAV(t.Context(), &second, 4096-1234))
	assert.Equal(t, "RIFF", string(full.Bytes()[:4]))
	assert.Equal(t, "WAVE", string(full.Bytes()[8:12]))
	assert.Equal(t, uint32(44100), binary.LittleEndian.Uint32(full.Bytes()[24:]))
	assert.Equal(t, uint32(8192), binary.LittleEndian.Uint32(full.Bytes()[40:]))
	assert.Equal(t, 44+8192, full.Len())
	combined := append([]byte{}, first.Bytes()[44:]...)
	combined = append(combined, second.Bytes()[44:]...)
	assert.Equal(t, full.Bytes()[44:], combined)
	assert.Equal(t, uint64(0), fullSystem.AudioStats().Dropped)
	assert.Equal(t, uint64(0), fullSystem.AudioStats().Silence)
}

func TestWAVRecordingCanBeCancelled(t *testing.T) {
	sys := newAudioTestSystem(t, toneProgram)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output bytes.Buffer
	assert.ErrorIs(t, sys.WriteWAV(ctx, &output, 100), context.Canceled)
	assert.Equal(t, 0, output.Len())
}

func TestWAVRecordingReportsWriteFailure(t *testing.T) {
	sys := newAudioTestSystem(t, toneProgram)
	errWrite := errors.New("write failed")
	assert.ErrorIs(t, sys.WriteWAV(t.Context(), failedRecording{errWrite}, 100), errWrite)
}

type failedRecording struct {
	err error
}

func (w failedRecording) Write(_ []byte) (int, error) {
	return 0, w.err
}
