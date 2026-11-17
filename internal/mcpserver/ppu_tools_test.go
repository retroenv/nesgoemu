package mcpserver

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp/mcptest"
)

func TestNametableRenderMatchesScreenshotOnCHRROM(t *testing.T) {
	client, id := pictureMachine(t, "")

	shot, err := client.CallTool(t.Context(), screenshotToolName, map[string]any{machineIDKey: id})
	assert.NoError(t, err)
	screen, err := png.Decode(bytes.NewReader(imageContent(t, shot)))
	assert.NoError(t, err)

	result, err := client.CallTool(t.Context(), nametablesToolName, map[string]any{
		machineIDKey: id,
		"render":     true,
		"table":      0,
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	table, err := png.Decode(bytes.NewReader(imageContent(t, result)))
	assert.NoError(t, err)

	// The tile at the top left is solid color 3, the backdrop is black.
	for _, point := range [][2]int{{1, 1}, {6, 6}, {20, 1}, {100, 100}} {
		assert.Equal(t, screen.At(point[0], point[1]), table.At(point[0], point[1]),
			"pixel %v", point)
	}
	assert.NotEqual(t, table.At(1, 1), table.At(100, 100))
}

func TestReadMemoryPPUShowsMappedCHRROM(t *testing.T) {
	client, id := pictureMachine(t, "")

	pattern := callTool[readMemoryResult](t, client, readMemoryToolName, map[string]any{
		machineIDKey: id,
		spaceKey:     spacePPU,
		addressKey:   0x10,
		lengthKey:    4,
	})
	assert.Equal(t, 4, pattern.Available)
	assert.Equal(t, []int{0xFF, 0xFF, 0xFF, 0xFF}, pattern.Bytes)
}

func TestPaletteToolShowsMirrorsAndDisplayColors(t *testing.T) {
	client, id := pictureMachine(t, "")

	palette := inspectSection[paletteResult](t, client, sectionPalette, id)
	assert.Len(t, palette.Entries, 32)
	// $3F10 mirrors $3F00, so the write of $2A replaced the backdrop.
	assert.Equal(t, 0x2A, palette.Entries[0].Color)
	assert.Equal(t, 0x2A, palette.Entries[16].Color)
	assert.Equal(t, palette.Entries[0].R, palette.Entries[16].R)
	for _, entry := range palette.Entries {
		assert.True(t, entry.Color <= 0x3F, "entry %d has %#x", entry.Index, entry.Color)
	}
}

func TestPaletteToolUsesTheMachineDisplayPalette(t *testing.T) {
	path := filepath.Join(t.TempDir(), "white.pal")
	assert.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{255, 255, 255}, 64), 0o600))
	dir := filepath.Dir(path)
	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	t.Cleanup(client.Close)

	rom := filepath.Join(dir, "picture.nes")
	writePictureROM(t, rom)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
		paletteKey:    path,
	})
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: created.MachineID,
		framesKey:    3,
	})

	palette := inspectSection[paletteResult](t, client, sectionPalette, created.MachineID)
	for _, entry := range palette.Entries[:4] {
		assert.Equal(t, 255, entry.R)
		assert.Equal(t, 255, entry.G)
		assert.Equal(t, 255, entry.B)
	}

	pattern, err := client.CallTool(t.Context(), patternTablesToolName, map[string]any{machineIDKey: created.MachineID})
	assert.NoError(t, err)
	tables, err := png.Decode(bytes.NewReader(imageContent(t, pattern)))
	assert.NoError(t, err)
	// Tile 1 is solid color 3, which the white palette shows as white.
	red, green, blue, _ := tables.At(8+1, 1).RGBA()
	assert.Equal(t, uint32(0xFFFF), red)
	assert.Equal(t, uint32(0xFFFF), green)
	assert.Equal(t, uint32(0xFFFF), blue)
}

