package mcpserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp/mcptest"
)

// Addresses of the recursion ROM.
const (
	recursionCaller     = 0x8000
	recursionAfterCall  = 0x8003
	recursionFunction   = 0x8010
	recursionInnerCall  = 0x8018
	recursionInnerAfter = 0x801B
)

func TestStepOverSkipsDeeperReturnToTheSameAddress(t *testing.T) {
	client, id := recursionMachine(t)
	before := inspectSection[cpuStateResult](t, client, sectionCPU, id)

	run := callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey: modeOver, machineIDKey: id})

	// The inner calls return to $801B on deeper stack levels first. The step
	// ends only when the stack pointer is back at its level before the call.
	assert.Equal(t, "returned", run.Reason)
	assert.Equal(t, Address(recursionInnerAfter), run.After.PC)
	assert.Equal(t, before.SP, run.After.SP)
	assert.Equal(t, uint64(12), run.Instructions)
	assert.Equal(t, run.After.Cycles-before.Cycles, run.Cycles)
}

func TestStepOverTimeoutKeepsStopState(t *testing.T) {
	client, id := recursionMachine(t)
	before := inspectSection[cpuStateResult](t, client, sectionCPU, id)
	message := callError(t, client, stepToolName, map[string]any{
		modeKey:      modeOver,
		machineIDKey: id,
		timeoutKey:   "1ns",
	})
	assert.Contains(t, message, "deadline exceeded")
	after := inspectSection[cpuStateResult](t, client, sectionCPU, id)
	assert.Equal(t, before, after)
}

func TestStepOverStopsAtUserBreakpoint(t *testing.T) {
	client, id := recursionMachine(t)

	breakpoint := callTool[addBreakpointResult](t, client, addBreakpointToolName, map[string]any{
		machineIDKey: id, kindKey: execKind, addressKey: recursionFunction,
	})
	run := callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey: modeOver, machineIDKey: id})

	assert.Equal(t, "breakpoint", run.Reason)
	assert.Equal(t, breakpoint.ID, run.BreakpointID)
	assert.Equal(t, Address(recursionFunction), run.After.PC)

	listed := callTool[listBreakpointsResult](t, client, listBreakpointsToolName, map[string]any{machineIDKey: id})
	assert.Len(t, listed.Breakpoints, 1)
}

func TestStepOutReturnsToTheCaller(t *testing.T) {
	client, id := recursionMachine(t)

	stack := callTool[callStackResult](t, client, callStackToolName, map[string]any{machineIDKey: id})
	assert.Len(t, stack.Frames, 1)
	assert.Equal(t, Address(recursionAfterCall), stack.Frames[0].ReturnAddress)
	assert.Equal(t, Address(recursionCaller), stack.Frames[0].CallerAddress)
	assert.Equal(t, 0, stack.Frames[0].CallerOffset)

	run := callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey: modeOut, machineIDKey: id})

	assert.Equal(t, "returned", run.Reason)
	assert.Equal(t, Address(recursionAfterCall), run.After.PC)
}

func TestSoftResetClearsCallStack(t *testing.T) {
	client, id := recursionMachine(t)

	callTool[resetResult](t, client, resetToolName, map[string]any{machineIDKey: id})

	stack := callTool[callStackResult](t, client, callStackToolName, map[string]any{machineIDKey: id})
	assert.Empty(t, stack.Frames)
}

// TestRunUntilTraceTailSurvivesFullRing checks that the trace tail still lists
// the new lines when the ring is full. A tail that counted the stored lines
// returned nothing once the ring reached its capacity.
func TestRunUntilTraceTailSurvivesFullRing(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	started := callTool[traceState](t, client, traceStartToolName, map[string]any{machineIDKey: id})
	assert.True(t, started.Enabled)
	callTool[runUntilResult](t, client, runUntilToolName, map[string]any{
		machineIDKey: id, instructionsKey: started.Capacity + 100,
	})
	full := callTool[traceState](t, client, traceStopToolName, map[string]any{machineIDKey: id})
	assert.Equal(t, full.Capacity, full.Lines)

	callTool[traceState](t, client, traceStartToolName, map[string]any{machineIDKey: id})
	callTool[runUntilResult](t, client, runUntilToolName, map[string]any{
		machineIDKey: id, instructionsKey: started.Capacity + 100,
	})
	run := callTool[runUntilResult](t, client, runUntilToolName, map[string]any{
		machineIDKey: id, instructionsKey: 5,
	})

	assert.Len(t, run.Trace, 5)
}

