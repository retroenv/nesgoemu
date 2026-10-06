package mapperbase

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestGraphicsReportsEffectiveCHRMapping(t *testing.T) {
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: make([]byte, 0x8000),
			CHR: make([]byte, 0x8000),
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	base.Initialize()
	var events []bus.GraphicsEvent
	base.ObserveGraphics(func(event bus.GraphicsEvent) {
		// Observers can inspect mapper state without a lock conflict.
		_ = base.State()
		events = append(events, event)
	})
	base.SetChrWindow(0, 1)
	base.SetChrWindow(0, 5)
	assert.Equal(t, []bus.GraphicsEvent{{
		Kind: bus.GraphicsCHRMapping,
		Page: bus.GraphicsPage{
			Memory: bus.GraphicsCHRROM,
			Index:  8,
		},
	}}, events)
	base.SetPrgWindow(0, 0)
	assert.Len(t, events, 1)
	base.ObserveGraphics(nil)
	base.SetChrWindow(0, 2)
	assert.Len(t, events, 1)
}

func TestGraphicsIdentifiesCHRMemorySource(t *testing.T) {
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: make([]byte, 0x8000),
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	base.SetChrRAM(make([]byte, 0x4000))
	base.Initialize()
	var event bus.GraphicsEvent
	base.ObserveGraphics(func(value bus.GraphicsEvent) { event = value })
	base.SetChrWindow(0, 1)
	assert.Equal(t, bus.GraphicsPage{
		Memory: bus.GraphicsCHRRAM,
		Index:  8,
	}, event.Page)
}
