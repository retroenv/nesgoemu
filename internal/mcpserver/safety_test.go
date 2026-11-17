package mcpserver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp/mcptest"
)

var concurrentToolCalls = []struct {
	args map[string]any
	tool string
}{
	{
		args: map[string]any{framesKey: 1},
		tool: runFramesToolName,
	},
	{
		args: map[string]any{enabledKey: true},
		tool: cdlSetEnabledToolName,
	},
	{
		args: map[string]any{enabledKey: false},
		tool: cdlSetEnabledToolName,
	},
	{
		args: map[string]any{},
		tool: cdlResetToolName,
	},
	{
		args: map[string]any{},
		tool: coverageToolName,
	},
	{
		args: map[string]any{sectionsKey: []string{sectionCPU}},
		tool: inspectMachineToolName,
	},
	{
		args: map[string]any{sectionsKey: []string{sectionPPU}},
		tool: inspectMachineToolName,
	},
	{
		args: map[string]any{sectionsKey: []string{sectionBanks}},
		tool: inspectMachineToolName,
	},
	{
		args: map[string]any{},
		tool: traceStartToolName,
	},
	{
		args: map[string]any{},
		tool: getTraceToolName,
	},
	{
		args: map[string]any{},
		tool: callStackToolName,
	},
	{
		args: map[string]any{},
		tool: listMachinesToolName,
	},
}

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

func TestRootsRejectDanglingSymbolicLink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "created.cdl")
	assert.NoError(t, os.Symlink(target, filepath.Join(root, "dangling.cdl")))
	assert.NoError(t, os.Symlink("../escaped/new.cdl", filepath.Join(root, "relative.cdl")))
	assert.NoError(t, os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "dir")))
	assert.NoError(t, os.Symlink("loop2", filepath.Join(root, "loop1")))
	assert.NoError(t, os.Symlink("loop1", filepath.Join(root, "loop2")))

	allowed, err := newRoots([]string{root})
	assert.NoError(t, err)

	for _, path := range []string{"dangling.cdl", "relative.cdl", "dir/new.cdl", "loop1"} {
		_, err = allowed.resolveOutput(path, ".cdl")
		assert.Error(t, err, "path %s", path)
	}

	// A dangling link that stays inside the root is allowed.
	assert.NoError(t, os.Symlink("inside.cdl", filepath.Join(root, "ok.cdl")))
	resolved, err := allowed.resolveOutput("ok.cdl", ".cdl")
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(allowed.first(), "inside.cdl"), resolved)
}

func TestRootsAcceptFilesystemRoot(t *testing.T) {
	allowed, err := newRoots([]string{string(filepath.Separator)})
	assert.NoError(t, err)

	path := filepath.Join(t.TempDir(), "game.nes")
	resolved, err := allowed.resolve(path)
	assert.NoError(t, err)
	assert.Contains(t, resolved, "game.nes")
}

func TestResolveOutputProtectsExistingFiles(t *testing.T) {
	root := t.TempDir()
	rom := filepath.Join(root, "game.nes")
	writeCounterROM(t, rom)
	assert.NoError(t, os.Mkdir(filepath.Join(root, "dir.cdl"), 0o755))

	allowed, err := newRoots([]string{root})
	assert.NoError(t, err)

	_, err = allowed.resolveOutput(rom, ".cdl")
	assert.ErrorContains(t, err, "must end in .cdl")
	_, err = allowed.resolveOutput("dir.cdl", ".cdl")
	assert.ErrorContains(t, err, "not a regular file")
	_, err = allowed.resolveOutput("new.CDL", ".cdl")
	assert.NoError(t, err)
}

func TestWriteFileAtomicKeepsTargetOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "out.bin")
	assert.NoError(t, writeFileAtomic(path, func(w io.Writer) error {
		_, err := w.Write([]byte("first"))
		return errors.Join(err)
	}))

	err := writeFileAtomic(path, func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return errors.New("failed")
	})
	assert.Error(t, err)

	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "first", string(data))
	entries, err := os.ReadDir(filepath.Dir(path))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestReadRegularFileLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.json")
	assert.NoError(t, os.WriteFile(path, []byte("12345"), 0o644))

	data, err := readRegularFile(t.Context(), path, 5)
	assert.NoError(t, err)
	assert.Equal(t, "12345", string(data))
	_, err = readRegularFile(t.Context(), path, 4)
	assert.ErrorContains(t, err, "larger than")
	_, err = readRegularFile(t.Context(), dir, 10)
	assert.ErrorContains(t, err, "not a regular file")
}

