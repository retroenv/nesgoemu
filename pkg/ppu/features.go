package ppu

import "github.com/retroenv/nesgoemu/pkg/feature"

// Features returns the PPU feature inventory in declaration order.
func (p *PPU) Features() []feature.Usage {
	return p.features.Features()
}

func (p *PPU) declareFeatures() {
	p.features.Declare(feature.BackgroundRendering)
	p.features.Declare(feature.BackgroundTableHigh)
	p.features.Declare(feature.ColorEmphasis)
	p.features.Declare(feature.Grayscale)
	p.features.Declare(feature.NMI)
	p.features.Declare(feature.OAMDMA)
	p.features.Declare(feature.SpriteRendering)
	p.features.Declare(feature.SpriteSize8x16)
	p.features.Declare(feature.SpriteTableHigh)
	p.features.Declare(feature.VRAMIncrement32)
}
