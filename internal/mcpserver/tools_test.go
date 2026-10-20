package mcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp/mcptest"
)

// createFile creates a file for a test.
func createFile(t *testing.T, path string) *os.File {
	t.Helper()

	file, err := os.Create(path)
	assert.NoError(t, err)
	return file
}

// filepathJoin joins path elements.
func filepathJoin(elements ...string) string {
	return filepath.Join(elements...)
}

// writeCounterROM writes a cartridge that increments zero page address $10 in
// a loop, so RAM changes with the elapsed time.
func writeCounterROM(t *testing.T, path string) {
	t.Helper()

	cart := cartridge.New()
	program := []byte{0xe6, 0x10, 0x4c, 0x00, 0x80} // INC $10; JMP $8000
	copy(cart.PRG, program)

	file := createFile(t, path)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())
}

// counterServer creates a server and client over a counter ROM.
func counterServer(t *testing.T) (*Server, *mcptest.Client, string) {
	t.Helper()

	dir := t.TempDir()
	romPath := filepathJoin(dir, "counter.nes")
	writeCounterROM(t, romPath)

	server := newTestServer(t, Options{
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	t.Cleanup(client.Close)
	return server, client, romPath
}

func TestDisassembleTool(t *testing.T) {
	dir := t.TempDir()
	romPath := filepathJoin(dir, "test.nes")
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

	var decoded disassembleResult
	err = client.CallToolInto(t.Context(), "disassemble", map[string]any{
		machineIDKey: created.MachineID, addressKey: 0x8000, "count": 2,
	}, &decoded)
	assert.NoError(t, err)
	assert.Len(t, decoded.Instructions, 2)
	assert.Equal(t, "LDA", decoded.Instructions[0].Mnemonic)
	assert.Equal(t, "#$2A", decoded.Instructions[0].Operand)
}

func TestBreakpointAndRunUntil(t *testing.T) {
	_, client, romPath := counterServer(t)

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)

	var breakpoint addBreakpointResult
	err = client.CallToolInto(t.Context(), "add_breakpoint", map[string]any{
		machineIDKey: created.MachineID, kindKey: execKind, addressKey: 0x8000, "once": true,
	}, &breakpoint)
	assert.NoError(t, err)

	var run runUntilResult
	err = client.CallToolInto(t.Context(), "run_until", map[string]any{
		machineIDKey: created.MachineID,
	}, &run)
	assert.NoError(t, err)
	assert.Equal(t, string("breakpoint"), run.Reason)
	assert.Equal(t, breakpoint.ID, run.BreakpointID)

	var listed listBreakpointsResult
	err = client.CallToolInto(t.Context(), "list_breakpoints", map[string]any{machineIDKey: created.MachineID}, &listed)
	assert.NoError(t, err)
	assert.Empty(t, listed.Breakpoints)
}

func TestCheckpointRestoreDeterministic(t *testing.T) {
	server, client, romPath := counterServer(t)
	_ = server

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)
	id := created.MachineID

	// Checkpoint at frame zero, then run three frames for the reference value.
	var checkpoint checkpointResult
	err = client.CallToolInto(t.Context(), "checkpoint", map[string]any{machineIDKey: id}, &checkpoint)
	assert.NoError(t, err)

	// Reference: run three frames and read the counter.
	assert.NoError(t, runFramesTool(t, client, id, 3))
	reference := readCounter(t, client, id)

	// Restore to the checkpoint and run three frames again.
	var restored restoreResult
	err = client.CallToolInto(t.Context(), "restore", map[string]any{
		machineIDKey: id, checkpointIDKey: checkpoint.CheckpointID,
	}, &restored)
	assert.NoError(t, err)

	assert.NoError(t, runFramesTool(t, client, id, 3))
	assert.Equal(t, reference, readCounter(t, client, id))
}

func TestForkMachineMatchesReplay(t *testing.T) {
	server, client, romPath := counterServer(t)

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)

	assert.NoError(t, runFramesTool(t, client, created.MachineID, 3))
	reference := readCounter(t, client, created.MachineID)

	frame := uint64(3)
	var fork createMachineResult
	err = client.CallToolInto(t.Context(), "fork_machine", map[string]any{
		machineIDKey: created.MachineID, frameKey: frame,
	}, &fork)
	assert.NoError(t, err)
	assert.NotEmpty(t, fork.MachineID)
	assert.NotEqual(t, created.MachineID, fork.MachineID)
	assert.Equal(t, reference, readCounter(t, client, fork.MachineID))
	_ = server
}

func TestMemorySearchFindsCounter(t *testing.T) {
	server, client, romPath := counterServer(t)
	_ = server

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)
	id := created.MachineID

	var start memorySearchResult
	err = client.CallToolInto(t.Context(), "memory_search", map[string]any{
		machineIDKey: id, actionKey: "start",
	}, &start)
	assert.NoError(t, err)

	var filter memorySearchResult
	err = client.CallToolInto(t.Context(), "memory_search", map[string]any{
		machineIDKey: id, actionKey: filterAction, "op": "changed", framesKey: 2,
	}, &filter)
	assert.NoError(t, err)
	assert.True(t, filter.Candidates >= 1 && filter.Candidates < start.Candidates)
}

