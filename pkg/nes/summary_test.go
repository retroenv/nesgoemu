package nes

import (
	"bytes"
	"errors"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/audio"
	"github.com/retroenv/retrogolib/gui"
	"github.com/retroenv/retrogolib/renderer"
)

// newSummaryCartridge returns a mapper 0 cartridge with the reset vector 0x8000.
func newSummaryCartridge(t *testing.T) *cartridge.Cartridge {
	t.Helper()

	rom := make([]byte, 16+0x8000+0x2000)
	copy(rom, []byte{'N', 'E', 'S', 0x1A, 2, 1, 1})
	cart, err := cartridge.LoadFile(bytes.NewReader(rom))
	assert.NoError(t, err)
	cart.PRG[0x7FFC] = 0x00
	cart.PRG[0x7FFD] = 0x80

	return cart
}

func newSummarySystem(t *testing.T) *System {
	t.Helper()

	rom := make([]byte, 16+0x8000+0x2000)
	copy(rom, []byte{'N', 'E', 'S', 0x1A, 2, 1, 1})
	cart, err := cartridge.LoadFile(bytes.NewReader(rom))
	assert.NoError(t, err)

	sys, err := NewSystem(NewOptions(WithCartridge(cart), WithSavePath("")))
	assert.NoError(t, err)

	return sys
}

func TestSummaryReportsRomAndFeatures(t *testing.T) {
	sys := newSummarySystem(t)

	sys.Bus.PPU.Write(register.PPU_CTRL, 0x20) // Enable 8x16 sprites.
	sys.Bus.APU.Write(register.APU_SND_CHN, 0x01)
	sys.Bus.Mapper.Write(0x6000, 0x5A) // Touch PRG RAM.

	summary := sys.Summary()
	assert.Len(t, summary.Groups, 3)
	assert.Equal(t, "PPU", summary.Groups[0].Name)
	assert.Equal(t, "APU", summary.Groups[1].Name)
	assert.Equal(t, "Mapper", summary.Groups[2].Name)

	text := summary.String()

	assert.Contains(t, text, "Run summary")
	assert.Contains(t, text, "0 NROM")
	assert.Contains(t, text, "PRG ROM:")
	assert.Contains(t, text, "vertical")
	assert.Contains(t, text, "Mapper features")
	assert.Contains(t, text, "PPU features")
	assert.Contains(t, text, "APU features")
	assert.Contains(t, text, "[x] Pulse 1")
	assert.Contains(t, text, "[ ] Pulse 2")
	assert.Contains(t, text, "[x] PRG RAM")
	assert.Contains(t, text, "[x] 8x16 sprites")
	assert.Contains(t, text, "[ ] NMI")
}

func TestSummaryTargetIsDisabledByDefault(t *testing.T) {
	opts := NewOptions()

	assert.Nil(t, opts.summaryTarget)
}

func TestStartWritesSummaryWhenEnabled(t *testing.T) {
	var out bytes.Buffer
	err := Start(
		WithCartridge(newSummaryCartridge(t)),
		WithDisabledGUI(),
		WithSavePath(""),
		WithStopAt(0x8000),
		WithSummaryTarget(&out),
	)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Run summary")
	assert.Contains(t, out.String(), "APU features")
}

func TestSummaryShowsRenderers(t *testing.T) {
	sys := newSummarySystem(t)
	summary := sys.Summary()
	assert.Nil(t, summary.GUI)
	assert.Nil(t, summary.Audio)
	assert.NotContains(t, summary.String(), "GUI:")
	assert.NotContains(t, summary.String(), "Audio:")

	sys.guiInfo = &renderer.Info{
		Name:    "sdl3",
		Version: "3.4.2",
	}
	sys.audioInfo = &renderer.Info{
		Name:    "sdl2",
		Version: "2.32.10",
		Skipped: errors.New("sdl3: loading SDL3: not found"),
	}

	text := sys.Summary().String()
	assert.Contains(t, text, "GUI:        sdl3 3.4.2\n")
	assert.Contains(t, text, "Audio:      sdl2 2.32.10 (fallback, sdl3: loading SDL3: not found)\n")
}

func TestStartRecordsRenderers(t *testing.T) {
	defer gui.Register("fakegui", 1, renderer.Renderer[gui.Initializer]{
		Setup: func(gui.Backend) (func() (bool, error), func(), error) {
			return func() (bool, error) { return false, nil }, func() {}, nil
		},
		Version: func() string { return "1.2.3" },
	})()
	defer audio.Register("fakeaudio", 1, renderer.Renderer[audio.Initializer]{
		Setup:   testRenderer(nil),
		Version: func() string { return "4.5.6" },
	})()

	var out bytes.Buffer
	err := Start(
		WithCartridge(newSummaryCartridge(t)),
		WithSavePath(""),
		WithSummaryTarget(&out),
	)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "GUI:        fakegui 1.2.3\n")
	assert.Contains(t, out.String(), "Audio:      fakeaudio 4.5.6\n")
}
