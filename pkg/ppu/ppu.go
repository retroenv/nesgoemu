// Package ppu provides PPU (Picture Processing Unit) functionality.
package ppu

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/addressing"
	"github.com/retroenv/nesgoemu/pkg/ppu/control"
	"github.com/retroenv/nesgoemu/pkg/ppu/mask"
	"github.com/retroenv/nesgoemu/pkg/ppu/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu/nmi"
	"github.com/retroenv/nesgoemu/pkg/ppu/palette"
	"github.com/retroenv/nesgoemu/pkg/ppu/renderstate"
	"github.com/retroenv/nesgoemu/pkg/ppu/screen"
	"github.com/retroenv/nesgoemu/pkg/ppu/sprites"
	"github.com/retroenv/nesgoemu/pkg/ppu/status"
	"github.com/retroenv/nesgoemu/pkg/ppu/tiles"
)

const FPS = 60

// PPU represents the Picture Processing Unit.
type PPU struct {
	bus *bus.Bus

	fineX          uint16
	dataReadBuffer byte

	addressing  *addressing.Addressing
	control     *control.Control
	mask        *mask.Mask
	memory      *memory.Memory
	nmi         *nmi.Nmi
	palette     *palette.Palette
	renderState *renderstate.RenderState
	screen      *screen.Screen
	sprites     *sprites.Sprites
	status      *status.Status
	ticker      bus.PPUTicker // optional mapper hook for each PPU cycle
	tiles       *tiles.Tiles
}

// New returns a new PPU.
func New(systemBus *bus.Bus) *PPU {
	p := &PPU{
		bus: systemBus,
	}
	p.reset()
	if timing, ok := systemBus.Mapper.(bus.TimingEnabler); ok {
		timing.EnableBusTiming()
	}
	return p
}

// Palette returns the palette.
func (p *PPU) Palette() bus.Palette {
	return p.palette
}

// Reset clears PPU control state. It keeps the current VRAM address and VBlank state.
// https://www.nesdev.org/wiki/PPU_power_up_state
func (p *PPU) Reset() {
	p.control.Set(0)
	p.mask.Set(0)

	p.fineX = 0
	p.addressing.Reset()

	p.dataReadBuffer = 0
}

func (p *PPU) reset() {
	p.fineX = 0
	p.dataReadBuffer = 0

	p.addressing = addressing.New()
	p.mask = mask.New()
	p.nmi = nmi.New()
	p.palette = palette.New()
	p.renderState = renderstate.New()
	p.screen = screen.New()
	p.status = status.New()

	p.memory = memory.New(p.bus.Mapper, p.bus.NameTable, p.palette)
	p.sprites = sprites.New(p.bus.CPU, p.bus.Mapper, p.bus.Memory, p.renderState, p.status)
	p.ticker, _ = p.bus.Mapper.(bus.PPUTicker)

	p.tiles = tiles.New(p.addressing, p.memory, p.bus.NameTable)

	p.control = control.New(p.addressing, p.nmi, p.sprites, p.tiles)
}

func (p *PPU) readData() byte {
	address := p.addressing.Address()
	address &= 0x3FFF // valid addresses are $0000-$3FFF; higher addresses will be mirrored down

	// when reading data, the contents of an internal read buffer is returned and the buffer
	// gets updated with the newly read data
	data := p.dataReadBuffer

	p.dataReadBuffer = p.memory.Read(address)

	if address >= 0x3F00 {
		// Palette data reads are unbuffered, $3F00-$3FFF are Palette RAM indexes and mirrors of it
		data = p.dataReadBuffer
		data = p.maskPaletteColor(data)
	}

	// TODO handle special case of reading during rendering
	p.addressing.Increment(p.control.VRAMIncrement)
	return data
}

// maskPaletteColor selects the gray column without changing palette RAM.
// Apply it to pixels and CPU palette reads when PPUMASK changes.
// https://www.nesdev.org/wiki/PPU_registers#Color_control
func (p *PPU) maskPaletteColor(value byte) byte {
	if p.mask.Grayscale {
		return value & 0x30
	}
	return value & 0x3F
}

func (p *PPU) getStatus() byte {
	p.addressing.ClearLatch()

	p.status.SetVerticalBlank(p.nmi.Occurred())
	p.nmi.SetOccurred(false)

	value := p.status.Value()
	return value
}