func TestOutputToolsRefuseUnsafePaths(t *testing.T) {
	server, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	before, err := os.ReadFile(romPath)
	assert.NoError(t, err)

	assert.Contains(t, callError(t, client, cdlExportToolName, map[string]any{
		machineIDKey: created.MachineID,
		pathKey:      romPath,
	}), "must end in .cdl")
	assert.Contains(t, callError(t, client, recordAudioToolName, map[string]any{
		machineIDKey: created.MachineID,
		pathKey:      romPath,
		samplesKey:   10,
	}), "must end in .wav")

	outside := t.TempDir()
	link := filepath.Join(server.roots.first(), "link.cdl")
	assert.NoError(t, os.Symlink(filepath.Join(outside, "pwned.cdl"), link))
	assert.Contains(t, callError(t, client, cdlExportToolName, map[string]any{
		machineIDKey: created.MachineID,
		pathKey:      "link.cdl",
	}), "outside the allowed roots")
	_, err = os.Stat(filepath.Join(outside, "pwned.cdl"))
	assert.True(t, errors.Is(err, os.ErrNotExist))

	after, err := os.ReadFile(romPath)
	assert.NoError(t, err)
	assert.Equal(t, before, after)
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
		input   string
		name    string
		want    time.Duration
		wantErr bool
	}{
		{
			input: "",
			name:  "empty selects the default",
			want:  defaultRunTimeout,
		},
		{
			input: "0s",
			name:  "zero selects the default",
			want:  defaultRunTimeout,
		},
		{
			input: "3s",
			name:  "explicit value",
			want:  3 * time.Second,
		},
		{
			input: "2m",
			name:  "at the limit",
			want:  maxRunTimeout,
		},
		{
			input:   "10m",
			name:    "above the limit",
			wantErr: true,
		},
		{
			input:   "-1s",
			name:    "negative",
			wantErr: true,
		},
		{
			input:   "soon",
			name:    "not a duration",
			wantErr: true,
		},
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
		message := callError(t, client, createMachineToolName, map[string]any{
			romKey:        romPath,
			entrypointKey: entrypoint,
		})
		assert.Contains(t, message, "entrypoint")
	}
}

func TestWriteMemoryChecksWholeRangeBeforeWriting(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	id := created.MachineID

	// The range starts in RAM and ends in the PPU registers.
	message := callError(t, client, writeMemoryToolName, map[string]any{
		machineIDKey: id,
		spaceKey:     spaceCPU,
		addressKey:   0x1FFE,
		bytesKey:     []int{1, 2, 3, 4},
	})
	assert.Contains(t, message, "not writable")

	memory := callTool[readMemoryResult](t, client, readMemoryToolName, map[string]any{
		machineIDKey: id,
		spaceKey:     spaceRAM,
		addressKey:   0x07FE,
		lengthKey:    2,
	})
	assert.Equal(t, []int{0, 0}, memory.Bytes)
	log := callTool[eventLogResult](t, client, eventLogToolName, map[string]any{machineIDKey: id})
	assert.Equal(t, 0, log.Total)

	message = callError(t, client, writeMemoryToolName, map[string]any{
		machineIDKey: id,
		spaceKey:     spaceCPU,
		addressKey:   0x10,
		bytesKey:     []int{256},
	})
	assert.Contains(t, message, "above the maximum")
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
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: romPath})
	memory := callTool[readMemoryResult](t, client, readMemoryToolName, map[string]any{
		machineIDKey: created.MachineID,
		spaceKey:     spacePRG,
		offsetKey:    0x1ABCD,
		lengthKey:    2,
	})
	assert.Equal(t, 0x1ABCD, memory.Start)
	assert.Equal(t, 0x7E, memory.Bytes[0])
	assert.Contains(t, memory.Hex, "01ABCD:")

	message := callError(t, client, readMemoryToolName, map[string]any{
		machineIDKey: created.MachineID,
		spaceKey:     spaceCPU,
		offsetKey:    5,
	})
	assert.Contains(t, message, "prg and chr")
}

func TestReadStatusRegisterKeepsPPUState(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	id := created.MachineID
	assert.NoError(t, runFramesTool(t, client, id, 2))

	before := inspectSection[ppuSnapshotOut](t, client, sectionPPU, id)
	memory := callTool[readMemoryResult](t, client, readMemoryToolName, map[string]any{
		machineIDKey: id,
		spaceKey:     spaceCPU,
		addressKey:   0x2002,
		lengthKey:    1,
	})
	after := inspectSection[ppuSnapshotOut](t, client, sectionPPU, id)

	assert.Equal(t, 0, memory.Available)
	assert.Equal(t, before, after)
}

func TestInvalidArgumentsAreRejected(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	id := created.MachineID

	for _, tt := range invalidArgumentTests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{machineIDKey: id}
			for key, value := range tt.args {
				args[key] = value
			}
			assert.Contains(t, callError(t, client, tt.tool, args), tt.want)
		})
	}
}

