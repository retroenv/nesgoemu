package mcpserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp"
	"github.com/retroenv/retrogolib/mcp/mcptest"
)

// machineSnapshot is the comparable state of a machine at a frame boundary.
type machineSnapshot struct {
	CPU        cpuStateResult
	RAM        []int
	Screenshot [sha256.Size]byte
}

// TestReplayReproducesMixedRun checks that fork, restore, and restore_machine
// rebuild the exact machine state of a run that mixes every recorded event.
// Bugs that this covers: button releases were not recorded, run_until inputs
// were recorded at relative frames, a reset was recorded after it happened, and
// restore kept the events of the discarded timeline.
func TestReplayReproducesMixedRun(t *testing.T) {
	client, romPath, _ := controllerServer(t)

	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	id := created.MachineID
	runMixedScenario(t, client, id)

	want := snapshotMachine(t, client, id)
	assert.True(t, want.RAM[0x10] > 0)
	assert.Equal(t, 0x55, want.RAM[0x200])

	checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{machineIDKey: id})
	eventCount := callTool[eventLogResult](t, client, eventLogToolName, map[string]any{machineIDKey: id}).Total

	fork := callTool[createMachineResult](t, client, forkMachineToolName, map[string]any{machineIDKey: id})
	assert.Equal(t, want, snapshotMachine(t, client, fork.MachineID))
	forkAtCheckpoint := callTool[createMachineResult](t, client, forkMachineToolName, map[string]any{
		machineIDKey:    id,
		checkpointIDKey: checkpoint.CheckpointID,
	})
	assert.Equal(t, want, snapshotMachine(t, client, forkAtCheckpoint.MachineID))

	// Run on, then restore. The events after the checkpoint belong to the
	// discarded timeline and must not return in a later restore.
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    5,
		inputsKey:    []map[string]any{pressA(0)},
	})
	callTool[restoreResult](t, client, restoreToolName, map[string]any{
		machineIDKey:    id,
		checkpointIDKey: checkpoint.CheckpointID,
	})
	assert.Equal(t, want, snapshotMachine(t, client, id))
	assert.Equal(t, eventCount, callTool[eventLogResult](t, client, eventLogToolName, map[string]any{machineIDKey: id}).Total)

	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    3,
	})
	callTool[restoreResult](t, client, restoreToolName, map[string]any{
		machineIDKey:    id,
		checkpointIDKey: checkpoint.CheckpointID,
	})
	assert.Equal(t, want, snapshotMachine(t, client, id))

	restored := callTool[createMachineResult](t, client, restoreMachineToolName, map[string]any{
		romKey:    romPath,
		eventsKey: checkpoint.EventsFile,
	})
	assert.Equal(t, want, snapshotMachine(t, client, restored.MachineID))
}

// TestEventLogRecordsAbsoluteFramesAndReleases checks the recorded events of
// run_until inputs and of a button release.
func TestEventLogRecordsAbsoluteFramesAndReleases(t *testing.T) {
	client, romPath, _ := controllerServer(t)

	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	id := created.MachineID

	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    5,
	})
	callTool[runUntilResult](t, client, runUntilToolName, map[string]any{
		machineIDKey: id,
		framesKey:    4,
		inputsKey:    []map[string]any{pressA(1), release(3)},
	})

	log := callTool[eventLogResult](t, client, eventLogToolName, map[string]any{machineIDKey: id})
	assert.Len(t, log.Events, 2)
	assert.Equal(t, uint64(6), log.Events[0].Frame)
	assert.Equal(t, []string{"a"}, log.Events[0].Buttons)
	assert.Equal(t, uint64(8), log.Events[1].Frame)
	assert.Empty(t, log.Events[1].Buttons)
	assert.True(t, log.Events[1].Cycles > log.Events[0].Cycles)
}

