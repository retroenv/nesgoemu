package mcpserver

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp"
)

const searchIDKey = "search_id"

func TestMemorySearchKeepsIndependentBaselines(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	var first struct {
		SearchID string `json:"search_id"`
	}
	err := client.CallToolInto(t.Context(), memorySearchToolName, map[string]any{
		actionKey:    searchStart,
		machineIDKey: created.MachineID,
	}, &first)
	assert.NoError(t, err)
	assert.NotEmpty(t, first.SearchID)
	callTool[writeMemoryResult](t, client, writeMemoryToolName, map[string]any{
		addressKey:   0x10,
		bytesKey:     []int{77},
		machineIDKey: created.MachineID,
		spaceKey:     spaceRAM,
	})
	var second struct {
		SearchID string `json:"search_id"`
	}
	err = client.CallToolInto(t.Context(), memorySearchToolName, map[string]any{
		actionKey:    searchStart,
		machineIDKey: created.MachineID,
	}, &second)
	assert.NoError(t, err)
	assert.NotEqual(t, first.SearchID, second.SearchID)

	// A second search must not replace the first search's baseline.
	changed := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		actionKey:    searchFilter,
		machineIDKey: created.MachineID,
		fieldOp:      "changed",
		searchIDKey:  first.SearchID,
	})
	assert.Equal(t, 1, changed.Candidates)
	unchanged := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		actionKey:    searchFilter,
		machineIDKey: created.MachineID,
		fieldOp:      "changed",
		searchIDKey:  second.SearchID,
	})
	assert.Equal(t, 0, unchanged.Candidates)
}

func TestMemorySearchHandlesHaveLimitsAndCleanup(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	var first string
	for index := range maxMemorySearches {
		started := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
			actionKey:    searchStart,
			machineIDKey: created.MachineID,
		})
		if index == 0 {
			first = started.SearchID
		}
	}

	message := callError(t, client, memorySearchToolName, map[string]any{
		actionKey:    searchStart,
		machineIDKey: created.MachineID,
	})
	assert.Contains(t, message, "search limit")
	listed := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		actionKey:    searchList,
		machineIDKey: created.MachineID,
		searchIDKey:  first,
	})
	assert.Equal(t, first, listed.SearchID)
	assert.Equal(t, 2048, listed.Candidates)
	released := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		actionKey:    searchRelease,
		machineIDKey: created.MachineID,
		searchIDKey:  first,
	})
	assert.True(t, released.Released)
	callError(t, client, memorySearchToolName, map[string]any{
		actionKey:    searchList,
		machineIDKey: created.MachineID,
		searchIDKey:  first,
	})
	newSearch := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		actionKey:    searchStart,
		machineIDKey: created.MachineID,
	})
	assert.NotEqual(t, first, newSearch.SearchID)
}

func TestMemorySearchRejectsInvalidHandlesAndFieldsBeforeRun(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	started := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		actionKey:    searchStart,
		machineIDKey: created.MachineID,
	})
	before := machine.cpuState()
	for _, arguments := range invalidMemorySearchRequests(started.SearchID) {
		arguments[machineIDKey] = created.MachineID
		callError(t, client, memorySearchToolName, arguments)
		assert.Equal(t, before, machine.cpuState())
	}
}

// invalidMemorySearchRequests returns memory search calls that must fail and keep the machine state.
func invalidMemorySearchRequests(searchID string) []map[string]any {
	return []map[string]any{
		{
			actionKey: searchFilter,
			fieldOp:   searchChanged,
			framesKey: 1,
		},
		{
			actionKey:   searchFilter,
			fieldOp:     searchChanged,
			framesKey:   1,
			searchIDKey: "missing",
		},
		{
			actionKey:   searchFilter,
			fieldOp:     searchValue,
			searchIDKey: searchID,
		},
		{
			actionKey:   searchFilter,
			fieldOp:     searchValue,
			fieldValue:  256,
			framesKey:   1,
			searchIDKey: searchID,
		},
		{
			actionKey:   searchFilter,
			fieldOp:     searchChanged,
			fieldValue:  1,
			searchIDKey: searchID,
		},
		{
			actionKey:   searchList,
			framesKey:   1,
			searchIDKey: searchID,
		},
		{
			actionKey:   searchStart,
			searchIDKey: searchID,
		},
		{
			actionKey:   searchRelease,
			framesKey:   1,
			searchIDKey: searchID,
		},
		{
			actionKey:   searchList,
			searchIDKey: nil,
		},
	}
}

func TestMemorySearchHandlesStayWithMachineAndTimeline(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey: rom,
	})
	started := callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		actionKey:    searchStart,
		machineIDKey: created.MachineID,
	})
	fork := callTool[createMachineResult](t, client, forkMachineToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	callError(t, client, memorySearchToolName, map[string]any{
		actionKey:    searchList,
		machineIDKey: fork.MachineID,
		searchIDKey:  started.SearchID,
	})
	checkpoint := callTool[checkpointResult](t, client, checkpointToolName, map[string]any{
		machineIDKey: created.MachineID,
	})
	callTool[restoreResult](t, client, restoreToolName, map[string]any{
		checkpointIDKey: checkpoint.CheckpointID,
		machineIDKey:    created.MachineID,
	})
	callError(t, client, memorySearchToolName, map[string]any{
		actionKey:    searchList,
		machineIDKey: created.MachineID,
		searchIDKey:  started.SearchID,
	})
	started = callTool[memorySearchResult](t, client, memorySearchToolName, map[string]any{
		actionKey:    searchStart,
		machineIDKey: created.MachineID,
	})
	callTool[resetResult](t, client, resetToolName, map[string]any{
		hardKey:      true,
		machineIDKey: created.MachineID,
	})
	callError(t, client, memorySearchToolName, map[string]any{
		actionKey:    searchList,
		machineIDKey: created.MachineID,
		searchIDKey:  started.SearchID,
	})
}

func TestInactiveSearchKeepsMachineState(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	before := machine.cpuState()
	_, err = server.memorySearch(t.Context(), mcp.ToolRequest{}, memorySearchArgs{
		Action:    searchFilter,
		MachineID: created.MachineID,

		Op: searchChanged,

		Frames: 1,
	})
	assert.Error(t, err)
	after := machine.cpuState()
	assert.Equal(t, before.Cycles, after.Cycles)
}
