package mcpserver

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/log"
	"github.com/retroenv/retrogolib/mcp"
	"github.com/retroenv/retrogolib/mcp/mcptest"
)

// Shared test literals.
const (
	testServerName  = "test"
	romKey          = "rom"
	machineIDKey    = "machine_id"
	entrypointKey   = "entrypoint"
	addressKey      = "address"
	framesKey       = "frames"
	frameKey        = "frame"
	instructionsKey = "instructions"
	spaceKey        = "space"
	lengthKey       = "length"
	bytesKey        = "bytes"
	offsetKey       = "offset"
	actionKey       = "action"
	kindKey         = "kind"
	pathKey         = "path"
	buttonsKey      = "buttons"
	inputsKey       = "inputs"
	filterAction    = "filter"
	execKind        = "exec"

	checkpointIDKey = "checkpoint_id"
)

// writeSyntheticROM writes a small NROM cartridge with a program that stores
// $2A to zero page address $10 and loops.
func writeSyntheticROM(t *testing.T, path string) {
	t.Helper()

	cart := cartridge.New()
	program := []byte{0xa9, 0x2a, 0x85, 0x10, 0xea, 0x4c, 0x00, 0x80}
	copy(cart.PRG, program)
	binary.LittleEndian.PutUint16(cart.PRG[0x7ffc:], 0x8000)

	file, err := os.Create(path)
	assert.NoError(t, err)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())
}

// newTestServer creates a server with the given options and a test logger.
func newTestServer(t *testing.T, opts Options) *Server {
	t.Helper()

	server, err := New(log.NewTestLogger(t), opts)
	assert.NoError(t, err)
	return server
}

func TestCreateStepReadScreenshot(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	writeSyntheticROM(t, romPath)

	server := newTestServer(t, Options{
		Name:    testServerName,
		Version: "0",
		Roots:   []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	discover, err := client.Discover(t.Context())
	assert.NoError(t, err)
	assert.Contains(t, discover.Instructions, "machine_id")

	var created createMachineResult
	err = client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)
	assert.NotEmpty(t, created.MachineID)
	assert.Equal(t, uint16(0), created.ROM.MapperID)
	assert.Equal(t, 0x8000, created.ROM.PRGSize)

	var stepped stepResultOut
	err = client.CallToolInto(t.Context(), "step", map[string]any{
		machineIDKey: created.MachineID, instructionsKey: 2,
	}, &stepped)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), stepped.Instructions)
	assert.Equal(t, byte(0x2a), stepped.After.A)

	var memory readMemoryResult
	err = client.CallToolInto(t.Context(), "read_memory", map[string]any{
		machineIDKey: created.MachineID, spaceKey: spaceRAM, addressKey: 0x10, lengthKey: 1,
	}, &memory)
	assert.NoError(t, err)
	assert.Equal(t, 1, memory.Available)
	assert.Equal(t, 0x2a, memory.Bytes[0])

	result, err := client.CallTool(t.Context(), "screenshot", map[string]any{
		machineIDKey: created.MachineID,
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)

	image := imageContent(t, result)
	decoded, err := png.Decode(bytes.NewReader(image))
	assert.NoError(t, err)
	assert.Equal(t, 256, decoded.Bounds().Dx())
	assert.Equal(t, 240, decoded.Bounds().Dy())

	var info machineInfoResult
	err = client.CallToolInto(t.Context(), "machine_info", map[string]any{
		machineIDKey: created.MachineID,
	}, &info)
	assert.NoError(t, err)
	assert.Equal(t, "vertical", info.Mirroring)
}

func TestRunFramesAdvancesTime(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	writeSyntheticROM(t, romPath)

	server := newTestServer(t, Options{
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)

	var run runFramesOut
	err = client.CallToolInto(t.Context(), "run_frames", map[string]any{
		machineIDKey: created.MachineID, framesKey: 3,
		"inputs": []map[string]any{{frameKey: 0, buttonsKey: []string{"A", "Start"}}},
	}, &run)
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), run.CompletedFrames)
	assert.Equal(t, "frames", run.Reason)
}

func TestCreateRejectsPathOutsideRoots(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, Options{
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	result, err := client.CallTool(t.Context(), "create_machine", map[string]any{romKey: "/etc/hostname"})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestUnknownMachineHasRecreateHint(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, Options{
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	result, err := client.CallTool(t.Context(), "cpu_state", map[string]any{machineIDKey: "missing"})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "create_machine")
}

func TestMachineCapRejectsExtraMachine(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	writeSyntheticROM(t, romPath)

	server := newTestServer(t, Options{
		Name:        testServerName,
		Roots:       []string{dir},
		MaxMachines: 1,
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{romKey: romPath}, &created)
	assert.NoError(t, err)

	result, err := client.CallTool(t.Context(), "create_machine", map[string]any{romKey: romPath})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "limit")
}

func TestReadDocsResource(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, Options{
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	contents, err := client.ReadResource(t.Context(), docsURI)
	assert.NoError(t, err)
	assert.Len(t, contents, 1)
	assert.Contains(t, contents[0].Text, "create_machine")
}

func TestScreenResource(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	writeSyntheticROM(t, romPath)

	server := newTestServer(t, Options{
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{romKey: romPath}, &created)
	assert.NoError(t, err)

	contents, err := client.ReadResource(t.Context(), "nesgoemu://machines/"+created.MachineID+"/screen")
	assert.NoError(t, err)
	assert.Len(t, contents, 1)
	assert.NotEmpty(t, contents[0].Blob)
}

func TestAddressAcceptsForms(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Address
	}{
		{name: "integer", input: "49152", want: 0xC000},
		{name: "dollar", input: `"$C000"`, want: 0xC000},
		{name: "hex", input: `"0xC000"`, want: 0xC000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var address Address
			err := address.UnmarshalJSON([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, address)
		})
	}
}

func TestRegistryExpiresIdleMachine(t *testing.T) {
	registry := newRegistry(log.NewTestLogger(t), 4, time.Minute)
	machine := &Machine{id: "old"}
	machine.touch()
	machine.modified.Store(time.Now().Add(-2 * time.Minute).UnixNano())
	assert.NoError(t, registry.add(machine))

	registry.mu.Lock()
	registry.expireLocked()
	registry.mu.Unlock()

	_, err := registry.get("old")
	assert.Error(t, err)
}

// imageContent returns the first image content data of a tool result.
func imageContent(t *testing.T, result *mcp.ToolResult) []byte {
	t.Helper()

	for _, content := range result.Content {
		if content.Type == "image" {
			return content.Data
		}
	}
	t.Fatal("tool result has no image content")
	return nil
}
