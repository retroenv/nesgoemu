package ppu

import (
	_ "embed"
	"fmt"
	"image/color"
)

const (
	baseColorCount   = 64
	emphasisCount    = 8
	rgbChannels      = 3
	basePaletteBytes = baseColorCount * rgbChannels
	fullPaletteBytes = basePaletteBytes * emphasisCount
	colorIndexMask   = baseColorCount - 1
	emphasisMask     = emphasisCount - 1
	emphasisShift    = 5 // PPUMASK bits 7:5.
)

// defaultPaletteRGB is an unchanged copy of the 2C02_default.pal example
// palette of Pally, by Persune, under the MIT No Attribution license in
// palettes/LICENSE.md. Pally makes it from a 2C02 NTSC signal model with its
// default settings. The file has 512 RGB entries: 64 colors for each of the
// eight emphasis values. TestDefaultDisplayPaletteMatchesPublicSource checks
// its SHA-256 hash.
// https://github.com/Gumball2415/pally/blob/9635e9b71505f969014c7efa15dca3ba35ade871/docs/example_palettes/2C02_default.pal
//
//go:embed palettes/pally-2c02.pal
var defaultPaletteRGB string

var defaultPalette = mustParseDisplayPalette([]byte(defaultPaletteRGB))

// DisplayPalette maps the six-bit PPU color index and the three emphasis bits
// to RGB. It changes only the display output. It does not change palette RAM.
type DisplayPalette [baseColorCount * emphasisCount]color.RGBA

// Color returns the RGB entry for a color index and emphasis value. It does
// not apply grayscale. The PPU applies grayscale to the index before the call.
func (p *DisplayPalette) Color(index, emphasis byte) color.RGBA {
	return p[int(emphasis&emphasisMask)*baseColorCount+int(index&colorIndexMask)]
}

// SetDisplayPalette sets the RGB conversion. Call it before emulation starts.
func (p *PPU) SetDisplayPalette(value DisplayPalette) {
	p.displayPalette = value
}

// DefaultDisplayPalette returns a copy of the embedded Pally 2C02 NTSC
// palette. It has explicit RGB entries for all eight emphasis values. It is a
// fixed palette. It does not simulate analog video or other PPU variants.
func DefaultDisplayPalette() DisplayPalette {
	return defaultPalette
}

// RGB returns the color of a six-bit NES palette index in the default display
// palette, without emphasis. The PPU renders with its own display palette and
// the PPUMASK emphasis bits.
func RGB(index byte) color.RGBA {
	return defaultPalette.Color(index, 0)
}

// ParseDisplayPalette reads the bytes of an RGB .pal file. A 192-byte file
// has 64 colors; the function calculates its emphasis entries. A 1536-byte
// file has all eight emphasis sets. Alpha is always opaque. The function
// returns an error for all other sizes.
func ParseDisplayPalette(data []byte) (DisplayPalette, error) {
	if len(data) != basePaletteBytes && len(data) != fullPaletteBytes {
		return DisplayPalette{}, fmt.Errorf("palette has %d bytes; expected %d or %d",
			len(data), basePaletteBytes, fullPaletteBytes)
	}

	var result DisplayPalette
	for i := range len(data) / rgbChannels {
		offset := i * rgbChannels
		result[i] = color.RGBA{
			R: data[offset],
			G: data[offset+1],
			B: data[offset+2],
			A: 255,
		}
	}

	if len(data) == basePaletteBytes {
		expandEmphasis(&result)
	}
	return result, nil
}

func mustParseDisplayPalette(data []byte) DisplayPalette {
	palette, err := ParseDisplayPalette(data)
	if err != nil {
		panic(fmt.Errorf("embedded display palette: %w", err))
	}
	return palette
}

// expandEmphasis fills the emphasis sets 1-7 from the 64 colors of set 0.
// Each emphasis bit multiplies the two other channels by 3/4. This is an
// approximation. Only 192-byte palette files use it.
func expandEmphasis(palette *DisplayPalette) {
	for emphasis := 1; emphasis < emphasisCount; emphasis++ {
		for index, c := range palette[:baseColorCount] {
			if emphasis&1 != 0 {
				c.G, c.B = attenuate(c.G), attenuate(c.B)
			}
			if emphasis&2 != 0 {
				c.R, c.B = attenuate(c.R), attenuate(c.B)
			}
			if emphasis&4 != 0 {
				c.R, c.G = attenuate(c.R), attenuate(c.G)
			}
			palette[emphasis*baseColorCount+index] = c
		}
	}
}

func attenuate(value byte) byte {
	return byte(int(value) * 3 / 4)
}