// TestEveryToolWithArgumentsAdvertisesASchema checks that a client can
// discover the arguments of each tool.
func TestEveryToolWithArgumentsAdvertisesASchema(t *testing.T) {
	_, client, _ := counterServer(t)

	tools, err := client.ListTools(t.Context())
	assert.NoError(t, err)
	assert.NotEmpty(t, tools)
	for _, tool := range tools {
		if tool.Name == listMachinesToolName {
			continue
		}
		assert.True(t, strings.Contains(string(tool.InputSchema), "machine_id") ||
			tool.Name == createMachineToolName || tool.Name == restoreMachineToolName,
			"tool %s has the schema %s", tool.Name, tool.InputSchema)
	}
}

func TestCreateRejectsInvalidPalette(t *testing.T) {
	_, client, rom := counterServer(t)
	path := filepath.Join(filepath.Dir(rom), "invalid.pal")
	for _, size := range []int{0, 1, 1537} {
		assert.NoError(t, os.WriteFile(path, make([]byte, size), 0o600))
		result, err := client.CallTool(t.Context(), createMachineToolName, map[string]any{
			romKey:     rom,
			paletteKey: path,
		})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
	}
	outside := filepath.Join(t.TempDir(), "outside.pal")
	assert.NoError(t, os.WriteFile(outside, make([]byte, 192), 0o600))
	result, err := client.CallTool(t.Context(), createMachineToolName, map[string]any{
		romKey:     rom,
		paletteKey: outside,
	})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
}

// writePictureROM writes a CHR-ROM cartridge. Tile 1 is solid color 3. The
// program waits for two vertical blanks, sets the background palette to a
// black backdrop and white colors, writes tile 1 to the top left of nametable
// 0, and enables the background. It writes $2A to $3F10 and $16 to $3F14.
func writePictureROM(t *testing.T, path string) {
	t.Helper()

	cart := cartridge.New()
	for i := 16; i < 32; i++ {
		cart.CHR[i] = 0xFF
	}

	program := make([]byte, 0, 96)
	program = append(program,
		0xad, 0x02, 0x20, 0x10, 0xfb, // wait: LDA $2002; BPL wait
		0xad, 0x02, 0x20, 0x10, 0xfb,
		0xa9, 0x3f, 0x8d, 0x06, 0x20, // $2006 = $3F00
		0xa9, 0x00, 0x8d, 0x06, 0x20,
		0xa9, 0x0f, 0x8d, 0x07, 0x20, // $3F00 = $0F
		0xa9, 0x30, 0x8d, 0x07, 0x20, // $3F01 = $30
		0x8d, 0x07, 0x20, // $3F02 = $30
		0x8d, 0x07, 0x20, // $3F03 = $30
		0xa9, 0x3f, 0x8d, 0x06, 0x20, // $2006 = $3F10
		0xa9, 0x10, 0x8d, 0x06, 0x20,
		0xa9, 0x2a, 0x8d, 0x07, 0x20, // $3F10 = $2A, mirrors $3F00
		0xa9, 0x20, 0x8d, 0x06, 0x20, // $2006 = $2000
		0xa9, 0x00, 0x8d, 0x06, 0x20,
		0xa9, 0x01, 0x8d, 0x07, 0x20, // tile 1 at $2000
		0xa9, 0x00, 0x8d, 0x06, 0x20, // reset the scroll address
		0x8d, 0x06, 0x20,
		0xa9, 0x0a, 0x8d, 0x01, 0x20, // show the background
	)
	end := 0x8000 + len(program)
	program = append(program, 0x4c, byte(end), byte(end>>8)) // JMP *
	copy(cart.PRG, program)
	binary.LittleEndian.PutUint16(cart.PRG[0x7ffc:], 0x8000)

	file := createFile(t, path)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())
}

// pictureMachine creates a machine over the picture ROM after three frames.
func pictureMachine(t *testing.T, palettePath string) (*mcptest.Client, string) {
	t.Helper()

	dir := t.TempDir()
	romPath := filepath.Join(dir, "picture.nes")
	writePictureROM(t, romPath)
	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	t.Cleanup(client.Close)

	args := map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	}
	if palettePath != "" {
		args[paletteKey] = palettePath
	}
	created := callTool[createMachineResult](t, client, createMachineToolName, args)
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: created.MachineID,
		framesKey:    3,
	})
	return client, created.MachineID
}
