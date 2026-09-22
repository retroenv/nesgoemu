package nes

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/retroenv/nesgoemu/pkg/apu"
)

// WriteWAV runs the system from its current state and writes exactly frames
// mono S16 samples at 44100 Hz. It includes any samples already in the queue.
// Call it with emulation and playback stopped. CPU cycles control the output;
// wall-clock timing and audio hardware do not affect the recording.
func (sys *System) WriteWAV(ctx context.Context, destination io.Writer, frames uint64) error {
	if frames > (math.MaxUint32-36)/2 {
		return errors.New("audio recording exceeds the WAV size limit")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("starting audio recording: %w", err)
	}
	writer := bufio.NewWriter(destination)
	header := waveHeader(uint32(frames))
	if _, err := writer.Write(header[:]); err != nil {
		return fmt.Errorf("writing WAV header: %w", err)
	}
	if err := sys.writePCM(ctx, writer, frames); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing WAV output: %w", err)
	}
	return nil
}

func (sys *System) writePCM(ctx context.Context, destination io.Writer, frames uint64) error {
	var buffer [2048]byte
	for frames > 0 {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("recording audio: %w", err)
		}
		available := sys.apu.DrainSamples(buffer[:min(frames, 1024)*2])
		if available > 0 {
			if _, err := destination.Write(buffer[:available*2]); err != nil {
				return fmt.Errorf("writing audio samples: %w", err)
			}
			frames -= uint64(available)
			continue
		}
		end := sys.CPU.Cycles() + 1024
		for sys.CPU.Cycles() < end {
			if _, err := sys.StepSystem(); err != nil {
				return err
			}
		}
	}
	return nil
}

func waveHeader(frames uint32) [44]byte {
	var header [44]byte
	copy(header[:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], 36+frames*2)
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 1)
	binary.LittleEndian.PutUint32(header[24:], apu.SampleRate)
	binary.LittleEndian.PutUint32(header[28:], apu.SampleRate*2)
	binary.LittleEndian.PutUint16(header[32:], 2)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], frames*2)
	return header
}