func TestRunUntilTraceTailIsEmptyWhileTracingIsOff(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})

	run := callTool[runUntilResult](t, client, runUntilToolName, map[string]any{
		machineIDKey: created.MachineID, instructionsKey: 5,
	})

	assert.Empty(t, run.Trace)
}

func TestRunUntilReportsInstructionOfMemoryBreakpoint(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	callTool[addBreakpointResult](t, client, addBreakpointToolName, map[string]any{
		machineIDKey: id, kindKey: "write", addressKey: 0x10,
	})
	run := callTool[runUntilResult](t, client, runUntilToolName, map[string]any{machineIDKey: id})

	assert.Equal(t, "breakpoint", run.Reason)
	assert.Equal(t, Address(0x8000), run.HitPC)
	assert.Equal(t, Address(0x10), run.HitAddress)
	// INC writes the old value back before it writes the new value.
	assert.Equal(t, 0, run.HitValue)
}

// TestHardResetReattachesRecorders checks that the recorders follow the new
// system. They stayed on the discarded system before, so the code log of a
// hard-reset machine stayed empty.
func TestHardResetReattachesRecorders(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	callTool[resetResult](t, client, resetToolName, map[string]any{machineIDKey: id, hardKey: true})
	callTool[cdlState](t, client, cdlSetEnabledToolName, map[string]any{machineIDKey: id, enabledKey: true})
	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey: modeInstructions, machineIDKey: id, instructionsKey: 4})

	coverage := callTool[coverageResult](t, client, coverageToolName, map[string]any{machineIDKey: id})
	assert.NotEmpty(t, coverage.Segments)
	assert.True(t, coverage.Segments[0].Code > 0)
}

func TestFindAccessorsSeparatesReadsWritesAndValues(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})

	found := callTool[findAccessorsResult](t, client, findAccessorsToolName, map[string]any{
		machineIDKey: created.MachineID, addressKey: 0x10, framesKey: 1,
	})

	// INC $10 reads the byte, then writes the old and the new value.
	assert.Len(t, found.Hits, 1)
	hit := found.Hits[0]
	assert.Equal(t, Address(0x8000), hit.PC)
	assert.True(t, hit.Reads > 0)
	assert.True(t, hit.Writes >= hit.Reads)
	assert.NotEmpty(t, hit.Values)
}

// TestCallStackDropsFrameOfPulledReturnAddress checks a jump table routine. It
// pulls its return address and jumps, so no RTS ends its frame. The frame
// stayed on the stack before, and the next RTS removed the frame of the caller
// instead.
func TestCallStackDropsFrameOfPulledReturnAddress(t *testing.T) {
	dir := t.TempDir()
	romPath := filepath.Join(dir, "jumptable.nes")

	cart := cartridge.New()
	copy(cart.PRG, []byte{
		0x20, 0x10, 0x80, // $8000 JSR $8010
		0x4c, 0x03, 0x80, // $8003 JMP $8003
	})
	copy(cart.PRG[0x10:], []byte{
		0x20, 0x20, 0x80, // $8010 JSR $8020
		0x60, // $8013 RTS
	})
	copy(cart.PRG[0x20:], []byte{
		0x68,             // $8020 PLA
		0x68,             // $8021 PLA
		0x4c, 0x30, 0x80, // $8022 JMP $8030
	})
	copy(cart.PRG[0x30:], []byte{
		0x60, // $8030 RTS
	})
	file := createFile(t, romPath)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())

	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: recursionCaller,
	})
	id := created.MachineID

	// JSR, JSR, PLA, PLA, and JMP. Only the frame of the first call is left.
	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey: modeInstructions, machineIDKey: id, instructionsKey: 5})
	stack := callTool[callStackResult](t, client, callStackToolName, map[string]any{machineIDKey: id})
	assert.Len(t, stack.Frames, 1)
	assert.Equal(t, Address(recursionAfterCall), stack.Frames[0].ReturnAddress)

	// The RTS at $8030 returns to the first caller and ends its frame.
	stepped := callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey: modeInstructions, machineIDKey: id, instructionsKey: 1})
	assert.Equal(t, Address(recursionAfterCall), stepped.After.PC)
	stack = callTool[callStackResult](t, client, callStackToolName, map[string]any{machineIDKey: id})
	assert.Empty(t, stack.Frames)
}

