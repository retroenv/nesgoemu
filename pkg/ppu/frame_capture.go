package ppu

import (
	"image"
	"slices"

	"github.com/retroenv/nesgoemu/pkg/ppu/mask"
	"github.com/retroenv/nesgoemu/pkg/ppu/screen"
)

// Frame geometry and timing units of a FrameCapture.
const (
	Width  = screen.Width
	Height = screen.Height

	// DotsPerScanline is the number of PPU dots in one scanline.
	DotsPerScanline = 341
	// VisibleDots is the number of PPU dots of the scanlines 0-239.
	VisibleDots = Height * DotsPerScanline

	// MaskBackground and MaskSprites are the PPUMASK bits that enable the
	// background and the sprites. MaskBoth selects both bits.
	MaskBackground byte = mask.MASK_BG
	MaskSprites    byte = mask.MASK_SPR
	MaskBoth       byte = MaskBackground | MaskSprites
)

// MaskChange records a PPUMASK write that changed the background or sprite
// enable bit. The PPU uses the new value from the dot that follows Dot.
type MaskChange struct {
	Scanline int // 0-261
	Dot      int // 0-340
	Value    byte
}

// DotSpan is a half-open range [Start, End) of PPU dots in the visible area.
// A dot index is Scanline*DotsPerScanline + Dot, with Scanline 0-239.
type DotSpan struct {
	Start int
	End   int
}

// FrameCapture is the picture and the rendering state of one frame period.
// The period runs from the start of the vertical blank of the previous
// picture, scanline 241 dot 1, to the same point of this picture. The visible
// scanlines 0-239 are inside it. The PPU delivers the capture at the end of
// the period, when the picture is complete.
type FrameCapture struct {
	// Frame is the PPU frame counter value of the picture.
	Frame uint64
	// Partial is true when observation started inside the period. Pixels
	// from before that point are unknown and have index 0.
	Partial bool
	// Indices holds the Width*Height system palette indexes of the picture,
	// row by row. A pixel that the PPU drew has the index that the emulator
	// shows. A pixel that the PPU did not draw, because background and
	// sprite rendering were both off, has the backdrop index at that dot.
	// The index includes the grayscale mask.
	Indices []byte
	// Emphasis has PPUMASK bits 7:5 for each pixel, also during forced blank.
	// Indices keeps the six-bit palette values.
	Emphasis []byte
	// RenderedPixels counts, per scanline, the pixels that the PPU drew
	// with background or sprite rendering enabled.
	RenderedPixels [Height]uint16
	// MaskStart is the PPUMASK value at the start of the period.
	MaskStart byte
	// MaskChanges lists, in time order, the writes and resets that changed
	// bit 3 or bit 4 of PPUMASK in the period.
	MaskChanges []MaskChange
}

// Rendered returns the number of pixels that the PPU drew.
func (c *FrameCapture) Rendered() int {
	total := 0
	for _, count := range c.RenderedPixels {
		total += int(count)
	}
	return total
}

// Clone returns a copy that stays valid after the PPU reuses the buffers.
func (c *FrameCapture) Clone() FrameCapture {
	clone := *c
	clone.Indices = slices.Clone(c.Indices)
	clone.Emphasis = slices.Clone(c.Emphasis)
	clone.MaskChanges = slices.Clone(c.MaskChanges)
	return clone
}

// RGBA returns the picture as a new RGBA image. A pixel that the PPU drew has
// its display color, including emphasis. Forced-blank pixels show the backdrop.
func (c *FrameCapture) RGBA() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for index, paletteIndex := range c.Indices {
		var emphasis byte
		if index < len(c.Emphasis) {
			emphasis = c.Emphasis[index]
		}
		color := defaultPalette.rgb(paletteIndex, emphasis)
		offset := index * 4
		img.Pix[offset] = color.R
		img.Pix[offset+1] = color.G
		img.Pix[offset+2] = color.B
		img.Pix[offset+3] = color.A
	}
	return img
}

