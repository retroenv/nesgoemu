package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp"
)

// deadlineTransitionContext ends on its third error check.
type deadlineTransitionContext struct {
	checks int
	done   chan struct{}
}

func (*deadlineTransitionContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (ctx *deadlineTransitionContext) Done() <-chan struct{} {
	return ctx.done
}

func (ctx *deadlineTransitionContext) Err() error {
	ctx.checks++
	if ctx.checks < 3 {
		return nil
	}
	if ctx.checks == 3 {
		close(ctx.done)
	}
	return context.DeadlineExceeded
}

func (*deadlineTransitionContext) Value(any) any {
	return nil
}

// The deadline can expire between error checks in one loop iteration.
// The run must return a timeout result instead of an execution error.
func TestRunFramesReportsDeadlineBetweenChecks(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	ctx := &deadlineTransitionContext{
		done: make(chan struct{}),
	}
	result, err := machine.runFrames(ctx, 1, nil, 0)
	assert.NoError(t, err)
	assert.True(t, result.TimedOut)
	assert.Equal(t, nes.StopTimeout, result.Reason)
}

func TestStepHonorsCancellation(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	want := snapshotMachine(t, client, created.MachineID)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := server.step(ctx, mcp.ToolRequest{}, stepArgs{
		MachineID: created.MachineID,
		Mode:      modeInstructions,
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, want, snapshotMachine(t, client, created.MachineID))
}

func TestSteppingToolsValidateTimeout(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	for _, mode := range []stepMode{modeInstructions, modeOver, modeOut} {
		t.Run(string(mode), func(t *testing.T) {
			assert.Contains(t, callError(t, client, stepToolName, map[string]any{
				modeKey:      mode,
				machineIDKey: created.MachineID,
				timeoutKey:   "-1s",
			}), "negative")
		})
	}
}

func TestStepCancellationDuringRun(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	remove := machine.sys.AddPreExecutionHook(func(_ *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) {
		cancel()
	})
	defer remove()

	result, err := machine.step(ctx, 2)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, uint64(1), result.Instructions)
}

func TestSteppingToolsHonorTimeout(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	want := snapshotMachine(t, client, created.MachineID)
	args := map[string]any{
		machineIDKey: created.MachineID,
		timeoutKey:   "1ns",
	}
	for _, mode := range []stepMode{modeInstructions, modeOver, modeOut} {
		args["mode"] = mode
		assert.Contains(t, callError(t, client, stepToolName, args), "deadline exceeded")
	}
	assert.Equal(t, want, snapshotMachine(t, client, created.MachineID))
}

func TestSingleDebugStepReportsCycles(t *testing.T) {
	for _, mode := range []stepMode{modeOver, modeOut} {
		t.Run(string(mode), func(t *testing.T) {
			_, client, rom := counterServer(t)
			created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
				romKey:        rom,
				entrypointKey: 0x8000,
			})
			run := callTool[stepResultOut](t, client, stepToolName, map[string]any{
				modeKey:      mode,
				machineIDKey: created.MachineID,
			})
			assert.Equal(t, uint64(1), run.Instructions)
			assert.True(t, run.Cycles > 0)
			assert.Equal(t, run.After.Cycles-created.CPU.Cycles, run.Cycles)
		})
	}
}

func TestCheckpointSnapshotMatchesReplayTarget(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	callTool[writeMemoryResult](t, client, writeMemoryToolName, map[string]any{
		machineIDKey: created.MachineID,
		spaceKey:     spaceCPU,
		addressKey:   0,
		bytesKey:     []int{1},
	})
	checkpoint, data, err := machine.checkpoint()
	assert.NoError(t, err)
	id := checkpoint.CheckpointID
	var stored eventLogFile
	assert.NoError(t, json.Unmarshal(data, &stored))
	callTool[writeMemoryResult](t, client, writeMemoryToolName, map[string]any{
		machineIDKey: created.MachineID,
		spaceKey:     spaceCPU,
		addressKey:   0,
		bytesKey:     []int{2},
	})
	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey:      modeInstructions,
		machineIDKey: created.MachineID,
	})
	machine.mu.Lock()
	defer machine.mu.Unlock()
	assert.NotNil(t, machine.checkpoints[id-1].snapshot)
	events, point, err := machine.resolveTarget(replayTarget{
		Checkpoint: &id,
	})
	assert.NoError(t, err)
	assert.Equal(t, stored.Cycles, point.Cycles)
	assert.Equal(t, stored.Frame, point.Frame)
	assert.Equal(t, stored.Events, events)
	assert.Equal(t, created.ROM.SHA256, stored.ROMHash)
}