// TestRestoreMachineRejectsOtherROM checks that the event log stores the ROM
// checksums.
func TestRestoreMachineRejectsOtherROM(t *testing.T) {
	client, romPath, dir := controllerServer(t)

	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        romPath,
		entrypointKey: 0x8000,
	})
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: created.MachineID,
		framesKey:    2,
	})
	checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{
		machineIDKey: created.MachineID,
	})

	data, err := os.ReadFile(checkpoint.EventsFile)
	assert.NoError(t, err)
	var stored eventLogFile
	assert.NoError(t, json.Unmarshal(data, &stored))
	assert.Equal(t, eventLogVersion, stored.Version)
	assert.Equal(t, 0x8000, *stored.Entrypoint)
	assert.Equal(t, uint64(2), stored.Frame)

	otherPath := filepath.Join(dir, "other.nes")
	writeCounterROM(t, otherPath)
	result, err := client.CallTool(t.Context(), restoreMachineToolName, map[string]any{
		romKey:    otherPath,
		eventsKey: checkpoint.EventsFile,
	})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
}

// TestReplayIsExactInsideAFrame checks a checkpoint and a fork that are taken
// between two instructions of a frame. The replay stopped at the frame
// boundary before, so the restored machine was a few instructions behind.
func TestReplayIsExactInsideAFrame(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	id := created.MachineID

	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    3,
	})
	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey:         modeInstructions,
		machineIDKey:    id,
		instructionsKey: 37,
	})
	callTool[writeMemoryResult](t, client, writeMemoryToolName, map[string]any{
		machineIDKey: id,
		spaceKey:     spaceCPU,
		addressKey:   0x0200,
		bytesKey:     []int{0x55},
	})
	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey:         modeInstructions,
		machineIDKey:    id,
		instructionsKey: 5,
	})
	want := snapshotMachine(t, client, id)
	checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{machineIDKey: id})

	fork := callTool[createMachineResult](t, client, forkMachineToolName, map[string]any{machineIDKey: id})
	assert.Equal(t, want, snapshotMachine(t, client, fork.MachineID))

	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    2,
	})
	assert.NotEqual(t, want, snapshotMachine(t, client, id))
	callTool[restoreResult](t, client, restoreToolName, map[string]any{
		machineIDKey:    id,
		checkpointIDKey: checkpoint.CheckpointID,
	})
	assert.Equal(t, want, snapshotMachine(t, client, id))

	restored := callTool[createMachineResult](t, client, restoreMachineToolName, map[string]any{
		romKey:    rom,
		eventsKey: checkpoint.EventsFile,
	})
	assert.Equal(t, want, snapshotMachine(t, client, restored.MachineID))

	// The restored machine continues like the original.
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    2,
	})
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: fork.MachineID,
		framesKey:    2,
	})
	assert.Equal(t, snapshotMachine(t, client, id), snapshotMachine(t, client, fork.MachineID))
}

func TestReplayKeepsInitialBatteryData(t *testing.T) {
	root := t.TempDir()
	rom := filepath.Join(root, "battery.nes")
	cart := cartridge.New()
	cart.Battery = 1
	file := createFile(t, rom)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())
	saved := make([]byte, 8192)
	saved[0] = 0x11
	assert.NoError(t, os.WriteFile(rom+".sav", saved, 0o600))
	machine, _, err := loadMachine(t.Context(), nil, rom, 0x8000, true, nil, nil)
	assert.NoError(t, err)
	before := machine.sys.Bus.Memory.Read(0x6000)
	assert.Equal(t, byte(0x11), before)
	saved[0] = 0x22
	assert.NoError(t, os.WriteFile(rom+".sav", saved, 0o600))
	fork, _, err := machine.forkMachine(t.Context(), nil, replayTarget{}, time.Second)
	assert.NoError(t, err)
	after := fork.sys.Bus.Memory.Read(0x6000)
	assert.Equal(t, before, after)
	server := newTestServer(t, Options{
		Roots: []string{root},
	})
	assert.NoError(t, server.registry.add(machine))
	checkpoint, err := server.checkpoint(t.Context(), mcp.ToolRequest{}, machineIDArgs{
		MachineID: machine.id,
	})
	assert.NoError(t, err)
	assert.NoError(t, os.Remove(rom+".sav"))
	restored, err := server.restoreMachine(t.Context(), mcp.ToolRequest{}, restoreMachineArgs{
		Events: checkpoint.EventsFile,
		ROM:    rom,
	})
	assert.NoError(t, err)
	reloaded, err := server.registry.get(restored.MachineID)
	assert.NoError(t, err)
	assert.Equal(t, before, reloaded.sys.Bus.Memory.Read(0x6000))
	assert.NoError(t, machine.hardReset(t.Context(), nil))
	assert.Equal(t, before, machine.sys.Bus.Memory.Read(0x6000))
}

