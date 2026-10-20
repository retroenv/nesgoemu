package mcpserver

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

// The test uses the MCP tools and restores after it deletes the palette file.
// Event logs contain the palette bytes, so a replay shows the same colors.
func TestDisplayPaletteSurvivesReplay(t *testing.T) {
	server, client, rom := counterServer(t)
	path := filepath.Join(filepath.Dir(rom), "test.pal")
	assert.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{200, 160, 120}, 64), 0o600))
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: rom, entrypointKey: 0x8000, paletteKey: path,
	})
	id := created.MachineID
	callTool[runFramesOut](t, client, "run_frames", map[string]any{machineIDKey: id, framesKey: 2})
	want := snapshotMachine(t, client, id)
	checkpoint := callTool[checkpointResult](t, client, "checkpoint", map[string]any{machineIDKey: id})
	assert.NoError(t, os.Remove(path))

	raw, err := client.CallTool(t.Context(), "screenshot", map[string]any{machineIDKey: id})
	assert.NoError(t, err)
	rawImage, err := png.Decode(bytes.NewReader(imageContent(t, raw)))
	assert.NoError(t, err)
	red, green, blue, _ := rawImage.At(0, 0).RGBA()
	assert.Equal(t, uint32(200*257), red)
	assert.Equal(t, uint32(160*257), green)
	assert.Equal(t, uint32(120*257), blue)
	assert.Equal(t, want, snapshotMachine(t, client, id))

	fork := callTool[createMachineResult](t, client, "fork_machine", map[string]any{machineIDKey: id})
	assert.Equal(t, want, snapshotMachine(t, client, fork.MachineID))
	restored := callTool[createMachineResult](t, client, "restore_machine", map[string]any{
		romKey: rom, eventsKey: checkpoint.EventsFile,
	})
	assert.Equal(t, want, snapshotMachine(t, client, restored.MachineID))
	callTool[runFramesOut](t, client, "run_frames", map[string]any{machineIDKey: id, framesKey: 3})
	callTool[restoreResult](t, client, "restore", map[string]any{
		machineIDKey: id, checkpointIDKey: checkpoint.CheckpointID,
	})
	assert.Equal(t, want, snapshotMachine(t, client, id))

	machine, err := server.registry.get(id)
	assert.NoError(t, err)
	assert.NoError(t, machine.hardReset(nil))
	callTool[runFramesOut](t, client, "run_frames", map[string]any{machineIDKey: id, framesKey: 2})
	assert.Equal(t, want, snapshotMachine(t, client, id))
}

func TestCreateRejectsInvalidPalette(t *testing.T) {
	_, client, rom := counterServer(t)
	path := filepath.Join(filepath.Dir(rom), "invalid.pal")
	for _, size := range []int{0, 1, 1537} {
		assert.NoError(t, os.WriteFile(path, make([]byte, size), 0o600))
		result, err := client.CallTool(t.Context(), "create_machine", map[string]any{romKey: rom, paletteKey: path})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
	}
	outside := filepath.Join(t.TempDir(), "outside.pal")
	assert.NoError(t, os.WriteFile(outside, make([]byte, 192), 0o600))
	result, err := client.CallTool(t.Context(), "create_machine", map[string]any{romKey: rom, paletteKey: outside})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
}
