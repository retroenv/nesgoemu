package mcpserver

import (
	"path/filepath"
	"testing"

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
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	t.Cleanup(client.Close)

	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: recursionCaller,
	})
	stepped := callTool[stepResultOut](t, client, "step", map[string]any{
		machineIDKey: created.MachineID, instructionsKey: 5,
	})
	assert.Equal(t, Address(recursionInnerCall), stepped.After.PC)
	return client, created.MachineID
}

func TestStepOverSkipsDeeperReturnToTheSameAddress(t *testing.T) {
	client, id := recursionMachine(t)
	before := callTool[cpuStateResult](t, client, "cpu_state", map[string]any{machineIDKey: id})

	run := callTool[runUntilResult](t, client, "step_over", map[string]any{machineIDKey: id})

	// The inner calls return to $801B on deeper stack levels first. The step
	// ends only when the stack pointer is back at its level before the call.
	assert.Equal(t, "returned", run.Reason)
	assert.Equal(t, Address(recursionInnerAfter), run.CPU.PC)
	assert.Equal(t, before.SP, run.CPU.SP)
}

func TestStepOverStopsAtUserBreakpoint(t *testing.T) {
	client, id := recursionMachine(t)

	breakpoint := callTool[addBreakpointResult](t, client, "add_breakpoint", map[string]any{
		machineIDKey: id, kindKey: execKind, addressKey: recursionFunction,
	})
	run := callTool[runUntilResult](t, client, "step_over", map[string]any{machineIDKey: id})

	assert.Equal(t, "breakpoint", run.Reason)
	assert.Equal(t, breakpoint.ID, run.BreakpointID)
	assert.Equal(t, Address(recursionFunction), run.CPU.PC)

	listed := callTool[listBreakpointsResult](t, client, "list_breakpoints", map[string]any{machineIDKey: id})
	assert.Len(t, listed.Breakpoints, 1)
}

func TestStepOutReturnsToTheCaller(t *testing.T) {
	client, id := recursionMachine(t)

	stack := callTool[callStackResult](t, client, "call_stack", map[string]any{machineIDKey: id})
	assert.Len(t, stack.Frames, 1)
	assert.Equal(t, Address(recursionAfterCall), stack.Frames[0].ReturnAddress)
	assert.Equal(t, Address(recursionCaller), stack.Frames[0].CallerAddress)
	assert.Equal(t, 0, stack.Frames[0].CallerOffset)

	run := callTool[runUntilResult](t, client, "step_out", map[string]any{machineIDKey: id})

	assert.Equal(t, "returned", run.Reason)
	assert.Equal(t, Address(recursionAfterCall), run.CPU.PC)
}

func TestSoftResetClearsCallStack(t *testing.T) {
	client, id := recursionMachine(t)

	callTool[resetResult](t, client, "reset", map[string]any{machineIDKey: id})

	stack := callTool[callStackResult](t, client, "call_stack", map[string]any{machineIDKey: id})
	assert.Empty(t, stack.Frames)
}

// TestRunUntilTraceTailSurvivesFullRing checks that the trace tail still lists
// the new lines when the ring is full. A tail that counted the stored lines
// returned nothing once the ring reached its capacity.
func TestRunUntilTraceTailSurvivesFullRing(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	started := callTool[traceState](t, client, "trace_start", map[string]any{machineIDKey: id})
	assert.True(t, started.Enabled)
	callTool[runUntilResult](t, client, "run_until", map[string]any{
		machineIDKey: id, instructionsKey: started.Capacity + 100,
	})
	full := callTool[traceState](t, client, "trace_stop", map[string]any{machineIDKey: id})
	assert.Equal(t, full.Capacity, full.Lines)

	callTool[traceState](t, client, "trace_start", map[string]any{machineIDKey: id})
	callTool[runUntilResult](t, client, "run_until", map[string]any{
		machineIDKey: id, instructionsKey: started.Capacity + 100,
	})
	run := callTool[runUntilResult](t, client, "run_until", map[string]any{
		machineIDKey: id, instructionsKey: 5,
	})

	assert.Len(t, run.Trace, 5)
}

func TestRunUntilTraceTailIsEmptyWhileTracingIsOff(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})

	run := callTool[runUntilResult](t, client, "run_until", map[string]any{
		machineIDKey: created.MachineID, instructionsKey: 5,
	})

	assert.Empty(t, run.Trace)
}

func TestRunUntilReportsInstructionOfMemoryBreakpoint(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	callTool[addBreakpointResult](t, client, "add_breakpoint", map[string]any{
		machineIDKey: id, kindKey: "write", addressKey: 0x10,
	})
	run := callTool[runUntilResult](t, client, "run_until", map[string]any{machineIDKey: id})

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
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})
	id := created.MachineID

	callTool[resetResult](t, client, "reset", map[string]any{machineIDKey: id, "hard": true})
	callTool[cdlState](t, client, "cdl_start", map[string]any{machineIDKey: id})
	callTool[stepResultOut](t, client, "step", map[string]any{machineIDKey: id, instructionsKey: 4})

	coverage := callTool[coverageResult](t, client, "coverage", map[string]any{machineIDKey: id})
	assert.NotEmpty(t, coverage.Banks)
	assert.True(t, coverage.Banks[0].Code > 0)
}

func TestFindAccessorsSeparatesReadsWritesAndValues(t *testing.T) {
	_, client, romPath := counterServer(t)
	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: 0x8000,
	})

	found := callTool[findAccessorsResult](t, client, "find_accessors", map[string]any{
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
		Name:  testServerName,
		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	defer client.Close()

	created := callTool[createMachineResult](t, client, "create_machine", map[string]any{
		romKey: romPath, entrypointKey: recursionCaller,
	})
	id := created.MachineID

	// JSR, JSR, PLA, PLA, and JMP. Only the frame of the first call is left.
	callTool[stepResultOut](t, client, "step", map[string]any{machineIDKey: id, instructionsKey: 5})
	stack := callTool[callStackResult](t, client, "call_stack", map[string]any{machineIDKey: id})
	assert.Len(t, stack.Frames, 1)
	assert.Equal(t, Address(recursionAfterCall), stack.Frames[0].ReturnAddress)

	// The RTS at $8030 returns to the first caller and ends its frame.
	stepped := callTool[stepResultOut](t, client, "step", map[string]any{machineIDKey: id, instructionsKey: 1})
	assert.Equal(t, Address(recursionAfterCall), stepped.After.PC)
	stack = callTool[callStackResult](t, client, "call_stack", map[string]any{machineIDKey: id})
	assert.Empty(t, stack.Frames)
}
