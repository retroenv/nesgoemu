package ppu

import "github.com/retroenv/nesgoemu/pkg/bus"

// CHRRead reports a physical pattern-table read before the mapper reads it.
// Rendering is true for background and sprite fetches, including prefetches
// and empty sprite slots. It is false for program reads through $2007.
type CHRRead struct {
	Mapping   bus.GraphicsMapping
	Rendering bool
}

// ObserveCHRReads sets a callback for mapped pattern-table reads.
// A nil callback disables observation. Set it while emulation is stopped.
func (p *PPU) ObserveCHRReads(observer func(CHRRead)) {
	p.chrReadObserver = observer
	var renderingObserver bus.PPUPatternReadObserver
	if observer != nil {
		renderingObserver = p.observeRenderingCHR
	}

	p.tiles.ObservePatternReads(renderingObserver)
	p.sprites.ObservePatternReads(renderingObserver)
}

func (p *PPU) observeRenderingCHR(address uint16) {
	p.observeCHRRead(address, true)
}

func (p *PPU) observeCHRRead(address uint16, rendering bool) {
	if p.chrReadObserver == nil {
		return
	}
	inspector, ok := p.bus.Mapper.(bus.PPUMappingInspector)
	if !ok {
		return
	}
	mapping, ok := inspector.PPUReadMapping(address)
	if !ok {
		return
	}
	p.chrReadObserver(CHRRead{
		Mapping:   mapping,
		Rendering: rendering,
	})
}
