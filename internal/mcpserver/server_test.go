package mcpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"image/png"
	"io"
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
	hardKey         = "hard"
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
	eventsKey       = "events"
	paletteKey      = "palette"
	limitKey        = "limit"
	samplesKey      = "samples"
	timeoutKey      = "timeout"
	shortTimeout    = "20ms"

	execKind = "exec"

	modeKey       = "mode"
	enabledKey    = "enabled"
	sectionsKey   = "sections"
	memoryKey     = "memory"
	scaleKey      = "scale"
	captureKey    = "capture"
	screenshotKey = "screenshot"

	checkpointIDKey = "checkpoint_id"
)

// newTestServer creates a server with the given options and a test logger.
func newTestServer(t *testing.T, opts Options) *Server {
	t.Helper()

	server, err := New(log.NewTestLogger(t), opts)
	assert.NoError(t, err)
	return server
}

func TestServeClosesInputOnCancellation(t *testing.T) {
	server := newTestServer(t, Options{})
	reader, writer := io.Pipe()
	defer func() { _ = writer.Close() }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, reader, io.Discard) }()
	_, err := io.WriteString(writer, "\n")
	assert.NoError(t, err)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after cancellation")
	}

	_, err = io.WriteString(writer, "\n")
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestCreateStepReadScreenshot(t *testing.T) {
	client, created := syntheticMachineClient(t)

	discover, err := client.Discover(t.Context())
	assert.NoError(t, err)
	assert.Contains(t, discover.Instructions, "machine_id")

	assert.NotEmpty(t, created.MachineID)
	assert.Equal(t, uint16(0), created.ROM.MapperID)
	assert.Equal(t, 0x8000, created.ROM.PRGSize)

	var stepped stepResultOut
	err = client.CallToolInto(t.Context(), stepToolName, map[string]any{
		"mode":          modeInstructions,
		machineIDKey:    created.MachineID,
		instructionsKey: 2,
	}, &stepped)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), stepped.Instructions)
	assert.Equal(t, byte(0x2a), stepped.After.A)

	var memory readMemoryResult
	err = client.CallToolInto(t.Context(), readMemoryToolName, map[string]any{
		machineIDKey: created.MachineID,
		spaceKey:     spaceRAM,
		addressKey:   0x10,
		lengthKey:    1,
	}, &memory)
	assert.NoError(t, err)
	assert.Equal(t, 1, memory.Available)
	assert.Equal(t, 0x2a, memory.Bytes[0])

	result, err := client.CallTool(t.Context(), screenshotToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)

	image := imageContent(t, result)
	decoded, err := png.Decode(bytes.NewReader(image))
	assert.NoError(t, err)
	assert.Equal(t, 256, decoded.Bounds().Dx())
	assert.Equal(t, 240, decoded.Bounds().Dy())

	info := inspectSection[machineInfoResult](t, client, sectionCartridge, created.MachineID)
	assert.Equal(t, "vertical", info.Mirroring)
}

