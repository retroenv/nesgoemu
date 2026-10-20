package mcpserver

import (
	"image"

	"github.com/retroenv/nesgoemu/pkg/ppu"
)

// Tile geometry.
const (
	tileSize      = 8
	nametableCols = 32
	nametableRows = 30
	patternTable  = 0x1000
	patternTiles  = 256
	patternAcross = 16
)

// renderPatternTable renders one pattern table to a 128x128 image.
func renderPatternTable(peek func(uint16) (byte, bool), base uint16, palette [4]byte) *image.RGBA {
	side := patternAcross * tileSize
	target := image.NewRGBA(image.Rect(0, 0, side, side))

	for tile := range patternTiles {
		tileX := tile % patternAcross
		tileY := tile / patternAcross
		renderTile(target, tileX*tileSize, tileY*tileSize, base+uint16(tile)*16, peek, palette)
	}
	return target
}

// renderNametable renders one nametable to a 256x240 image.
func renderNametable(peek func(uint16) (byte, bool), base, attributeBase, patternBase uint16,
	palette []byte) *image.RGBA {

	width := nametableCols * tileSize
	height := nametableRows * tileSize
	target := image.NewRGBA(image.Rect(0, 0, width, height))

	for row := range nametableRows {
		for col := range nametableCols {
			tileIndex, ok := peek(base + uint16(row*nametableCols+col))
			if !ok {
				continue
			}
			tilePalette := nametableTilePalette(peek, attributeBase, row, col)
			colors := backgroundPalette(palette, tilePalette)
			renderTile(target, col*tileSize, row*tileSize, patternBase+uint16(tileIndex)*16, peek, colors)
		}
	}
	return target
}

// renderTile draws one 8x8 tile from the pattern table.
func renderTile(target *image.RGBA, x, y int, address uint16, peek func(uint16) (byte, bool), palette [4]byte) {
	for row := range tileSize {
		plane0, ok0 := peek(address + uint16(row))
		plane1, ok1 := peek(address + tileSize + uint16(row))
		if !ok0 || !ok1 {
			continue
		}
		for col := range tileSize {
			bit0 := (plane0 >> (7 - col)) & 1
			bit1 := (plane1 >> (7 - col)) & 1
			index := bit1<<1 | bit0
			target.Set(x+col, y+row, ppu.RGB(palette[index]))
		}
	}
}

// nametableTilePalette returns the 4 palette indexes of a tile.
func nametableTilePalette(peek func(uint16) (byte, bool), attributeBase uint16, row, col int) [4]byte {
	attribute, ok := peek(attributeBase + uint16(row/4)*8 + uint16(col/4))
	if !ok {
		return [4]byte{}
	}
	shift := uint((row%4)/2*4 + (col%4)/2*2)
	group := (attribute >> shift) & 0b11
	return [4]byte{0, group*4 + 1, group*4 + 2, group*4 + 3}
}

// backgroundPalette maps palette indexes to colors.
func backgroundPalette(palette []byte, indexes [4]byte) [4]byte {
	var colors [4]byte
	colors[0] = paletteByte(palette, 0)
	for index := 1; index < 4; index++ {
		colors[index] = paletteByte(palette, int(indexes[index]))
	}
	return colors
}

// paletteByte returns a palette byte or the backdrop color.
func paletteByte(palette []byte, index int) byte {
	if index < 0 || index >= len(palette) {
		return 0x0F
	}
	return palette[index]
}
