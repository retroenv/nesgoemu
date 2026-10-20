package ppu

import (
	_ "embed"
	"fmt"
	"image/color"
)

const (
	baseColorCount = 64
	emphasisCount  = 8
	rgbChannels    = 3
	paletteBytes   = baseColorCount * emphasisCount * rgbChannels
	colorIndexMask = baseColorCount - 1
	emphasisMask   = emphasisCount - 1
	emphasisShift  = 5 // PPUMASK bits 7:5.
)

// defaultPaletteRGB is an unchanged copy of the 2C02_default.pal example
// palette of Pally, by Persune, under the MIT No Attribution license in
// palettes/LICENSE.md. Pally makes it from a 2C02 NTSC signal model with its
// default settings. The file has 512 RGB entries: 64 colors for each of the
// eight emphasis values. TestDefaultPaletteMatchesPublicSource checks its
// SHA-256 hash.
// https://github.com/Gumball2415/pally/blob/9635e9b71505f969014c7efa15dca3ba35ade871/docs/example_palettes/2C02_default.pal
//
//go:embed palettes/pally-2c02.pal
var defaultPaletteRGB string

var defaultPalette = newDisplayPalette(defaultPaletteRGB)

// RGB returns the display color of a six-bit NES palette index, without
// emphasis.
func RGB(index byte) color.RGBA {
	return defaultPalette.rgb(index, 0)
}

// displayPalette maps the six-bit PPU color index and the three emphasis bits
// to RGB. It changes only the display output. It does not change palette RAM.
type displayPalette [baseColorCount * emphasisCount]color.RGBA

// newDisplayPalette converts the bytes of an RGB .pal file with all eight
// emphasis sets. The only input is the embedded file. A different size is a
// programming error, so the function panics when the package initializes.
func newDisplayPalette(data string) displayPalette {
	if len(data) != paletteBytes {
		panic(fmt.Sprintf("display palette has %d bytes; expected %d", len(data), paletteBytes))
	}

	var result displayPalette
	for i := range result {
		offset := i * rgbChannels
		result[i] = color.RGBA{
			R: data[offset],
			G: data[offset+1],
			B: data[offset+2],
			A: 255,
		}
	}
	return result
}

// rgb returns the RGB entry for a color index and emphasis value. It does not
// apply grayscale. The PPU applies grayscale to the index before the call.
func (p *displayPalette) rgb(index, emphasis byte) color.RGBA {
	return p[int(emphasis&emphasisMask)*baseColorCount+int(index&colorIndexMask)]
}
