package nes

import (
	"bytes"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

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
	sys.Bus.Mapper.Write(0x6000, 0x5A)         // Touch PRG RAM.

	summary := sys.Summary()
	assert.Len(t, summary.Groups, 2)
	assert.Equal(t, "PPU", summary.Groups[0].Name)
	assert.Equal(t, "Mapper", summary.Groups[1].Name)

	text := summary.String()

	assert.Contains(t, text, "Run summary")
	assert.Contains(t, text, "0 NROM")
	assert.Contains(t, text, "PRG ROM:")
	assert.Contains(t, text, "vertical")
	assert.Contains(t, text, "Mapper features")
	assert.Contains(t, text, "PPU features")
	assert.Contains(t, text, "[x] PRG RAM")
	assert.Contains(t, text, "[x] 8x16 sprites")
	assert.Contains(t, text, "[ ] NMI")
}

func TestSummaryTargetIsDisabledByDefault(t *testing.T) {
	opts := NewOptions()

	assert.Nil(t, opts.summaryTarget)
}

func TestStartWritesSummaryWhenEnabled(t *testing.T) {
	rom := make([]byte, 16+0x8000+0x2000)
	copy(rom, []byte{'N', 'E', 'S', 0x1A, 2, 1, 1})
	cart, err := cartridge.LoadFile(bytes.NewReader(rom))
	assert.NoError(t, err)
	cart.PRG[0x7FFC] = 0x00 // Reset vector 0x8000.
	cart.PRG[0x7FFD] = 0x80

	var out bytes.Buffer
	err = Start(
		WithCartridge(cart),
		WithDisabledGUI(),
		WithSavePath(""),
		WithStopAt(0x8000),
		WithSummaryTarget(&out),
	)
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "Run summary")
}