func TestReplayKeepsAbsentBatterySave(t *testing.T) {
	root := t.TempDir()
	rom := filepath.Join(root, "battery.nes")
	cart := cartridge.New()
	cart.Battery = 1
	file := createFile(t, rom)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())
	machine, _, err := loadMachine(t.Context(), nil, rom, 0x8000, true, nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, machine.batteryData)
	saved := make([]byte, 8192)
	saved[0] = 0x55
	assert.NoError(t, os.WriteFile(rom+".sav", saved, 0o600))
	fork, _, err := machine.forkMachine(t.Context(), nil, replayTarget{}, time.Second)
	assert.NoError(t, err)
	assert.Equal(t, byte(0), fork.sys.Bus.Memory.Read(0x6000))
}

func TestLegacyBatteryLogRequiresInitialSave(t *testing.T) {
	logFile := eventLogFile{
		Battery: true,
	}
	_, _, err := loadRestoredMachine(t.Context(), nil, "unused.nes", logFile, time.Second)
	assert.ErrorContains(t, err, "no initial save data")
}

// The test uses the MCP tools and restores after it deletes the palette file.
// Event logs contain the palette bytes, so a replay shows the same colors.
func TestDisplayPaletteSurvivesReplay(t *testing.T) {
	server, client, rom := counterServer(t)
	path := filepath.Join(filepath.Dir(rom), "test.pal")
	assert.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{200, 160, 120}, 64), 0o600))
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
		paletteKey:    path,
	})
	id := created.MachineID
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    2,
	})
	want := snapshotMachine(t, client, id)
	checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{machineIDKey: id})
	assert.NoError(t, os.Remove(path))

	raw, err := client.CallTool(t.Context(), screenshotToolName, map[string]any{machineIDKey: id})
	assert.NoError(t, err)
	rawImage, err := png.Decode(bytes.NewReader(imageContent(t, raw)))
	assert.NoError(t, err)
	red, green, blue, _ := rawImage.At(0, 0).RGBA()
	assert.Equal(t, uint32(200*257), red)
	assert.Equal(t, uint32(160*257), green)
	assert.Equal(t, uint32(120*257), blue)
	assert.Equal(t, want, snapshotMachine(t, client, id))

	fork := callTool[createMachineResult](t, client, forkMachineToolName, map[string]any{machineIDKey: id})
	assert.Equal(t, want, snapshotMachine(t, client, fork.MachineID))
	restored := callTool[createMachineResult](t, client, restoreMachineToolName, map[string]any{
		romKey:    rom,
		eventsKey: checkpoint.EventsFile,
	})
	assert.Equal(t, want, snapshotMachine(t, client, restored.MachineID))
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    3,
	})
	callTool[restoreResult](t, client, restoreToolName, map[string]any{
		machineIDKey:    id,
		checkpointIDKey: checkpoint.CheckpointID,
	})
	assert.Equal(t, want, snapshotMachine(t, client, id))

	machine, err := server.registry.get(id)
	assert.NoError(t, err)
	assert.NoError(t, machine.hardReset(t.Context(), nil))
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    2,
	})
	assert.Equal(t, want, snapshotMachine(t, client, id))
}

