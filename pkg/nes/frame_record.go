package nes

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu"
)

// FrameRecord describes what one frame period of a real run produced. The
// period runs from scanline 241 dot 1 of the previous picture to the same
// point of this picture, so the picture and the bus traffic that made it are
// in one record. See ppu.FrameCapture for the picture and the PPUMASK spans.
//
// Accesses counts what happened in the period. An access in progress when the
// picture completes is counted in the next period. The first record after
// ObserveFrames is Partial and counts only from the start of observation.
type FrameRecord struct {
	ppu.FrameCapture

	Accesses memory.AccessCounts
}

// Clone returns a copy that stays valid after the emulator reuses the buffers.
func (r *FrameRecord) Clone() FrameRecord {
	return FrameRecord{
		FrameCapture: r.FrameCapture.Clone(),
		Accesses:     r.Accesses,
	}
}

// ObserveFrames replaces the optional observer for frame records. The
// observer runs on the emulation goroutine at the end of each picture, and
// the record is valid only during the call; use Clone to keep it. It reads the
// system and does not change emulation results: the same program gives the
// same frames, cycles and bus traffic with and without it. A nil observer
// disables it and its counting. It reports false when the PPU cannot capture
// frames. The observer must not change system state.
// Call ObserveFrames on the emulation goroutine, or before it starts.
func (sys *System) ObserveFrames(observer func(*FrameRecord)) bool {
	return observeFrames(sys.Bus, sys.memory, observer)
}

// ObserveFrames replaces the optional observer for frame records. See
// System.ObserveFrames. The records count the bus accesses that the caller
// makes through Read and Write.
func (n *Native) ObserveFrames(observer func(*FrameRecord)) bool {
	return observeFrames(n.bus, n.memory, observer)
}

// frameObservable is implemented by the PPU of this module.
type frameObservable interface {
	ObserveFrames(observer func(*ppu.FrameCapture))
}

// observeFrames connects the optional frame record callback of a device set.
// It reports false when the PPU cannot capture frames.
func observeFrames(b *bus.Bus, m *memory.Memory, observer func(*FrameRecord)) bool {
	capturer, ok := b.PPU.(frameObservable)
	if !ok {
		return false
	}
	if observer == nil {
		capturer.ObserveFrames(nil)
		m.CountAccesses(nil)
		return true
	}

	counts := &memory.AccessCounts{}
	var record FrameRecord
	m.CountAccesses(counts)
	capturer.ObserveFrames(func(capture *ppu.FrameCapture) {
		record.FrameCapture = *capture
		record.Accesses = *counts
		*counts = memory.AccessCounts{}
		observer(&record)
	})
	return true
}
