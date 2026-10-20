package mcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp/mcptest"
)

// callError calls a tool and returns the error text of a failed call. It fails
// the test when the call succeeds.
func callError(t *testing.T, client *mcptest.Client, name string, args map[string]any) string {
	t.Helper()

	result, err := client.CallTool(t.Context(), name, args)
	if err != nil {
		return err.Error()
	}
	assert.True(t, result.IsError, "call %s did not fail", name)
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}

func TestRootsRejectSymbolicLinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeCounterROM(t, filepath.Join(outside, "secret.nes"))

	assert.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))
	assert.NoError(t, os.Symlink(filepath.Join(outside, "secret.nes"), filepath.Join(root, "file.nes")))

	allowed, err := newRoots([]string{root})
	assert.NoError(t, err)

	_, err = allowed.resolve(filepath.Join(root, "link", "secret.nes"))
	assert.ErrorContains(t, err, "outside the allowed roots")
	_, err = allowed.resolve(filepath.Join(root, "file.nes"))
	assert.ErrorContains(t, err, "outside the allowed roots")
	_, err = allowed.resolve(filepath.Join(root, "link", "new.cdl"))
	assert.ErrorContains(t, err, "outside the allowed roots")

	inside, err := allowed.resolve(filepath.Join(root, "new", "output.cdl"))
	assert.NoError(t, err)
	assert.Contains(t, inside, "output.cdl")

	relative, err := allowed.resolve("game.nes")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(allowed.first(), "game.nes"), relative)
	_, err = allowed.resolve("../escape.nes")
	assert.ErrorContains(t, err, "outside the allowed roots")
}

func TestRootsAcceptRootThatIsASymbolicLink(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "roms")
	assert.NoError(t, os.Symlink(target, link))
	writeCounterROM(t, filepath.Join(target, "game.nes"))

	allowed, err := newRoots([]string{link})
	assert.NoError(t, err)

	_, err = allowed.resolve(filepath.Join(link, "game.nes"))
	assert.NoError(t, err)
}

func TestParseTimeoutLimits(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "empty selects the default", input: "", want: defaultRunTimeout},
		{name: "zero selects the default", input: "0s", want: defaultRunTimeout},
		{name: "explicit value", input: "3s", want: 3 * time.Second},
		{name: "at the limit", input: "2m", want: maxRunTimeout},
		{name: "above the limit", input: "10m", wantErr: true},
		{name: "negative", input: "-1s", wantErr: true},
		{name: "not a duration", input: "soon", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeout(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateMachineRejectsEntrypointOutsideAddressSpace(t *testing.T) {
	_, client, romPath := counterServer(t)

	for _, entrypoint := range []int{-5, 0x10000} {
		message := callError(t, client, "create_machine", map[string]any{
			romKey: romPath, entrypointKey: entrypoint,
		})
		assert.Contains(t, message, "entrypoint")
	}
}

func TestWriteMemoryChecksWholeRangeBeforeWriting(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	// The range starts in RAM and ends in the PPU registers.
	message := callError(t, client, "write_memory", map[string]any{
		machineIDKey: id, spaceKey: spaceCPU, addressKey: 0x1FFE, bytesKey: []int{1, 2, 3, 4},
	})
	assert.Contains(t, message, "not writable")

	memory := callTool[readMemoryResult](t, client, "read_memory", map[string]any{
		machineIDKey: id, spaceKey: spaceRAM, addressKey: 0x07FE, lengthKey: 2,
	})
	assert.Equal(t, []int{0, 0}, memory.Bytes)
	log := callTool[eventLogResult](t, client, "event_log", map[string]any{machineIDKey: id})
	assert.Equal(t, 0, log.Total)

	message = callError(t, client, "write_memory", map[string]any{
		machineIDKey: id, spaceKey: spaceCPU, addressKey: 0x10, bytesKey: []int{256},
	})
	assert.Contains(t, message, "outside 0 to 255")
}

func TestReadMemoryReachesPRGBeyond64KiB(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "big.nes")

	cart := cartridge.New()
	cart.Mapper = 2
	cart.PRG = make([]byte, 0x20000)
	cart.PRG[0x1ABCD] = 0x7E
	file := createFile(t, romPath)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())

	server := newTestServer(t, Options{
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{romKey: romPath})
	memory := callTool[readMemoryResult](t, client, "read_memory", map[string]any{
		machineIDKey: created.MachineID, spaceKey: spacePRG, offsetKey: 0x1ABCD, lengthKey: 2,
	})
	assert.Equal(t, 0x1ABCD, memory.Start)
	assert.Equal(t, 0x7E, memory.Bytes[0])
	assert.Contains(t, memory.Hex, "01ABCD:")

	message := callError(t, client, "read_memory", map[string]any{
		machineIDKey: created.MachineID, spaceKey: spaceCPU, offsetKey: 5,
	})
	assert.Contains(t, message, "prg and chr")
}

func TestReadStatusRegisterKeepsPPUState(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID
	assert.NoError(t, runFramesTool(t, client, id, 2))

	before := callTool[ppuSnapshotOut](t, client, "ppu_state", map[string]any{machineIDKey: id})
	memory := callTool[readMemoryResult](t, client, "read_memory", map[string]any{
		machineIDKey: id, spaceKey: spaceCPU, addressKey: 0x2002, lengthKey: 1,
	})
	after := callTool[ppuSnapshotOut](t, client, "ppu_state", map[string]any{machineIDKey: id})

	assert.Equal(t, 0, memory.Available)
	assert.Equal(t, before, after)
}

func TestInvalidArgumentsAreRejected(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	tests := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{
			name: "filter value above a byte",
			tool: "memory_search",
			args: map[string]any{actionKey: filterAction, "op": searchValue, "value": 300},
			want: "outside 0 to 255",
		},
		{
			name: "unknown filter",
			tool: "memory_search",
			args: map[string]any{actionKey: filterAction, "op": "bigger"},
			want: "unknown filter",
		},
		{
			name: "negative trace offset",
			tool: "get_trace",
			args: map[string]any{offsetKey: -1},
			want: "negative",
		},
		{
			name: "breakpoint value above a byte",
			tool: "add_breakpoint",
			args: map[string]any{kindKey: "write", addressKey: 0x10, "value": 999},
			want: "outside 0 to 255",
		},
		{
			name: "timeout above the limit",
			tool: "run_frames",
			args: map[string]any{framesKey: 1, "timeout": "1h"},
			want: "exceeds the limit",
		},
		{
			name: "record zero samples",
			tool: "record_audio",
			args: map[string]any{pathKey: "x.wav", "samples": 0},
			want: "outside 1 to",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{machineIDKey: id}
			for key, value := range tt.args {
				args[key] = value
			}
			assert.Contains(t, callError(t, client, tt.tool, args), tt.want)
		})
	}
}

