package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/cli"
)

func TestReadCartridge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nes")
	_, err := readCartridge(path)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.NoError(t, os.WriteFile(path, []byte("invalid"), 0o644))
	_, err = readCartridge(path)
	assert.ErrorContains(t, err, "reading cartridge")

	want := cartridge.New()
	want.PRG[0] = 0x42
	file, err := os.Create(path)
	assert.NoError(t, err)
	assert.NoError(t, want.Save(file))
	assert.NoError(t, file.Close())
	got, err := readCartridge(path)
	assert.NoError(t, err)
	assert.Equal(t, want.PRG, got.PRG)
	assert.Equal(t, want.CHR, got.CHR)
	assert.Equal(t, want.Mapper, got.Mapper)
}

func TestEmulatorFlags(t *testing.T) {
	var options optionFlags
	var positional positionalArguments
	flags := cli.NewFlagSet("nesgoemu")
	flags.AddSection("Options", &options)
	flags.AddPositional(&positional)
	_, err := flags.Parse([]string{"-features", "-no-summary", "game.nes"})
	assert.NoError(t, err)
	assert.True(t, options.Features)
	assert.True(t, options.NoSummary)
	assert.Equal(t, "game.nes", positional.File)
	assert.Equal(t, -1, options.Entrypoint)
}