// writeControllerROM writes a cartridge that reads controller one in a loop.
// Zero page $10 counts the loop passes and $11 counts the passes with button A
// pressed.
func writeControllerROM(t *testing.T, path string) {
	t.Helper()

	cart := cartridge.New()
	program := []byte{
		0xa9, 0x01, // LDA #$01
		0x8d, 0x16, 0x40, // STA $4016
		0xa9, 0x00, // LDA #$00
		0x8d, 0x16, 0x40, // STA $4016
		0xad, 0x16, 0x40, // LDA $4016
		0x29, 0x01, // AND #$01
		0xf0, 0x02, // BEQ +2
		0xe6, 0x11, // INC $11
		0xe6, 0x10, // INC $10
		0x4c, 0x00, 0x80, // JMP $8000
	}
	copy(cart.PRG, program)

	file := createFile(t, path)
	assert.NoError(t, cart.Save(file))
	assert.NoError(t, file.Close())
}

// controllerServer creates a server and client over the controller ROM.
func controllerServer(t *testing.T) (*mcptest.Client, string, string) {
	t.Helper()

	dir := t.TempDir()
	romPath := filepath.Join(dir, "controller.nes")
	writeControllerROM(t, romPath)

	server := newTestServer(t, Options{
		Name: testServerName,

		Roots: []string{dir},
	})
	client := mcptest.New(t.Context(), server.mcp)
	t.Cleanup(client.Close)
	return client, romPath, dir
}

// callTool calls a tool and decodes its structured result.
func callTool[T any](t *testing.T, client *mcptest.Client, name string, args map[string]any) T {
	t.Helper()

	var out T
	assert.NoError(t, client.CallToolInto(t.Context(), name, args, &out))
	return out
}

// snapshotMachine captures the CPU, RAM, and screen of a machine.
func snapshotMachine(t *testing.T, client *mcptest.Client, machineID string) machineSnapshot {
	t.Helper()

	memory := callTool[readMemoryResult](t, client, readMemoryToolName, map[string]any{
		machineIDKey: machineID,
		spaceKey:     spaceRAM,
		addressKey:   0,
		lengthKey:    nes.RAMEndAddress + 1,
	})
	result, err := client.CallTool(t.Context(), screenshotToolName, map[string]any{machineIDKey: machineID})
	assert.NoError(t, err)

	return machineSnapshot{
		CPU:        inspectSection[cpuStateResult](t, client, sectionCPU, machineID),
		RAM:        memory.Bytes,
		Screenshot: sha256.Sum256(imageContent(t, result)),
	}
}

// pressA is an input event that presses button A at a relative frame.
func pressA(frame int) map[string]any {
	return map[string]any{
		frameKey:   frame,
		buttonsKey: []string{"A"},
	}
}

// release is an input event that releases every button at a relative frame.
func release(frame int) map[string]any {
	return map[string]any{
		frameKey:   frame,
		buttonsKey: []string{},
	}
}

// runMixedScenario runs inputs, releases, a poke, a hold, a mid-frame step, a
// run_until with inputs, and a soft reset. It ends at a frame boundary.
func runMixedScenario(t *testing.T, client *mcptest.Client, id string) {
	t.Helper()

	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    4,
		inputsKey:    []map[string]any{pressA(1), release(3)},
	})
	callTool[writeMemoryResult](t, client, writeMemoryToolName, map[string]any{
		machineIDKey: id,
		spaceKey:     spaceCPU,
		addressKey:   0x0200,
		bytesKey:     []int{0x55},
	})
	callTool[setButtonsResult](t, client, setButtonsToolName, map[string]any{
		machineIDKey:  id,
		buttonsKey:    []string{"A"},
		"hold_frames": 2,
	})
	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey:         modeInstructions,
		machineIDKey:    id,
		instructionsKey: 37,
	})
	callTool[runUntilResult](t, client, runUntilToolName, map[string]any{
		machineIDKey: id,
		framesKey:    3,
		inputsKey:    []map[string]any{pressA(1), release(2)},
	})
	callTool[resetResult](t, client, resetToolName, map[string]any{machineIDKey: id})
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    3,
		inputsKey:    []map[string]any{pressA(0)},
	})
	callTool[runFramesOut](t, client, runFramesToolName, map[string]any{
		machineIDKey: id,
		framesKey:    2,
	})
}
