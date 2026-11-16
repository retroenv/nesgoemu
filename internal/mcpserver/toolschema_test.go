package mcpserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp"
	"github.com/retroenv/retrogolib/set"
)

// Registration errors must name the tool once and retain the cause.
func TestRegistrationErrorsKeepCauseAndOneToolName(t *testing.T) {
	server := newTestServer(t, Options{})
	for _, tool := range []struct {
		name     string
		register func() error
	}{
		{
			name:     createMachineToolName,
			register: server.registerCreateMachine,
		},
		{
			name:     screenshotToolName,
			register: server.registerScreenshot,
		},
	} {
		t.Run(tool.name, func(t *testing.T) {
			err := tool.register()
			assert.ErrorIs(t, err, mcp.ErrDuplicateTool)
			assert.Equal(t, "registering "+tool.name+": "+mcp.ErrDuplicateTool.Error(), err.Error())
		})
	}
}

func TestStructuredToolsPublishValidSchemas(t *testing.T) {
	_, client, rom := counterServer(t)
	tools, err := client.ListTools(t.Context())
	assert.NoError(t, err)
	images := set.NewFromSlice([]string{screenshotToolName, nametablesToolName, patternTablesToolName})
	for _, tool := range tools {
		_, err := mcp.CompileSchema(tool.InputSchema)
		assert.NoError(t, err, "input schema of %s", tool.Name)
		if images.Contains(tool.Name) {
			continue
		}
		assert.NotEmpty(t, tool.OutputSchema, "output schema of %s", tool.Name)
		_, err = mcp.CompileSchema(tool.OutputSchema)
		assert.NoError(t, err, "output schema of %s", tool.Name)
	}
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	for _, name := range []string{inspectMachineToolName, getTraceToolName, eventLogToolName, listBreakpointsToolName, callStackToolName, coverageToolName} {
		result, err := client.CallTool(t.Context(), name, map[string]any{machineIDKey: created.MachineID})
		assert.NoError(t, err)
		assert.False(t, result.IsError)
		data, err := json.Marshal(result.StructuredContent)
		assert.NoError(t, err)
		for _, tool := range tools {
			if tool.Name == name {
				schema, err := mcp.CompileSchema(tool.OutputSchema)
				assert.NoError(t, err)
				assert.NoError(t, schema.Validate(data), "result of %s", name)
			}
		}
	}
}

func TestQueuedToolDeadlineIncludesWait(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	release, err := machine.acquireRequest(t.Context())
	assert.NoError(t, err)
	defer release()

	result, err := client.CallTool(t.Context(), setButtonsToolName, map[string]any{
		machineIDKey: created.MachineID,
		buttonsKey:   []string{"A"},
		timeoutKey:   shortTimeout,
	})
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "deadline exceeded")
	assert.Empty(t, machine.events)
}

func TestMachineQueueHonorsCancellation(t *testing.T) {
	server, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	machine, err := server.registry.get(created.MachineID)
	assert.NoError(t, err)
	release, err := machine.acquireRequest(t.Context())
	assert.NoError(t, err)
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := machine.acquireRequest(ctx)
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("cancelled request did not leave the machine queue")
	}
	_, err = machine.setButtons(ctx, 1, 0, 0, time.Second)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, machine.events)
}

func TestTypedToolRejectsMissingAndNullAddress(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	for _, args := range []map[string]any{
		{
			machineIDKey: created.MachineID,
			spaceKey:     spaceCPU,
			bytesKey:     []int{90},
		},
		{
			machineIDKey: created.MachineID,
			spaceKey:     spaceCPU,
			bytesKey:     []int{90},
			addressKey:   nil,
		},
	} {
		result, err := client.CallTool(t.Context(), writeMemoryToolName, args)
		assert.NoError(t, err)
		assert.True(t, result.IsError)
	}
}

func TestImageToolsValidateClosedSchemas(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{romKey: rom})
	for _, tool := range []string{screenshotToolName, nametablesToolName, patternTablesToolName} {
		t.Run(tool, func(t *testing.T) {
			result, err := client.CallTool(t.Context(), tool, map[string]any{machineIDKey: created.MachineID, "unrecognized": true})
			assert.NoError(t, err)
			assert.True(t, result.IsError)
		})
	}
	for _, arguments := range []map[string]any{
		{
			machineIDKey: created.MachineID,
			scaleKey:     nil,
		},
		{
			machineIDKey: created.MachineID,
			scaleKey:     "large",
		},
		{},
	} {
		result, err := client.CallTool(t.Context(), screenshotToolName, arguments)
		assert.NoError(t, err)
		assert.True(t, result.IsError)
	}
}

func TestGroupedCatalogHas37Tools(t *testing.T) {
	_, client, _ := counterServer(t)
	tools, err := client.ListTools(t.Context())
	assert.NoError(t, err)
	assert.Len(t, tools, 37)
	removed := set.NewFromSlice([]string{
		"machine_info", "cpu_state", "ppu_state", "apu_state", "bank_state", "palette", "oam",
		"step_over", "step_out", "cdl_start", "cdl_stop",
	})
	for _, tool := range tools {
		assert.False(t, removed.Contains(tool.Name), "old tool %s is still advertised", tool.Name)
	}
}
