package ppu

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image/color"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

// The test hashes the 512 decoded RGB entries in emphasis-major order. The
// hash must match the published file, including the order of its emphasis sets.
func TestDefaultPaletteMatchesPublicSource(t *testing.T) {
	const sourceSHA256 = "46f38b15abea5c819504eb08975f43622214fb9c62852848d093de25784ac634"

	rgb := make([]byte, 0, paletteBytes)
	for emphasis := range byte(emphasisCount) {
		for index := range byte(baseColorCount) {
			entry := defaultPalette.rgb(index, emphasis)
			assert.Equal(t, byte(255), entry.A)
			rgb = append(rgb, entry.R, entry.G, entry.B)
			if emphasis == 0 {
				assert.Equal(t, entry, RGB(index))
			}
		}
	}

	assert.Equal(t, sourceSHA256, fmt.Sprintf("%x", sha256.Sum256(rgb)))

	// Entry $00 of the published file for each of the eight emphasis values.
	want := [emphasisCount][rgbChannels]byte{
		{98, 98, 98}, {99, 70, 62}, {60, 92, 50}, {66, 68, 43},
		{75, 72, 121}, {73, 58, 77}, {55, 68, 73}, {58, 58, 58},
	}
	for emphasis, entry := range want {
		actual := defaultPalette.rgb(0, byte(emphasis))
		assert.Equal(t, entry, [rgbChannels]byte{actual.R, actual.G, actual.B})
	}
}

func TestNewDisplayPalette(t *testing.T) {
	data := make([]byte, paletteBytes)
	for i := range data {
		data[i] = byte(i)
	}

	palette := newDisplayPalette(string(data))

	for emphasis := range byte(emphasisCount) {
		for index := range byte(baseColorCount) {
			offset := (int(emphasis)*baseColorCount + int(index)) * rgbChannels
			entry := palette.rgb(index, emphasis)
			assert.Equal(t, data[offset:offset+rgbChannels], []byte{entry.R, entry.G, entry.B})
		}
	}

	for _, size := range []int{0, 192, paletteBytes - 1, paletteBytes + 1} {
		assert.Panics(t, func() { newDisplayPalette(string(make([]byte, size))) })
	}
}

// An emphasis change in the frame applies from the next pixel. The test checks
// rendered pixels and forced blank, with grayscale applied first.
func TestRasterEmphasisCaptureAndScreen(t *testing.T) {
	for _, layers := range []byte{0, MaskBoth} {
		p := newOpenBusTestPPU()
		writePaletteEntry(p, 0, 0x21)
		p.Write(register.PPU_MASK, layers)
		captures := collectFrames(p)
		stepToCapture(p, captures)
		stepToCapture(p, captures)
		stepToScanline(p, 100, 128)
		p.Write(register.PPU_MASK, layers|0xA1)
		stepToCapture(p, captures)
		frame := (*captures)[2]
		assert.Equal(t, byte(0), frame.Emphasis[100*Width+127])
		assert.Equal(t, byte(5), frame.Emphasis[100*Width+128])
		assert.Equal(t, byte(0x20), frame.Indices[100*Width+128])
		assert.Equal(t, p.Image().Pix, frame.RGBA().Pix)

		// A capture keeps its palette when the PPU palette changes.
		kept := frame.Clone()
		before := kept.RGBA().RGBAAt(0, 200)
		p.SetDisplayPalette(DisplayPalette{})
		assert.Equal(t, before, kept.RGBA().RGBAAt(0, 200))
	}
}

func TestEmphasisDoesNotChangePaletteRead(t *testing.T) {
	p := newOpenBusTestPPU()
	writePaletteEntry(p, 0, 0x21)
	p.Write(register.PPU_MASK, 0xE0)
	p.Write(register.PPU_ADDR, 0x3F)
	p.Write(register.PPU_ADDR, 0)
	assert.Equal(t, byte(0x21), p.Read(register.PPU_DATA)&0x3F)
}

func TestDefaultDisplayPaletteIsACopy(t *testing.T) {
	palette := DefaultDisplayPalette()
	original := palette[0]
	palette[0] = color.RGBA{}
	assert.Equal(t, original, DefaultDisplayPalette()[0])
}

func TestParseDisplayPalette(t *testing.T) {
	for _, size := range []int{0, 191, 193, paletteBytes - 1, paletteBytes + 1} {
		_, err := ParseDisplayPalette(make([]byte, size))
		assert.Error(t, err)
	}

	base, err := ParseDisplayPalette(bytes.Repeat([]byte{200, 160, 120}, baseColorCount))
	assert.NoError(t, err)
	assert.Equal(t, color.RGBA{
		R: 200,
		G: 160,
		B: 120,
		A: 255,
	}, base.rgb(0, 0))
	assert.Equal(t, color.RGBA{
		R: 200,
		G: 120,
		B: 90,
		A: 255,
	}, base.rgb(0, 1))
	assert.Equal(t, color.RGBA{
		R: 112,
		G: 90,
		B: 67,
		A: 255,
	}, base.rgb(0, 7))

	data := make([]byte, paletteBytes)
	for i := range data {
		data[i] = byte(i)
	}
	full, err := ParseDisplayPalette(data)
	assert.NoError(t, err)
	for emphasis := range byte(emphasisCount) {
		for index := range byte(baseColorCount) {
			offset := (int(emphasis)*baseColorCount + int(index)) * rgbChannels
			entry := full.rgb(index, emphasis)
			assert.Equal(t, data[offset:offset+rgbChannels], []byte{entry.R, entry.G, entry.B})
		}
	}
}

func TestSetDisplayPaletteChangesScreen(t *testing.T) {
	p := newOpenBusTestPPU()
	writePaletteEntry(p, 0, 0x21)
	p.Write(register.PPU_MASK, 0)
	captures := collectFrames(p)

	var custom DisplayPalette
	for i := range custom {
		custom[i] = color.RGBA{
			R: 1,
			G: 2,
			B: 3,
			A: 255,
		}
	}
	p.SetDisplayPalette(custom)
	stepToCapture(p, captures)
	stepToCapture(p, captures)

	want := color.RGBA{
		R: 1,
		G: 2,
		B: 3,
		A: 255,
	}
	assert.Equal(t, want, p.Image().RGBAAt(0, 100))
	assert.Equal(t, want, (*captures)[1].RGBA().RGBAAt(0, 100))
}
