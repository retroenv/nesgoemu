// Package renderstate handles PPU render handling of cycles, scan lines and frames.
package renderstate

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/internal/stateio"
)

const (
	// FirstVisibleDot and LastVisibleDot bound the visible dots of a scanline.
	FirstVisibleDot = 1
	LastVisibleDot  = 256

	// FirstBackgroundPrefetchDot and LastBackgroundPrefetchDot bound the
	// background tile fetches for the next scanline.
	FirstBackgroundPrefetchDot = 321
	LastBackgroundPrefetchDot  = 336

	// VisibleScanlines is the number of scanlines that show the picture.
	VisibleScanlines = 240

	// PreRenderScanline is the last scanline of an NTSC frame.
	PreRenderScanline = 261
)

const lastScanlineDot = 340

type mask interface {
	RenderBackground() bool
	RenderSprites() bool
}

// RenderState implements a PPU render state manager.
type RenderState struct {
	cycle    int // 0-340, 0=idle,1-336=tile data fetching,337-340=nameTable fetching
	scanLine int // 0-261, 0-239=visible, 240=post-render, 241-260=vertical blank, 261=pre-render
	frame    uint64
}

// New returns a new render state manager.
func New() *RenderState {
	return &RenderState{
		cycle:    lastScanlineDot,
		scanLine: VisibleScanlines,
	}
}

// Tick updates cycle, scanLine and frame counters.
func (r *RenderState) Tick(mask mask) {
	r.TickRendering(mask.RenderBackground() || mask.RenderSprites())
}

// TickRendering updates the counters with the current rendering state.
func (r *RenderState) TickRendering(rendering bool) {
	if rendering {
		// for odd frames, the cycle at the end of the scanline is skipped
		if r.scanLine == PreRenderScanline && r.cycle == 339 && r.frame%2 == 1 {
			r.nextFrame()
			return
		}
	}

	r.cycle++
	if r.cycle <= lastScanlineDot {
		return
	}

	r.cycle = 0

	r.scanLine++
	if r.scanLine <= PreRenderScanline {
		return
	}

	r.nextFrame()
}

// Cycle returns the current cycle, possible values are 0-340.
func (r *RenderState) Cycle() int {
	return r.cycle
}

// ScanLine returns the current scanline, possible values are 0-261.
func (r *RenderState) ScanLine() int {
	return r.scanLine
}

// Frame returns the number of frames that reached the first visible scanline.
func (r *RenderState) Frame() uint64 {
	return r.frame
}

// TransferState transfers stopped RenderState state. The system owns the record version.
func (r *RenderState) TransferState(archive *stateio.Archive) error {
	if err := archive.Values(&r.cycle, &r.scanLine, &r.frame); err != nil {
		return fmt.Errorf("transferring renderstate state: %w", err)
	}
	if archive.Reading() && (r.cycle < 0 || r.cycle > lastScanlineDot || r.scanLine < 0 || r.scanLine > PreRenderScanline) {
		return errors.New("invalid PPU position snapshot")
	}

	return nil
}

func (r *RenderState) nextFrame() {
	r.cycle = 0
	r.scanLine = 0
	r.frame++
}
