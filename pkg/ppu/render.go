package ppu

import (
	"image"
)

// Image returns the rendered image to display.
func (p *PPU) Image() *image.RGBA {
	return p.screen.Image()
}

// Step executes PPU cycles.
func (p *PPU) Step(cycles int) {
	p.openBus.Tick(cycles)

	for range cycles {
		p.step()
	}
}

func (p *PPU) step() {
	p.nmi.Trigger(p.bus.CPU)
	rendering := p.mask.RenderBackground() || p.mask.RenderSprites()
	p.renderState.TickRendering(rendering)
	cycle := p.renderState.Cycle()
	scanLine := p.renderState.ScanLine()

	if p.ticker != nil {
		p.ticker.TickPPU(cycle, scanLine, rendering)
	}

	if rendering {
		p.renderBackground(cycle, scanLine)
		// sprite evaluation occurs if either the sprite layer or background layer is enabled
		if cycle >= 257 && cycle <= 320 && (scanLine < 240 || scanLine == 261) {
			p.sprites.Render()
		}
	}

	if cycle != 1 {
		return
	}

	switch scanLine {
	case 241:
		// the vertical blank flag of the PPU is set at tick 1 (the second tick) of scanline 241,
		// where the vertical blank NMI also occurs
		p.screen.FinishRendering()
		p.nmi.SetOccurred(true)

	case 261:
		p.nmi.SetOccurred(false)
		p.status.SetSpriteOverflow(false)
		p.status.SetSpriteZeroHit(false)
	}
}

func (p *PPU) renderBackground(cycle, scanLine int) {
	preLine := scanLine == 261
	visibleLine := scanLine < 240
	renderLine := preLine || visibleLine

	// cycle 0 is an idle cycle
	preFetchCycle := cycle >= 321 && cycle <= 336
	visibleCycle := cycle >= 1 && cycle <= 256
	fetchCycle := preFetchCycle || visibleCycle

	if visibleLine && visibleCycle {
		p.renderPixel()
	}

	if renderLine && fetchCycle {
		p.tiles.FetchCycle(cycle)
	}

	if preLine && cycle >= 280 && cycle <= 304 {
		p.addressing.CopyY()
	}

	if renderLine && lineUpdateCycles[cycle] {
		p.renderLine(cycle, fetchCycle)
	}
}

func (p *PPU) renderLine(cycle int, fetchCycle bool) {
	if fetchCycle && cycle%8 == 0 {
		p.addressing.IncrementX()
	}

	if cycle == 256 {
		p.addressing.IncrementY()
	}

	// Only the sprite fetch phases and the final two reads use this address.
	if !lineReadCycles[cycle] {
		return
	}

	// Bits 0-11 select the nametable and tile. Fine Y must not enter this address.
	// https://www.nesdev.org/wiki/PPU_scrolling#Tile_and_attribute_fetching
	address := p.addressing.Address()
	if cycle == 257 {
		// The multiplexed bus keeps the old low byte while horizontal reload
		// changes the high address bits for the first sprite nametable fetch.
		// https://www.nesdev.org/wiki/PPU_rendering#Cycles_257-320
		// https://www.nesdev.org/wiki/PPU_programmer_reference
		low := address & 0xFF
		p.addressing.CopyX()
		reloadedAddress := p.addressing.Address()
		address = reloadedAddress&0xFF00 | low
	}

	// Each sprite slot has two unused nametable reads before its pattern reads.
	// The PPU reuses its background fetch sequence for sprite fetches, so the
	// nametable phases still occur even though the sprite does not use their data.
	// The read starts are 257/259, 265/267, ... , 313/315. The modulo test
	// selects phases 1 and 3 in each of the eight sprite slots on dots 257-320.
	// Dots 337 and 339 start two more nametable reads at the end of the line.
	// A read occupies two dots; this emulator performs it on the first dot.
	// The values are unused, but the cartridge must observe the transactions.
	// The final two reads use the next line's first tile-fetch address.
	// A mapper can count these reads even though they do not supply a pixel.
	// renderBackground calls this only on visible and pre-render lines, and step
	// calls renderBackground only when background or sprite rendering is enabled.
	// https://www.nesdev.org/wiki/PPU_rendering#Cycles_257-320
	// https://www.nesdev.org/wiki/PPU_rendering#Cycles_337-340
	nameTableAddress := 0x2000 | (address & 0x0FFF)
	p.memory.Read(nameTableAddress)
}

func (p *PPU) renderPixel() {
	var backgroundColor, spriteColor byte
	if p.mask.RenderBackground() {
		backgroundColor = p.tiles.BackgroundPixel(p.fineX)
	}

	var spritePriority, spriteZeroHit bool
	if p.mask.RenderSprites() {
		spritePriority, spriteZeroHit, spriteColor = p.sprites.Pixel()
	}

	x := p.renderState.Cycle() - 1
	if x < 8 {
		if !p.mask.RenderBackgroundLeft() {
			backgroundColor = 0
		}
		if !p.mask.RenderSpritesLeft() {
			spriteColor = 0
		}
	}

	hasBackground := backgroundColor%4 != 0
	hasSprite := spriteColor%4 != 0
	var paletteIndex byte

	switch {
	case !hasBackground && hasSprite:
		paletteIndex = spriteColor | 0x10

	case hasBackground && !hasSprite:
		paletteIndex = backgroundColor

	case hasBackground && hasSprite:
		if spriteZeroHit && x < 255 {
			p.status.SetSpriteZeroHit(true)
		}

		if spritePriority {
			paletteIndex = spriteColor | 0x10
		} else {
			paletteIndex = backgroundColor
		}
	}

	colorIndex := p.palette.Read(uint16(paletteIndex))
	colorIndex = p.maskPaletteColor(colorIndex)
	color := colors[colorIndex]
	y := p.renderState.ScanLine()
	p.screen.SetPixel(x, y, color)
}
