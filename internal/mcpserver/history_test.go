package mcpserver

import (
	"encoding/json"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/retroenv/retrogolib/assert"
)

func TestHistoryLimitRejectsWritesBeforeMutation(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	machine.events = make([]machineEvent, 65_536)
	before := machine.readMemory(spaceRAM, 0x10, 1)
	message := callError(t, client, writeMemoryToolName, map[string]any{
		addressKey:   0x10,
		bytesKey:     []int{77},
		machineIDKey: created.MachineID,
		spaceKey:     spaceRAM,
	})
	assert.Contains(t, message, "event history limit")
	assert.Equal(t, before, machine.readMemory(spaceRAM, 0x10, 1))
	assert.Len(t, machine.events, 65_536)
}

func TestHistoryBudgetRejectsCompleteCallsBeforeMutation(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	before := machine.cpuState()
	machine.historyBytes = maxHistoryBytes - eventMetadataBytes
	inputs := []map[string]any{
		{
			buttonsKey: []string{"A"},
			frameKey:   0,
		},
		{
			buttonsKey: []string{"B"},
			frameKey:   1,
		},
	}
	for _, tool := range []string{runFramesToolName, runUntilToolName, ramDiffToolName} {
		callError(t, client, tool, map[string]any{
			framesKey:    2,
			inputsKey:    inputs,
			machineIDKey: created.MachineID,
		})
		assert.Equal(t, before, machine.cpuState())
		assert.Empty(t, machine.events)
	}
	callError(t, client, setButtonsToolName, map[string]any{
		buttonsKey:    []string{"A"},
		"hold_frames": 1,
		machineIDKey:  created.MachineID,
	})
	assert.Equal(t, before, machine.cpuState())
	assert.Empty(t, machine.events)

	// One event fits exactly. The next reset must leave CPU and history intact.
	callTool[setButtonsResult](t, client, setButtonsToolName, map[string]any{
		buttonsKey:   []string{"A"},
		machineIDKey: created.MachineID,
	})
	assert.Equal(t, maxHistoryBytes, machine.historyBytes)
	assert.Len(t, machine.events, 1)
	callError(t, client, resetToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	assert.Equal(t, before, machine.cpuState())
	assert.Len(t, machine.events, 1)
}

func TestCheckpointLimitRejectsBeforeAllocatingHandleOrWriting(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	machine.checkpoints = make([]checkpointRecord, maxCheckpoints-1)
	callTool[checkpointResult](t, client, checkpointToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	message := callError(t, client, checkpointToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	assert.Contains(t, message, "checkpoint limit")
	assert.Len(t, machine.checkpoints, maxCheckpoints)
}

func TestEventMetadataBudgetCoversLargestValues(t *testing.T) {
	address := Address(math.MaxUint16)
	controller := 2
	event := machineEvent{
		Cycles: math.MaxUint64,
		Frame:  math.MaxUint64,
		Kind:   eventInput,

		Buttons:    slices.Repeat([]string{"select"}, 8),
		Controller: &controller,

		Address: &address,
		Bytes:   make([]int, maxWriteLength),
	}
	for index := range event.Bytes {
		event.Bytes[index] = math.MaxUint8
	}

	data, err := json.Marshal(event)
	assert.NoError(t, err)
	assert.True(t, len(data) <= eventMetadataBytes+eventValueBytes*len(event.Bytes))
}

func TestEventLogFileSizeBoundary(t *testing.T) {
	logFile := eventLogFile{
		Version: eventLogVersion,
	}
	data, err := json.Marshal(logFile)
	assert.NoError(t, err)
	encoded, err := encodeEventLog(logFile, len(data))
	assert.NoError(t, err)
	assert.Equal(t, data, encoded)
	_, err = encodeEventLog(logFile, len(data)-1)
	assert.ErrorContains(t, err, "file limit")
}

func TestImportedHistoryLimitsPrecedeROMAccess(t *testing.T) {
	logFile := eventLogFile{
		Events: make([]machineEvent, maxHistoryEvents+1),
	}
	_, _, err := loadRestoredMachine(t.Context(), nil, "missing.nes", logFile, time.Second)
	assert.ErrorContains(t, err, "event history limit")
}

func TestRestoreAndHardResetReleaseHistoryBudget(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	callTool[writeMemoryResult](t, client, writeMemoryToolName, map[string]any{
		addressKey:   0x10,
		bytesKey:     []int{77},
		machineIDKey: created.MachineID,
		spaceKey:     spaceRAM,
	})
	assert.Equal(t, eventMetadataBytes+eventValueBytes, machine.historyBytes)
	fork := callTool[createMachineResult](t, client, forkMachineToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	forked, err := server.registry.get(fork.MachineID)
	assert.NoError(t, err)
	assert.Equal(t, machine.historyBytes, forked.historyBytes)
	callTool[restoreResult](t, client, restoreToolName, map[string]any{
		checkpointIDKey: checkpoint.CheckpointID,
		machineIDKey:    created.MachineID,
	})
	assert.Equal(t, 0, machine.historyBytes)
	assert.Empty(t, machine.events)
	callTool[resetResult](t, client, resetToolName, map[string]any{
		hardKey:      true,
		machineIDKey: fork.MachineID,
	})
	assert.Equal(t, 0, forked.historyBytes)
	assert.Empty(t, forked.events)
	assert.Empty(t, forked.checkpoints)
}

func TestWriteLengthBoundaryAndSchema(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	valid := callTool[writeMemoryResult](t, client, writeMemoryToolName, map[string]any{
		addressKey:   0,
		bytesKey:     make([]int, 4096),
		machineIDKey: created.MachineID,
		spaceKey:     spaceRAM,
	})
	assert.Equal(t, 4096, valid.Written)
	before := machine.readMemory(spaceRAM, 0, 1)
	callError(t, client, writeMemoryToolName, map[string]any{
		addressKey:   0,
		bytesKey:     make([]int, 4097),
		machineIDKey: created.MachineID,
		spaceKey:     spaceRAM,
	})
	assert.Len(t, machine.events, 1)
	assert.Equal(t, before, machine.readMemory(spaceRAM, 0, 1))
}

func TestImportedHistoryBudgetAndPayloadValidation(t *testing.T) {
	address := Address(0)
	events := slices.Repeat([]machineEvent{{
		Kind: eventPoke,

		Address: &address,
		Bytes:   make([]int, maxWriteLength),
	}}, maxHistoryBytes/(eventMetadataBytes+maxWriteLength*eventValueBytes)+1)
	_, err := historyBytes(events)
	assert.ErrorContains(t, err, "byte limit")
	_, err = historyBytes([]machineEvent{{
		Kind: eventInput,

		Buttons: []string{" A "},
	}})
	assert.ErrorContains(t, err, "unknown event button")
}
