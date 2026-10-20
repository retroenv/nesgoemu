package ppu

import (
	"fmt"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

const dotsPerFrame = DotsPerScanline * 262

func writePaletteEntry(p *PPU, index, value byte) {
	p.Write(register.PPU_ADDR, 0x3F)
	p.Write(register.PPU_ADDR, index)
	p.Write(register.PPU_DATA, value)
	// Keep the test setup from selecting a forced-blank palette override.
	p.Write(register.PPU_ADDR, 0x20)
	p.Write(register.PPU_ADDR, 0)
}

// stepToCapture steps until the observer reports one more capture.
func stepToCapture(p *PPU, captures *[]FrameCapture) {
	before := len(*captures)
	for len(*captures) == before {
		p.Step(1)
	}
}

func collectFrames(p *PPU) *[]FrameCapture {
	var captures []FrameCapture
	p.ObserveFrames(func(c *FrameCapture) { captures = append(captures, c.Clone()) })
	return &captures
}

func TestObserveFramesBlankFrameShowsBackdrop(t *testing.T) {
	p := newOpenBusTestPPU()
	writePaletteEntry(p, 0, 0x21)
	captures := collectFrames(p)

	stepToCapture(p, captures)
	stepToCapture(p, captures)
	stepToCapture(p, captures)

	assert.Len(t, *captures, 3)
	first, second, third := (*captures)[0], (*captures)[1], (*captures)[2]
	assert.True(t, first.Partial)
	assert.False(t, second.Partial)
	assert.False(t, third.Partial)
	assert.Equal(t, second.Frame+1, third.Frame)
	assert.Len(t, second.Indices, Width*Height)
	for _, index := range second.Indices {
		assert.Equal(t, byte(0x21), index)
	}
	assert.Equal(t, 0, second.Rendered())
	assert.Equal(t, []DotSpan{{Start: 0, End: VisibleDots}}, second.OffSpans(MaskBoth))
	assert.Equal(t, []DotSpan{{Start: 0, End: VisibleDots}}, second.OffSpans(MaskBackground))
	assert.Len(t, second.MaskChanges, 0)

	img := second.RGBA()
	assert.Equal(t, RGB(0x21), img.RGBAAt(0, 0))
	assert.Equal(t, RGB(0x21), img.RGBAAt(Width-1, Height-1))
}

func TestObserveFramesRenderedFrame(t *testing.T) {
	p := newOpenBusTestPPU()
	writePaletteEntry(p, 0, 0x21)
	captures := collectFrames(p)
	stepToCapture(p, captures)
	p.Write(register.PPU_MASK, MaskBackground)
	stepToCapture(p, captures)
	stepToCapture(p, captures)

	frame := (*captures)[2]
	assert.Equal(t, Width*Height, frame.Rendered())
	for _, count := range frame.RenderedPixels {
		assert.Equal(t, uint16(Width), count)
	}
	assert.Equal(t, MaskBackground, frame.MaskStart)
	assert.Len(t, frame.OffSpans(MaskBoth), 0)
	assert.Len(t, frame.OffSpans(MaskBackground), 0)
	// Sprites stay off for the whole picture.
	assert.Equal(t, []DotSpan{{Start: 0, End: VisibleDots}}, frame.OffSpans(MaskSprites))

	// The write in the second period is in that capture, before the picture.
	changed := (*captures)[1]
	assert.Len(t, changed.MaskChanges, 1)
	assert.Equal(t, MaskBackground, changed.MaskChanges[0].Value)
	assert.True(t, changed.MaskChanges[0].Scanline >= 241)
}

func TestObserveFramesMaskSpansInsidePicture(t *testing.T) {
	p := newOpenBusTestPPU()
	captures := collectFrames(p)
	p.Write(register.PPU_MASK, MaskBoth)
	stepToCapture(p, captures)
	stepToCapture(p, captures)

	// Rendering goes off in scanline 10 and on in scanline 20 of the next
	// picture. Both writes happen between steps, like a CPU write.
	stepToScanline(p, 10, 100)
	p.Write(register.PPU_MASK, 0)
	stepToScanline(p, 20, 50)
	p.Write(register.PPU_MASK, MaskSprites)
	stepToScanline(p, 30, 7)
	p.Write(register.PPU_MASK, MaskBoth)
	stepToCapture(p, captures)

	frame := (*captures)[2]
	assert.Equal(t, MaskBoth, frame.MaskStart)
	assert.Equal(t, []MaskChange{
		{Scanline: 10, Dot: 100, Value: 0},
		{Scanline: 20, Dot: 50, Value: MaskSprites},
		{Scanline: 30, Dot: 7, Value: MaskBoth},
	}, frame.MaskChanges)

	off1Start := 10*DotsPerScanline + 100 + 1
	off1End := 20*DotsPerScanline + 50 + 1
	bgEnd := 30*DotsPerScanline + 7 + 1
	assert.Equal(t, []DotSpan{{Start: off1Start, End: off1End}}, frame.OffSpans(MaskBoth))
	assert.Equal(t, []DotSpan{{Start: off1Start, End: bgEnd}}, frame.OffSpans(MaskBackground))
	assert.Equal(t, []DotSpan{{Start: off1Start, End: off1End}}, frame.OffSpans(MaskSprites))

	// The scanlines with rendering off for the whole line drew no pixel.
	for scanline := 11; scanline < 20; scanline++ {
		assert.Equal(t, uint16(0), frame.RenderedPixels[scanline])
	}
	assert.Equal(t, uint16(Width), frame.RenderedPixels[5])
	assert.Equal(t, uint16(Width), frame.RenderedPixels[35])
	// Dots 1-100 of scanline 10 drew pixels before the write.
	assert.Equal(t, uint16(100), frame.RenderedPixels[10])
}

func TestObserveFramesMaskChangeIgnoresOtherBits(t *testing.T) {
	p := newOpenBusTestPPU()
	captures := collectFrames(p)
	stepToCapture(p, captures)
	p.Write(register.PPU_MASK, MaskBoth)
	p.Write(register.PPU_MASK, MaskBoth|0x01)
	p.Write(register.PPU_MASK, MaskBoth|0xE0)
	stepToCapture(p, captures)
	assert.Len(t, (*captures)[1].MaskChanges, 1)
}

func TestObserveFramesResetRecordsMaskChange(t *testing.T) {
	p := newOpenBusTestPPU()
	captures := collectFrames(p)
	p.Write(register.PPU_MASK, MaskBoth)
	stepToCapture(p, captures)
	p.Reset()
	stepToCapture(p, captures)
	frame := (*captures)[1]
	assert.Equal(t, MaskBoth, frame.MaskStart)
	assert.Len(t, frame.MaskChanges, 1)
	assert.Equal(t, byte(0), frame.MaskChanges[0].Value)
}

func TestObserveFramesGrayscaleBackdrop(t *testing.T) {
	p := newOpenBusTestPPU()
	writePaletteEntry(p, 0, 0x21)
	p.Write(register.PPU_MASK, 0x01)
	captures := collectFrames(p)
	stepToCapture(p, captures)
	stepToCapture(p, captures)
	assert.Equal(t, byte(0x21&0x30), (*captures)[1].Indices[0])
}

func TestObserveFramesCloneAndDisable(t *testing.T) {
	p := newOpenBusTestPPU()
	writePaletteEntry(p, 0, 0x05)
	var kept *FrameCapture
	var cloned FrameCapture
	p.ObserveFrames(func(c *FrameCapture) {
		if kept == nil {
			kept = c
			cloned = c.Clone()
		}
	})
	for kept == nil {
		p.Step(1)
	}
	assert.Equal(t, byte(0), kept.Indices[0])
	writePaletteEntry(p, 0, 0x06)
	p.Step(dotsPerFrame)
	// The PPU reuses the buffer; the clone keeps the old content.
	assert.Equal(t, byte(0x06), kept.Indices[0])
	assert.Equal(t, byte(0), cloned.Indices[0])
	assert.NotNil(t, cloned.RGBA())

	calls := 0
	p.ObserveFrames(func(*FrameCapture) { calls++ })
	p.ObserveFrames(nil)
	p.Step(2 * dotsPerFrame)
	assert.Equal(t, 0, calls)
}

func TestObserveFramesDoNotChangePPU(t *testing.T) {
	run := func(observe bool) ([]byte, uint64, int, int) {
		p := newOpenBusTestPPU()
		writePaletteEntry(p, 0, 0x11)
		writePaletteEntry(p, 1, 0x16)
		if observe {
			p.ObserveFrames(func(*FrameCapture) {})
		}
		p.Write(register.PPU_MASK, MaskBoth)
		for range 4 {
			p.Step(dotsPerFrame/2 + 17)
			p.Write(register.PPU_MASK, p.mask.Value()^MaskBoth)
		}
		img := p.Image()
		return append([]byte(nil), img.Pix...), p.Frame(), p.renderState.ScanLine(), p.renderState.Cycle()
	}
	pixelsA, frameA, lineA, dotA := run(false)
	pixelsB, frameB, lineB, dotB := run(true)
	assert.Equal(t, pixelsA, pixelsB)
	assert.Equal(t, frameA, frameB)
	assert.Equal(t, lineA, lineB)
	assert.Equal(t, dotA, dotB)
}

func TestObserveFramesMaskWriteBeforeVBlankKeepsVisibleSpans(t *testing.T) {
	for _, initial := range []byte{0, MaskBoth} {
		t.Run(fmt.Sprintf("mask_%02x", initial), func(t *testing.T) {
			p := newOpenBusTestPPU()
			p.Write(register.PPU_MASK, initial)
			captures := collectFrames(p)
			stepToCapture(p, captures)
			stepToScanline(p, 241, 0)

			// Dot 0 precedes capture, but follows all visible pixels.
			// This write must not change the spans of the completed picture.
			p.Write(register.PPU_MASK, initial^MaskBoth)
			stepToCapture(p, captures)
			frame := (*captures)[1]
			assert.Len(t, frame.MaskChanges, 1)
			if initial == 0 {
				assert.Equal(t, []DotSpan{{
					Start: 0,
					End:   VisibleDots,
				}}, frame.OffSpans(MaskBoth))
			} else {
				assert.Empty(t, frame.OffSpans(MaskBoth))
			}

			stepToCapture(p, captures)
			assert.Equal(t, initial^MaskBoth, (*captures)[2].MaskStart)
		})
	}
}

func TestObserveFramesBlankPaletteOverride(t *testing.T) {
	tests := []struct {
		name    string
		address uint16
		mask    byte
		want    byte
	}{
		{
			name:    "palette",
			address: 0x3F04,
			want:    0x26,
		},
		{
			name:    "palette_mirror",
			address: 0x3FF4,
			want:    0x26,
		},
		{
			name:    "grayscale",
			address: 0x3F04,
			mask:    1,
			want:    0x20,
		},
		{
			name:    "nametable",
			address: 0x2F04,
			want:    0x11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newOpenBusTestPPU()
			writePaletteEntry(p, 0, 0x11)
			writePaletteEntry(p, 4, 0x26)
			p.Write(register.PPU_ADDR, byte(tt.address>>8))
			p.Write(register.PPU_ADDR, byte(tt.address))
			p.Write(register.PPU_MASK, tt.mask)
			captures := collectFrames(p)
			stepToCapture(p, captures)
			stepToCapture(p, captures)

			// During forced blank, v selects the palette entry when its
			// low 14 bits are in $3F00-$3FFF.
			// https://www.nesdev.org/wiki/PPU_rendering#Rendering_disabled
			frame := (*captures)[1]
			assert.Equal(t, tt.want, frame.Indices[0])
			assert.Equal(t, tt.want, frame.Indices[Width*Height-1])
			assert.Equal(t, RGB(tt.want), frame.RGBA().RGBAAt(0, 0))
			assert.Equal(t, 0, frame.Rendered())
			assert.Equal(t, tt.address, p.addressing.Address())
		})
	}
}

func stepToScanline(p *PPU, scanline, dot int) {
	for p.renderState.ScanLine() != scanline || p.renderState.Cycle() != dot {
		p.Step(1)
	}
}