func TestCDLCoverageAndExport(t *testing.T) {
	server, client, romPath := counterServer(t)

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)
	id := created.MachineID

	var started cdlState
	err = client.CallToolInto(t.Context(), "cdl_start", map[string]any{machineIDKey: id}, &started)
	assert.NoError(t, err)

	assert.NoError(t, runFramesTool(t, client, id, 1))

	var stopped cdlState
	err = client.CallToolInto(t.Context(), "cdl_stop", map[string]any{machineIDKey: id}, &stopped)
	assert.NoError(t, err)

	var coverage coverageResult
	err = client.CallToolInto(t.Context(), "coverage", map[string]any{machineIDKey: id}, &coverage)
	assert.NoError(t, err)
	assert.NotEmpty(t, coverage.Banks)
	assert.True(t, coverage.Banks[0].Code > 0)

	cdlPath := filepathJoin(server.roots.first(), "out.cdl")
	var exported cdlExportResult
	err = client.CallToolInto(t.Context(), "cdl_export", map[string]any{
		machineIDKey: id, pathKey: cdlPath,
	}, &exported)
	assert.NoError(t, err)
	assert.Equal(t, cdlPath, exported.Path)
}

func TestPPUTools(t *testing.T) {
	server, client, romPath := counterServer(t)

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)
	id := created.MachineID

	var state ppuSnapshotOut
	err = client.CallToolInto(t.Context(), "ppu_state", map[string]any{machineIDKey: id}, &state)
	assert.NoError(t, err)

	var palette paletteResult
	err = client.CallToolInto(t.Context(), "palette", map[string]any{machineIDKey: id}, &palette)
	assert.NoError(t, err)
	assert.Len(t, palette.Entries, 32)

	var oam oamResult
	err = client.CallToolInto(t.Context(), "oam", map[string]any{machineIDKey: id}, &oam)
	assert.NoError(t, err)
	assert.Len(t, oam.Sprites, 64)

	result, err := client.CallTool(t.Context(), "pattern_tables", map[string]any{machineIDKey: id})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	assert.NotEmpty(t, imageContent(t, result))
	_ = server
}

func TestTraceRingTool(t *testing.T) {
	server, client, romPath := counterServer(t)
	_ = server

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	}, &created)
	assert.NoError(t, err)
	id := created.MachineID

	var started traceState
	err = client.CallToolInto(t.Context(), "trace_start", map[string]any{machineIDKey: id}, &started)
	assert.NoError(t, err)

	var stepped stepResultOut
	err = client.CallToolInto(t.Context(), "step", map[string]any{
		machineIDKey: id, instructionsKey: 2,
	}, &stepped)
	assert.NoError(t, err)

	var trace getTraceResult
	err = client.CallToolInto(t.Context(), "get_trace", map[string]any{machineIDKey: id}, &trace)
	assert.NoError(t, err)
	assert.NotEmpty(t, trace.Lines)
	assert.Contains(t, trace.Lines[0], "INC")
}

// runFramesTool runs a frame count through the run_frames tool.
func runFramesTool(t *testing.T, client *mcptest.Client, machineID string, frames uint64) error {
	t.Helper()

	var out runFramesOut
	if err := client.CallToolInto(t.Context(), "run_frames", map[string]any{
		machineIDKey: machineID, framesKey: frames,
	}, &out); err != nil {
		return fmt.Errorf("running frames: %w", err)
	}
	return nil
}

// readCounter reads zero page address $10.
func readCounter(t *testing.T, client *mcptest.Client, machineID string) int {
	t.Helper()

	var memory readMemoryResult
	err := client.CallToolInto(t.Context(), "read_memory", map[string]any{
		machineIDKey: machineID, spaceKey: spaceRAM, addressKey: 0x10, lengthKey: 1,
	}, &memory)
	assert.NoError(t, err)
	return memory.Bytes[0]
}

// nestestDir returns the nestest test ROM directory.
func nestestDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	assert.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "testroms", "nestest")
}

func TestNestestTraceThroughMCP(t *testing.T) {
	cpu6502.IscInst.Name = "isb"

	dir := nestestDir(t)
	if _, err := os.Stat(filepath.Join(dir, "nestest.nes")); err != nil {
		t.Skip("nestest ROM is missing")
	}

	server := newTestServer(t, Options{
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	var created createMachineResult
	err := client.CallToolInto(t.Context(), "create_machine", map[string]any{
		romKey: filepath.Join(dir, "nestest.nes"), entrypointKey: 0xC000,
	}, &created)
	assert.NoError(t, err)

	var traceOn traceState
	err = client.CallToolInto(t.Context(), "trace_start", map[string]any{
		machineIDKey: created.MachineID,
	}, &traceOn)
	assert.NoError(t, err)

	var breakpoint addBreakpointResult
	err = client.CallToolInto(t.Context(), "add_breakpoint", map[string]any{
		machineIDKey: created.MachineID, kindKey: execKind, "address": 0x0001,
	}, &breakpoint)
	assert.NoError(t, err)

	var run runUntilResult
	err = client.CallToolInto(t.Context(), "run_until", map[string]any{
		machineIDKey: created.MachineID, instructionsKey: 100000,
	}, &run)
	assert.NoError(t, err)
	assert.Equal(t, string("breakpoint"), run.Reason)

	var trace getTraceResult
	err = client.CallToolInto(t.Context(), "get_trace", map[string]any{
		machineIDKey: created.MachineID, "limit": maxPage, offsetKey: 0,
	}, &trace)
	assert.NoError(t, err)

	expected := readReferenceTrace(t, filepath.Join(dir, "nestest_no_ppu.log"))
	assert.Len(t, expected, len(trace.Lines))
	for index := range expected {
		if index >= len(trace.Lines) {
			break
		}
		assert.Equal(t, strings.TrimRight(expected[index], "\r\n"), strings.TrimRight(trace.Lines[index], "\r\n"))
	}
}

// readReferenceTrace reads the reference trace lines.
func readReferenceTrace(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.Split(strings.TrimSuffix(normalized, "\n"), "\n")
}
