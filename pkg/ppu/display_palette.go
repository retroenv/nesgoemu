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
	paletteBytes     = basePaletteBytes * emphasisCount
	colorIndexMask   = baseColorCount - 1
	emphasisMask     = emphasisCount - 1
	emphasisShift    = 5 // PPUMASK bits 7:5.
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

// DisplayPalette maps the six-bit PPU color index and the three emphasis bits
// to RGB. It changes only the display output. It does not change palette RAM.
type DisplayPalette [baseColorCount * emphasisCount]color.RGBA

// SetDisplayPalette sets the RGB conversion. Call it before emulation starts.
func (p *PPU) SetDisplayPalette(value DisplayPalette) {
	p.displayPalette = &value
	if p.probe != nil {
		p.probe.capture.Palette = p.displayPalette
	}
}

// DisplayColor returns the display color of a palette index and an emphasis
// value 0-7 in the PPU display palette. It uses the six low bits of the index.
func (p *PPU) DisplayColor(index, emphasis byte) color.RGBA {
	return p.displayColor(index, emphasis)
}

// displayColor returns the display color of a palette index. A PPU with no
// set palette uses the default palette.
func (p *PPU) displayColor(index, emphasis byte) color.RGBA {
	if p.displayPalette == nil {
		return defaultPalette.rgb(index, emphasis)
	}
	return p.displayPalette.rgb(index, emphasis)
}

// rgb returns the RGB entry for a color index and emphasis value. It does not
// apply grayscale. The PPU applies grayscale to the index before the call.
func (p *DisplayPalette) rgb(index, emphasis byte) color.RGBA {
	return p[int(emphasis&emphasisMask)*baseColorCount+int(index&colorIndexMask)]
}

// RGB returns the display color of a six-bit NES palette index, without
// emphasis.
func RGB(index byte) color.RGBA {
	return defaultPalette.rgb(index, 0)
}

// DefaultDisplayPalette returns a copy of the embedded Pally 2C02 NTSC
// palette. It has explicit RGB entries for all eight emphasis values. It is a
// fixed palette. It does not simulate analog video or other PPU variants.
func DefaultDisplayPalette() DisplayPalette {
	return defaultPalette
}

// ParseDisplayPalette reads the bytes of an RGB .pal file. A 192-byte file
// has 64 colors; the function calculates its emphasis entries. A 1536-byte
// file has all eight emphasis sets. Alpha is always opaque. The function
// returns an error for all other sizes.
func ParseDisplayPalette(data []byte) (DisplayPalette, error) {
	if len(data) != basePaletteBytes && len(data) != paletteBytes {
		return DisplayPalette{}, fmt.Errorf("palette has %d bytes; expected %d or %d",
			len(data), basePaletteBytes, paletteBytes)
	}

	result := decodeDisplayPalette(string(data))
	if len(data) == basePaletteBytes {
		expandEmphasis(&result)
	}

	return result, nil
}

// newDisplayPalette converts the bytes of an RGB .pal file with all eight
// emphasis sets. The only input is the embedded file. A different size is a
// programming error, so the function panics when the package initializes.
func newDisplayPalette(data string) DisplayPalette {
	if len(data) != paletteBytes {
		panic(fmt.Sprintf("display palette has %d bytes; expected %d", len(data), paletteBytes))
	}

	return decodeDisplayPalette(data)
}

// decodeDisplayPalette converts RGB triplets. It fills the entries that the
// data covers and leaves the rest black.
func decodeDisplayPalette(data string) DisplayPalette {
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

	return result
}

// expandEmphasis fills the emphasis sets 1-7 from the 64 colors of set 0.
// Each emphasis bit multiplies the two other channels by 3/4. This is an
// approximation. Only 192-byte palette files use it.
func expandEmphasis(palette *DisplayPalette) {
	for emphasis := 1; emphasis < emphasisCount; emphasis++ {
		for index, c := range palette[:baseColorCount] {
			if emphasis&1 != 0 {
				c.G = attenuate(c.G)
				c.B = attenuate(c.B)
			}

			if emphasis&2 != 0 {
				c.R = attenuate(c.R)
				c.B = attenuate(c.B)
			}

			if emphasis&4 != 0 {
				c.R = attenuate(c.R)
				c.G = attenuate(c.G)
			}

			palette[emphasis*baseColorCount+index] = c
		}
	}
}

func attenuate(value byte) byte {
	return byte(int(value) * 3 / 4)
}