// invalidArgumentTests lists tool calls with invalid arguments and the expected error text.
var invalidArgumentTests = []struct {
	args map[string]any
	name string
	tool string
	want string
}{
	{
		args: map[string]any{
			actionKey:  searchFilter,
			fieldOp:    searchValue,
			fieldValue: 300,
		},
		name: "filter value above a byte",
		tool: memorySearchToolName,
		want: "invalid arguments",
	},
	{
		args: map[string]any{
			actionKey: searchFilter,
			fieldOp:   "bigger",
		},
		name: "unknown filter",
		tool: memorySearchToolName,
		want: "invalid arguments",
	},
	{
		args: map[string]any{offsetKey: -1},
		name: "negative trace offset",
		tool: getTraceToolName,
		want: "negative",
	},
	{
		args: map[string]any{
			kindKey:    "write",
			addressKey: 0x10,
			fieldValue: 999,
		},
		name: "breakpoint value above a byte",
		tool: addBreakpointToolName,
		want: "outside 0 to 255",
	},
	{
		args: map[string]any{
			framesKey:  1,
			timeoutKey: "1h",
		},
		name: "timeout above the limit",
		tool: runFramesToolName,
		want: "exceeds the limit",
	},
	{
		args: map[string]any{
			pathKey:    "x.wav",
			samplesKey: 0,
		},
		name: "record zero samples",
		tool: recordAudioToolName,
		want: "outside 1 to",
	},
}

func TestRecordAudioWritesRequestedSamples(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})

	path := filepath.Join(filepath.Dir(romPath), "out.wav")
	result := callTool[recordAudioResult](t, client, recordAudioToolName, map[string]any{
		machineIDKey: created.MachineID,
		pathKey:      path,
		samplesKey:   500,
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
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	id := created.MachineID

	failures := make(chan error, len(concurrentToolCalls))
	var group sync.WaitGroup
	for _, call := range concurrentToolCalls {
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

func TestBatterySaveFileSymbolicLinkRefused(t *testing.T) {
	_, client, romPath := counterServer(t)
	outside := filepath.Join(t.TempDir(), "secret.sav")
	assert.NoError(t, os.WriteFile(outside, make([]byte, 8192), 0o644))
	assert.NoError(t, os.Symlink(outside, romPath+".sav"))

	assert.Contains(t, callError(t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
		"battery":     true,
	}), "symbolic link")
}

func TestInstructionsStateConfiguredTTL(t *testing.T) {
	assert.Contains(t, instructions(5*time.Minute), "expires after 5m0s")
}

func TestMachineLimitRejectsBeforeLoading(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "counter.nes")
	writeCounterROM(t, romPath)
	server := newTestServer(t, Options{
		Name: testServerName,

		MaxMachines: 1,
		Roots:       []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	t.Cleanup(client.Close)

	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: romPath})

	// The ROM path does not exist, so a call that loads first would report it.
	missing := map[string]any{romKey: filepath.Join(dir, "missing.nes")}
	assert.Contains(t, callError(t, client, createMachineToolName, missing), "machine limit 1 reached")
	assert.Contains(t, callError(t, client, restoreMachineToolName, map[string]any{
		romKey:    filepath.Join(dir, "missing.nes"),
		eventsKey: "missing.json",
	}), "machine limit 1 reached")
	assert.Contains(t, callError(t, client, forkMachineToolName, map[string]any{
		machineIDKey: created.MachineID,
	}), "machine limit 1 reached")
}

func TestRecordAudioHonorsTimeoutAndWritesNoFile(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	path := filepath.Join(filepath.Dir(romPath), "slow.wav")

	message := callError(t, client, recordAudioToolName, map[string]any{
		machineIDKey: created.MachineID,
		pathKey:      path,
		samplesKey:   2_000_000,
		timeoutKey:   "1ms",
	})
	assert.Contains(t, message, "deadline exceeded")
	_, err := os.Stat(path)
	assert.True(t, errors.Is(err, os.ErrNotExist))

	// The machine is usable after the failed recording.
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: created.MachineID,
		framesKey:    1,
	})
}

func TestRecordAudioContainsOnlyNewAudio(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: created.MachineID,
		framesKey:    120,
	})
	before := inspectSection[cpuStateResult](t, client, sectionCPU, created.MachineID)

	path := filepath.Join(filepath.Dir(romPath), "new.wav")
	callTool[recordAudioResult](t, client, recordAudioToolName, map[string]any{
		machineIDKey: created.MachineID,
		pathKey:      path,
		samplesKey:   44100,
	})
	after := inspectSection[cpuStateResult](t, client, sectionCPU, created.MachineID)

	// One second of audio needs about 1.79 million CPU cycles of new emulation.
	elapsed := after.Cycles - before.Cycles
	assert.True(t, elapsed > 1_700_000 && elapsed < 1_900_000, "elapsed %d cycles", elapsed)
}