func TestReplayRejectsChangedROM(t *testing.T) {
	for _, tool := range []string{restoreToolName, forkMachineToolName, resetToolName} {
		for _, offset := range []int{6, 16} {
			t.Run(tool+"/byte"+strconv.Itoa(offset), func(t *testing.T) {
				_, client, rom := counterServer(t)
				created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
					romKey:        rom,
					entrypointKey: 0x8000,
				})
				checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{
					machineIDKey: created.MachineID,
				})
				want := snapshotMachine(t, client, created.MachineID)
				data, err := os.ReadFile(rom)
				assert.NoError(t, err)
				data[offset] ^= 1
				assert.NoError(t, os.WriteFile(rom, data, 0o644))

				args := map[string]any{machineIDKey: created.MachineID}
				switch tool {
				case restoreToolName:
					args[checkpointIDKey] = checkpoint.CheckpointID
				case resetToolName:
					args["hard"] = true
				}
				assert.Contains(t, callError(t, client, tool, args), "ROM has changed")
				assert.Equal(t, want, snapshotMachine(t, client, created.MachineID))
				assert.Contains(t, callError(t, client, restoreMachineToolName, map[string]any{
					romKey:    rom,
					eventsKey: checkpoint.EventsFile,
				}), "does not match")
			})
		}
	}
}

func TestRestoreMachineAcceptsLegacyChecksumLog(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	stored, err := readEventLog(t.Context(), checkpoint.EventsFile)
	assert.NoError(t, err)
	stored.ROMHash = ""
	data, err := json.Marshal(stored)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(checkpoint.EventsFile, data, 0o644))
	restored := callTool[createMachineResult](t, client, restoreMachineToolName, map[string]any{
		romKey:    rom,
		eventsKey: checkpoint.EventsFile,
	})
	assert.Equal(t, snapshotMachine(t, client, created.MachineID), snapshotMachine(t, client, restored.MachineID))
}

func TestRunFramesRejectsOutOfRangeInputs(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	want := snapshotMachine(t, client, created.MachineID)
	for _, frame := range []uint64{1, 2} {
		assert.Contains(t, callError(t, client, runFramesToolName, map[string]any{
			machineIDKey: created.MachineID,
			framesKey:    1,
			inputsKey: []map[string]any{{
				frameKey:   frame,
				buttonsKey: []string{"A"},
			}},
		}), "outside")
		assert.Equal(t, want, snapshotMachine(t, client, created.MachineID))
	}
}

func TestMachineCreationWaitHonorsCancellation(t *testing.T) {
	server := newTestServer(t, Options{})
	assert.NoError(t, server.acquireLoad(t.Context()))
	defer server.releaseLoad()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := server.createMachine(ctx, mcp.ToolRequest{}, createMachineArgs{})
	assert.ErrorIs(t, err, context.Canceled)
	_, err = server.restoreMachine(ctx, mcp.ToolRequest{}, restoreMachineArgs{})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMachineCreationFailureReleasesLoad(t *testing.T) {
	server := newTestServer(t, Options{})
	_, err := server.createMachine(t.Context(), mcp.ToolRequest{}, createMachineArgs{})
	assert.Error(t, err)
	assert.Empty(t, server.loads)
	_, err = server.restoreMachine(t.Context(), mcp.ToolRequest{}, restoreMachineArgs{})
	assert.Error(t, err)
	assert.Empty(t, server.loads)
}

func TestMachineLoadWaitDeadline(t *testing.T) {
	server := newTestServer(t, Options{})
	assert.NoError(t, server.acquireLoad(t.Context()))
	defer server.releaseLoad()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, server.acquireLoad(ctx), context.DeadlineExceeded)
	assert.Len(t, server.loads, 1)
}