// syntheticMachineClient creates a client and a machine over the synthetic ROM.
func syntheticMachineClient(t *testing.T) (*mcptest.Client, createMachineResult) {
	t.Helper()

	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	writeSyntheticROM(t, romPath)

	server := newTestServer(t, Options{
		Name:    testServerName,
		Version: "0",

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	t.Cleanup(client.Close)

	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	return client, created
}

func TestRunFramesAdvancesTime(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	writeSyntheticROM(t, romPath)

	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	var created createMachineResult
	err := client.CallToolInto(t.Context(), createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)

	var run runFramesOut
	err = client.CallToolInto(t.Context(), runFramesToolName, map[string]any{
		machineIDKey: created.MachineID,
		framesKey:    3,
		"inputs": []map[string]any{{
			frameKey:   0,
			buttonsKey: []string{"A", "Start"},
		}},
	}, &run)
	assert.NoError(t, err)
	assert.Equal(t, uint64(3), run.CompletedFrames)
	assert.Equal(t, "frames", run.Reason)
}

func TestCreateRejectsPathOutsideRoots(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	result, err := client.CallTool(t.Context(), createMachineToolName, map[string]any{romKey: "/etc/hostname"})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestUnknownMachineHasRecreateHint(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	result, err := client.CallTool(t.Context(), inspectMachineToolName, map[string]any{machineIDKey: "missing"})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, createMachineToolName)
}

func TestMachineCapRejectsExtraMachine(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	writeSyntheticROM(t, romPath)

	server := newTestServer(t, Options{
		Name: testServerName,

		MaxMachines: 1,
		Roots:       []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	var created createMachineResult
	err := client.CallToolInto(t.Context(), createMachineToolName, map[string]any{romKey: romPath}, &created)
	assert.NoError(t, err)

	result, err := client.CallTool(t.Context(), createMachineToolName, map[string]any{romKey: romPath})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "limit")
}

func TestReadDocsResource(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	contents, err := client.ReadResource(t.Context(), docsURI)
	assert.NoError(t, err)
	assert.Len(t, contents, 1)
	assert.Contains(t, contents[0].Text, createMachineToolName)
}

func TestScreenResource(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "test.nes")
	writeSyntheticROM(t, romPath)

	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	var created createMachineResult
	err := client.CallToolInto(t.Context(), createMachineToolName, map[string]any{romKey: romPath}, &created)
	assert.NoError(t, err)

	contents, err := client.ReadResource(t.Context(), "nesgoemu://machines/"+created.MachineID+"/screen")
	assert.NoError(t, err)
	assert.Len(t, contents, 1)
	assert.NotEmpty(t, contents[0].Blob)
}

func TestAddressAcceptsForms(t *testing.T) {
	tests := []struct {
		input string
		name  string
		want  Address
	}{
		{
			input: "49152",
			name:  "integer",
			want:  0xC000,
		},
		{
			input: `"$C000"`,
			name:  "dollar",
			want:  0xC000,
		},
		{
			input: `"0xC000"`,
			name:  "hex",
			want:  0xC000,
		},
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
	machine := &Machine{
		id: "old",
	}
	machine.touch()
	machine.modified.Store(time.Now().Add(-2 * time.Minute).UnixNano())
	assert.NoError(t, registry.add(machine))

	registry.mu.Lock()
	registry.expireLocked()
	registry.mu.Unlock()

	_, err := registry.get("old")
	assert.Error(t, err)
}

func TestInspectMachineReturnsSelectedSections(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	before := machine.cpuState()
	var result struct {
		CPU *cpuStateResult `json:"cpu"`
		PPU *ppuSnapshotOut `json:"ppu"`
		OAM *oamResult      `json:"oam"`
	}
	err = client.CallToolInto(t.Context(), inspectMachineToolName, map[string]any{
		machineIDKey: created.MachineID,
		sectionsKey:  []string{"cpu", "ppu"},
	}, &result)
	assert.NoError(t, err)
	assert.NotNil(t, result.CPU)
	assert.NotNil(t, result.PPU)
	assert.Nil(t, result.OAM)
	assert.Equal(t, before, *result.CPU)
	assert.Equal(t, before.Frame, result.PPU.Frame)
	assert.Equal(t, before, machine.cpuState())
}

func TestRunFramesCaptureMatchesSeparateObservations(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	fork := callTool[createMachineResult](t, client, forkMachineToolName, map[string]any{machineIDKey: created.MachineID})
	captured, err := client.CallTool(t.Context(), runFramesToolName, map[string]any{
		machineIDKey: created.MachineID,
		framesKey:    2,
		captureKey: map[string]any{
			screenshotKey: true,
			memoryKey: []map[string]any{{
				spaceKey:   spaceRAM,
				addressKey: 0x10,
				lengthKey:  2,
			}},
		},
	})
	assert.NoError(t, err)
	assert.False(t, captured.IsError)
	assert.NotEmpty(t, imageContent(t, captured))
	data, err := json.Marshal(captured.StructuredContent)
	assert.NoError(t, err)
	var observed runFramesOut
	assert.NoError(t, json.Unmarshal(data, &observed))
	assert.NotNil(t, observed.Capture)
	assert.True(t, observed.Capture.Screenshot)
	assert.Len(t, observed.Capture.Memory, 1)
	separate := callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: fork.MachineID,
		framesKey:    2,
	})
	assert.Equal(t, uint64(2), separate.CompletedFrames)
	assert.Equal(t, separate.Frame, observed.Frame)
	assert.Equal(t, separate.CPUCycles, observed.CPUCycles)
	memory := callTool[readMemoryResult](t, client, readMemoryToolName, map[string]any{
		machineIDKey: fork.MachineID,
		spaceKey:     spaceRAM,
		addressKey:   0x10,
		lengthKey:    2,
	})
	assert.Equal(t, memory, observed.Capture.Memory[0])
	screen, err := client.CallTool(t.Context(), screenshotToolName, map[string]any{machineIDKey: fork.MachineID})
	assert.NoError(t, err)
	assert.Equal(t, sha256.Sum256(imageContent(t, screen)), sha256.Sum256(imageContent(t, captured)))
}

func TestInspectionDefaultsAndInvalidSections(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	defaultResult := callTool[inspectMachineResult](t, client, inspectMachineToolName,
		map[string]any{machineIDKey: created.MachineID})
	assert.NotNil(t, defaultResult.CPU)
	assert.Nil(t, defaultResult.PPU)
	for _, sections := range []any{[]string{}, []string{sectionCPU, sectionCPU}, []string{"unknown"}, nil} {
		result, err := client.CallTool(t.Context(), inspectMachineToolName, map[string]any{
			machineIDKey: created.MachineID,
			sectionsKey:  sections,
		})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
	}
	all := callTool[inspectMachineResult](t, client, inspectMachineToolName, map[string]any{
		machineIDKey: created.MachineID,
		sectionsKey:  inspectionSections,
	})
	assert.NotNil(t, all.CPU)
	assert.NotNil(t, all.PPU)
	assert.NotNil(t, all.APU)
	assert.NotNil(t, all.Cartridge)
	assert.NotNil(t, all.Banks)
	assert.NotNil(t, all.Palette)
	assert.NotNil(t, all.OAM)
}

func TestCaptureBoundsMemoryRanges(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	reads := make([]map[string]any, maxCaptureReads+1)
	for index := range reads {
		reads[index] = map[string]any{
			spaceKey:   spaceRAM,
			addressKey: index,
			lengthKey:  1,
		}
	}
	arguments := map[string]any{
		machineIDKey: created.MachineID,
		framesKey:    0,
		captureKey:   map[string]any{memoryKey: reads},
	}
	result, err := client.CallTool(t.Context(), runFramesToolName, arguments)
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	arguments[captureKey] = map[string]any{memoryKey: reads[:maxCaptureReads]}
	valid := callTool[runFramesOut](t, client, runFramesToolName, arguments)
	assert.Len(t, valid.Capture.Memory, maxCaptureReads)
}

func TestRunFramesRejectsInvalidCaptureBeforeExecution(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	before := machine.cpuState()
	for _, capture := range []map[string]any{
		{
			screenshotKey: true,
			scaleKey:      0,
		},
		{
			scaleKey: 2,
		},
		{
			memoryKey: []map[string]any{{
				spaceKey:  spaceCPU,
				lengthKey: 0,
			}},
		},
		{
			memoryKey: []map[string]any{{
				spaceKey: "registers",
			}},
		},
		{
			memoryKey: []map[string]any{{
				spaceKey:  spaceCPU,
				offsetKey: 1,
			}},
		},
	} {
		result, err := client.CallTool(t.Context(), runFramesToolName, map[string]any{
			machineIDKey: created.MachineID,
			framesKey:    1,
			captureKey:   capture,
		})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Equal(t, before, machine.cpuState())
	}
}

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

// inspectSection decodes one named section from the public inspection result.
func inspectSection[T any](t *testing.T, client *mcptest.Client, section, machineID string) T {
	t.Helper()

	var result map[string]json.RawMessage
	err := client.CallToolInto(t.Context(), inspectMachineToolName, map[string]any{
		machineIDKey: machineID,
		sectionsKey:  []string{section},
	}, &result)
	assert.NoError(t, err)
	data, ok := result[section]
	assert.True(t, ok)
	var out T
	assert.NoError(t, json.Unmarshal(data, &out))
	return out
}