func TestRecordAudioWritesRequestedSamples(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})

	path := filepath.Join(filepath.Dir(romPath), "out.wav")
	result := callTool[recordAudioResult](t, client, "record_audio", map[string]any{
		machineIDKey: created.MachineID, pathKey: path, "samples": 500,
	})
	assert.Equal(t, uint64(500), result.Samples)

	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.Equal(t, int64(44+500*2), info.Size())
}

// TestConcurrentCallsOnOneMachine runs tools from several clients on one
// machine. The race detector reports any tool that skips the machine mutex.
func TestConcurrentCallsOnOneMachine(t *testing.T) {
	server, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	calls := []struct {
		tool string
		args map[string]any
	}{
		{tool: "run_frames", args: map[string]any{framesKey: 1}},
		{tool: "cdl_start", args: map[string]any{}},
		{tool: "cdl_stop", args: map[string]any{}},
		{tool: "cdl_reset", args: map[string]any{}},
		{tool: "coverage", args: map[string]any{}},
		{tool: "cpu_state", args: map[string]any{}},
		{tool: "ppu_state", args: map[string]any{}},
		{tool: "bank_state", args: map[string]any{}},
		{tool: "trace_start", args: map[string]any{}},
		{tool: "get_trace", args: map[string]any{}},
		{tool: "call_stack", args: map[string]any{}},
		{tool: "list_machines", args: map[string]any{}},
	}

	failures := make(chan error, len(calls))
	var group sync.WaitGroup
	for _, call := range calls {
		group.Add(1)
		go func() {
			defer group.Done()

			worker := mcptest.New(t.Context(), server.mcp)
			defer worker.Close()

			for range 5 {
				args := map[string]any{machineIDKey: id}
				for key, value := range call.args {
					args[key] = value
				}
				if _, err := worker.CallTool(t.Context(), call.tool, args); err != nil {
					failures <- fmt.Errorf("call %s: %w", call.tool, err)
					return
				}
			}
		}()
	}
	group.Wait()
	close(failures)

	for err := range failures {
		assert.NoError(t, err)
	}
}