func TestWatchWritesLimitStopsTheRun(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	limited := callTool[watchWritesResult](t, client, watchWritesToolName, map[string]any{
		machineIDKey: id, addressKey: 0x10, framesKey: 1000, limitKey: 5,
	})
	assert.Len(t, limited.Writes, 5)
	assert.True(t, limited.Truncated)
	assert.Equal(t, "event_limit", limited.Run.Reason)
	assert.True(t, limited.Run.CompletedFrames < 1000)

	complete := callTool[watchWritesResult](t, client, watchWritesToolName, map[string]any{
		machineIDKey: id, addressKey: 0x10, framesKey: 1, limitKey: 10000,
	})
	assert.NotEmpty(t, complete.Writes)
	assert.False(t, complete.Truncated)
	assert.Equal(t, "frames", complete.Run.Reason)
	assert.Equal(t, uint64(1), complete.Run.CompletedFrames)

	assert.Contains(t, callError(t, client, watchWritesToolName, map[string]any{
		machineIDKey: id, addressKey: 0x10, framesKey: 1, limitKey: 0,
	}), "outside 1 to")
	assert.Contains(t, callError(t, client, watchWritesToolName, map[string]any{
		machineIDKey: id, addressKey: 0x20, "end": 0x10, framesKey: 1,
	}), "below start")
	assert.Contains(t, callError(t, client, findAccessorsToolName, map[string]any{
		machineIDKey: id, addressKey: 0x20, "end": 0x10, framesKey: 1,
	}), "below start")
}

func TestAnalysisRunsReportTimeouts(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	// The byte counter can return to its initial value before the deadline.
	diff := callTool[ramDiffResult](t, client, ramDiffToolName, map[string]any{
		machineIDKey: id, framesKey: 100_000_000, timeoutKey: shortTimeout,
	})
	assert.True(t, diff.Run.TimedOut)
	assert.Equal(t, "timeout", diff.Run.Reason)

	found := callTool[findAccessorsResult](t, client, findAccessorsToolName, map[string]any{
		machineIDKey: id, addressKey: 0x10, framesKey: 100_000_000, timeoutKey: shortTimeout,
	})
	assert.True(t, found.Run.TimedOut)

	watched := callTool[watchWritesResult](t, client, watchWritesToolName, map[string]any{
		machineIDKey: id, addressKey: 0x2000, framesKey: 100_000_000, timeoutKey: shortTimeout,
	})
	assert.True(t, watched.Run.TimedOut)
	assert.Equal(t, "timeout", watched.Run.Reason)

	started := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{machineIDKey: id, actionKey: searchStart})
	search := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		machineIDKey: id, searchIDKey: started.SearchID, actionKey: searchFilter, fieldOp: searchChanged, framesKey: 100_000_000, timeoutKey: shortTimeout,
	})
	assert.NotNil(t, search.Run)
	assert.True(t, search.Run.TimedOut)
}

// TestRunContinuesPastTheBreakpointItStoppedAt checks that a machine stopped at
// an exec breakpoint can continue. Every run stopped at the same breakpoint
// without an instruction before.
func TestRunContinuesPastTheBreakpointItStoppedAt(t *testing.T) {
	type stopResult struct {
		After        cpuStateResult `json:"after"`
		CPU          cpuStateResult `json:"cpu"`
		Instructions uint64         `json:"instructions"`
		Reason       string         `json:"reason"`
	}
	for _, tool := range []string{runUntilToolName, stepToolName} {
		t.Run(tool, func(t *testing.T) {
			client, id := recursionMachine(t)
			callTool[addBreakpointResult](t, client, addBreakpointToolName, map[string]any{
				machineIDKey: id, kindKey: execKind, addressKey: recursionInnerCall,
			})
			args := map[string]any{machineIDKey: id}
			if tool == runUntilToolName {
				args[instructionsKey] = 1000
			} else {
				args["mode"] = modeOver
			}
			first := callTool[stopResult](t, client, tool, args)
			assert.Equal(t, "breakpoint", first.Reason)
			assert.Equal(t, uint64(0), first.Instructions)
			second := callTool[stopResult](t, client, tool, args)
			assert.Equal(t, "breakpoint", second.Reason)
			if tool == runUntilToolName {
				assert.Equal(t, Address(recursionInnerCall), second.CPU.PC)
				assert.True(t, second.Instructions > 0)
			} else {
				assert.Equal(t, Address(recursionInnerCall), second.After.PC)
				assert.True(t, second.After.SP < first.After.SP, "the recursion went deeper")
			}
		})
	}
}

