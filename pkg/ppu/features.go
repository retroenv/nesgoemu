package ppu

import "github.com/retroenv/nesgoemu/pkg/feature"

// visibleScanLines is the number of scan lines that show the picture.
const visibleScanLines = 240

// Features returns the PPU feature inventory in declaration order.
func (p *PPU) Features() []feature.Usage {
	return p.features.Features()
}

func (p *PPU) declareFeatures() {
	p.features.Declare(feature.BackgroundLeftHidden)
	p.features.Declare(feature.BackgroundRendering)
	p.features.Declare(feature.BackgroundTableHigh)
	p.features.Declare(feature.ColorEmphasis)
	p.features.Declare(feature.Grayscale)
	p.features.Declare(feature.NMI)
	p.features.Declare(feature.MidFrameScroll)
	p.features.Declare(feature.OAMDataWrites)
	p.features.Declare(feature.OAMDMA)
	p.features.Declare(feature.Sprite0HitRead)
	p.features.Declare(feature.SpriteLeftHidden)
	p.features.Declare(feature.SpriteRendering)
	p.features.Declare(feature.SpriteSize8x16)
	p.features.Declare(feature.SpriteTableHigh)
	p.features.Declare(feature.VRAMIncrement32)
}

// markStatusFeatures marks the sprite 0 hit flag when a program reads it as set
// while the PPU draws a visible line. A program that polls the flag at this
// time uses it for timing or collision checks. A read in vertical blank does
// not count, because the wait loops for vertical blank read the same register
// and see the flags of the last frame.
//
// The sprite overflow flag has no feature. The overflow flag of the hardware
// is often set by mistake, so a program that polls the hit flag can read it as
// set without any use of it.
// https://www.nesdev.org/wiki/PPU_registers#PPUSTATUS
func (p *PPU) markStatusFeatures(value byte) {
	if !p.drawingVisibleLine() {
		return
	}

	if value&(1<<6) != 0 {
		p.features.Mark(feature.Sprite0HitRead)
	}
}

// markScrollWrite marks a write to a scroll or address register while the PPU
// draws a visible line. Games use such writes for split screens.
// https://www.nesdev.org/wiki/PPU_scrolling
func (p *PPU) markScrollWrite() {
	if p.drawingVisibleLine() {
		p.features.Mark(feature.MidFrameScroll)
	}
}

// drawingVisibleLine reports whether rendering is on and the PPU is on a
// visible line.
func (p *PPU) drawingVisibleLine() bool {
	rendering := p.mask.RenderBackground() || p.mask.RenderSprites()
	return rendering && p.renderState.ScanLine() < visibleScanLines
}