// OffSpans returns the ranges of the visible area where none of the PPUMASK
// bits in layers was set. Use MaskBoth for ranges with no picture, and
// MaskBackground or MaskSprites for one layer. A write takes effect on the dot
// after it. Adjacent ranges are merged.
func (c *FrameCapture) OffSpans(layers byte) []DotSpan {
	state := c.MaskStart
	var spans []DotSpan
	start := 0
	add := func(end int) {
		if state&layers != 0 || end <= start {
			return
		}
		if last := len(spans) - 1; last >= 0 && spans[last].End == start {
			spans[last].End = end
			return
		}
		spans = append(spans, DotSpan{
			Start: start,
			End:   end,
		})
	}

	for _, change := range c.MaskChanges {
		switch {
		case change.Scanline > 241 || change.Scanline == 241 && change.Dot >= 1:
			// A write in the vertical blank before the picture sets the
			// state at the first visible dot.
			state = change.Value

		case change.Scanline < Height:
			at := min(change.Scanline*DotsPerScanline+change.Dot+1, VisibleDots)
			add(at)
			state = change.Value
			start = at
		}
		// Scanline 240 and scanline 241 dot 0 follow the visible area.
	}
	add(VisibleDots)
	return spans
}

// ObserveFrames replaces the optional observer for completed pictures.
// The observer runs on the emulation goroutine when the PPU completes the
// picture, at scanline 241 dot 1. The capture and its slices are valid only
// during the callback; use Clone to keep it. The first capture is Partial.
// The observer must not change system state. A nil observer disables
// capture. Call ObserveFrames on the emulation goroutine, or before it starts.
// Capture does not change emulation results.
func (p *PPU) ObserveFrames(observer func(*FrameCapture)) {
	if observer == nil {
		p.probe = nil
		return
	}
	p.probe = newFrameProbe(observer, p.mask.Value())
}

// probeMaskChange records a PPUMASK change for the frame observer.
func (p *PPU) probeMaskChange(newValue byte) {
	p.probe.maskChange(p.renderState.ScanLine(), p.renderState.Cycle(), p.mask.Value(), newValue)
}

// probePixel records a pixel that the PPU drew.
func (p *PPU) probePixel(x, y int, paletteIndex, emphasis byte) {
	if p.probe != nil {
		p.probe.pixel(x, y, paletteIndex, emphasis)
	}
}

// probeBackdrop records a forced-blank pixel.
func (p *PPU) probeBackdrop(x, y int, paletteIndex, emphasis byte) {
	if p.probe != nil {
		p.probe.backdrop(x, y, paletteIndex, emphasis)
	}
}

// probeFinish delivers the capture at the end of the picture.
func (p *PPU) probeFinish() {
	if p.probe != nil {
		p.probe.finish(p.renderState.Frame(), p.mask.Value())
	}
}

// frameProbe collects a FrameCapture while the PPU runs. It is nil when no
// observer is set, so the PPU pays only a nil check.
type frameProbe struct {
	observer func(*FrameCapture)
	capture  FrameCapture
}

func newFrameProbe(observer func(*FrameCapture), maskValue byte) *frameProbe {
	return &frameProbe{
		observer: observer,
		capture: FrameCapture{
			Partial:   true,
			Indices:   make([]byte, Width*Height),
			Emphasis:  make([]byte, Width*Height),
			MaskStart: maskValue,
		},
	}
}

func (f *frameProbe) pixel(x, y int, paletteIndex, emphasis byte) {
	if uint(x) >= Width || uint(y) >= Height {
		return
	}
	f.capture.Indices[y*Width+x] = paletteIndex
	f.capture.Emphasis[y*Width+x] = emphasis
	f.capture.RenderedPixels[y]++
}

func (f *frameProbe) backdrop(x, y int, paletteIndex, emphasis byte) {
	if uint(x) >= Width || uint(y) >= Height {
		return
	}
	f.capture.Indices[y*Width+x] = paletteIndex
	f.capture.Emphasis[y*Width+x] = emphasis
}

func (f *frameProbe) maskChange(scanline, dot int, oldValue, newValue byte) {
	if (oldValue^newValue)&MaskBoth == 0 {
		return
	}
	f.capture.MaskChanges = append(f.capture.MaskChanges, MaskChange{
		Scanline: scanline,
		Dot:      dot,
		Value:    newValue,
	})
}

// finish delivers the capture and starts the next period.
func (f *frameProbe) finish(frame uint64, maskValue byte) {
	f.capture.Frame = frame
	f.observer(&f.capture)
	f.capture.Partial = false
	f.capture.RenderedPixels = [Height]uint16{}
	f.capture.MaskStart = maskValue
	f.capture.MaskChanges = f.capture.MaskChanges[:0]
}
