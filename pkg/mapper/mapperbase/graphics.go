package mapperbase

import "github.com/retroenv/nesgoemu/pkg/bus"

// ObserveGraphics replaces the optional graphics observer.
// The caller must stop emulation before it changes the observer.
func (b *Base) ObserveGraphics(observer func(bus.GraphicsEvent)) {
	b.graphicsObserver = observer
}

// NotifyGraphics sends an event without a mapper lock held.
func (b *Base) NotifyGraphics(event bus.GraphicsEvent) {
	if b.graphicsObserver != nil {
		b.graphicsObserver(event)
	}
}

func (b *Base) notifyCHRMapping(window, bank int) {
	source := bus.GraphicsCHRROM
	if len(b.chrRAM) > 0 {
		source = bus.GraphicsCHRRAM
	}
	b.NotifyGraphics(bus.GraphicsEvent{
		Kind: bus.GraphicsCHRMapping,
		Page: bus.GraphicsPage{
			Memory: source,
			Index:  uint32(bank * b.chrWindowSize / 1024),
		},
		Slot: uint16(window * b.chrWindowSize),
	})
}
