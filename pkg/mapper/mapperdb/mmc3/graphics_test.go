package mmc3

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/assert"
)

func TestGraphicsIgnoresRegisterSelectionAndPRGBanks(t *testing.T) {
	m, _ := newTestMapper(t)
	var events []bus.GraphicsEvent
	m.ObserveGraphics(func(event bus.GraphicsEvent) { events = append(events, event) })
	m.Write(0x8000, 2)
	assert.Empty(t, events)
	m.Write(0x8001, 3)
	assert.Equal(t, []bus.GraphicsEvent{{
		Kind: bus.GraphicsCHRMapping,
		Page: bus.GraphicsPage{
			Memory: bus.GraphicsCHRROM,
			Index:  3,
		},
		Slot: 0x1000,
	}}, events)
	m.Write(0x8001, 3)
	m.Write(0x8000, 6)
	m.Write(0x8001, 2)
	assert.Len(t, events, 1)
}