func TestCallStackSurvivesRestoreAndFork(t *testing.T) {
	client, id := recursionMachine(t)
	want := callTool[callStackResult](t, client, callStackToolName, map[string]any{machineIDKey: id})
	assert.Len(t, want.Frames, 1)
	checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{machineIDKey: id})

	fork := callTool[createMachineResult](t, client, forkMachineToolName, map[string]any{machineIDKey: id})
	forked := callTool[callStackResult](t, client, callStackToolName, map[string]any{machineIDKey: fork.MachineID})
	assert.Equal(t, want, forked)

	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey: modeOver, machineIDKey: id})
	callTool[restoreResult](t, client, restoreToolName, map[string]any{
		machineIDKey: id, checkpointIDKey: checkpoint.CheckpointID,
	})
	restored := callTool[callStackResult](t, client, callStackToolName, map[string]any{machineIDKey: id})
	assert.Equal(t, want, restored)

	// Step mode out operates on the restored machine.
	run := callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey: modeOut, machineIDKey: id})
	assert.Equal(t, "returned", run.Reason)
}

func TestAccessorReportsExecutionBank(t *testing.T) {
	root := t.TempDir()
	rom := filepath.Join(root, "banked.nes")
	cart := cartridge.New()
	cart.Mapper = 2
	cart.PRG = make([]byte, 0xc000)
	copy(cart.PRG, []byte{0xad, 0x10, 0x00, 0xa9, 0x01, 0x8d, 0x00, 0x80})
	copy(cart.PRG[0x4008:], []byte{0x4c, 0x00, 0xc0})
	copy(cart.PRG[0x8000:], []byte{0x4c, 0x00, 0xc0})
	file := createFile(t, rom)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())
	machine, _, err := loadMachine(t.Context(), nil, rom, 0x8000, false, nil, nil)
	assert.NoError(t, err)
	bank, offset, ok := machine.sys.PRGLocation(0x8000)
	assert.True(t, ok)
	assert.Equal(t, 0, bank)
	assert.Equal(t, 0, offset)
	hits, _, err := machine.findAccessors(t.Context(), 0x10, 0x10, 1, time.Second)
	assert.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Equal(t, 0, hits[0].Bank)
	assert.Equal(t, 0, hits[0].PRGOffset)
}

func TestStepRejectsInvalidModeArgumentsBeforeExecution(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	before := machine.cpuState()
	for _, arguments := range []map[string]any{
		{},
		{
			modeKey: "skip",
		},
		{
			modeKey:         "over",
			instructionsKey: 2,
		},
		{
			modeKey:         "out",
			instructionsKey: 1,
		},
	} {
		arguments[machineIDKey] = created.MachineID
		result, err := client.CallTool(t.Context(), stepToolName, arguments)
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Equal(t, before, machine.cpuState())
	}
}

func TestStepModesPreserveBreakpointBehavior(t *testing.T) {
	for _, mode := range []stepMode{modeInstructions, modeOver, modeOut} {
		t.Run(string(mode), func(t *testing.T) {
			client, id := recursionMachine(t)
			callTool[addBreakpointResult](t, client, addBreakpointToolName, map[string]any{
				machineIDKey: id,
				addressKey:   recursionFunction,
			})
			result := callTool[stepResultOut](t, client, stepToolName, map[string]any{
				machineIDKey: id,
				modeKey:      mode,
			})
			assert.Equal(t, Address(recursionFunction), result.After.PC)
			assert.Equal(t, mode != modeInstructions, result.HasBreakpoint)
			assert.Equal(t, result.After.Cycles-result.Before.Cycles, result.Cycles)
		})
	}
}

// writeRecursionROM writes a cartridge with a function that calls itself until
// the depth counter in $20 reaches three.
func writeRecursionROM(t *testing.T, path string) {
	t.Helper()

	cart := cartridge.New()
	copy(cart.PRG, []byte{
		0x20, 0x10, 0x80, // $8000 JSR $8010
		0x4c, 0x03, 0x80, // $8003 JMP $8003
	})
	copy(cart.PRG[0x10:], []byte{
		0xe6, 0x20, // $8010 INC $20
		0xa5, 0x20, // $8012 LDA $20
		0xc9, 0x03, // $8014 CMP #$03
		0xb0, 0x03, // $8016 BCS $801B
		0x20, 0x10, 0x80, // $8018 JSR $8010
		0x60, // $801B RTS
	})

	file := createFile(t, path)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())
}

// recursionMachine creates a machine over the recursion ROM and steps it to the
// first recursive call at $8018.
func recursionMachine(t *testing.T) (*mcptest.Client, string) {
	t.Helper()

	dir := t.TempDir()
	romPath := filepath.Join(dir, "recursion.nes")
	writeRecursionROM(t, romPath)

	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	t.Cleanup(client.Close)

	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: romPath, entrypointKey: recursionCaller,
	})
	stepped := callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey:      modeInstructions,
		machineIDKey: created.MachineID, instructionsKey: 5,
	})
	assert.Equal(t, Address(recursionInnerCall), stepped.After.PC)
	return client, created.MachineID
}
