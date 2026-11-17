package mcpserver

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
	"github.com/retroenv/retrogolib/assert"
)

func TestCDLReadMatchesExport(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	id := created.MachineID
	callTool[cdlState](t, client, cdlSetEnabledToolName, map[string]any{
		machineIDKey: id,
		enabledKey:   true,
	})
	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		modeKey:         modeInstructions,
		machineIDKey:    id,
		instructionsKey: 2,
	})
	before := snapshotMachine(t, client, id)
	path := filepath.Join(filepath.Dir(rom), "flags.cdl")
	callTool[cdlExportResult](t, client, cdlExportToolName, map[string]any{
		machineIDKey: id,
		pathKey:      path,
	})
	exported, err := os.ReadFile(path)
	assert.NoError(t, err)
	for _, space := range []string{spacePRG, spaceCHR} {
		result := callTool[cdlReadResult](t, client, cdlReadToolName, map[string]any{
			machineIDKey: id,
			spaceKey:     space,
			lengthKey:    16,
		})
		assert.Len(t, result.Entries, 16)
		base := 0
		if space == spaceCHR {
			base = created.ROM.PRGSize
		}
		for index, entry := range result.Entries {
			assert.Equal(t, index, entry.Offset)
			assert.Equal(t, exported[base+index], entry.Raw)
		}
		if space == spacePRG {
			assert.True(t, slices.Contains(result.Entries[0].Flags, "code"))
		} else {
			assert.Equal(t, []string{}, result.Entries[0].Flags)
		}
	}
	assert.Equal(t, before, snapshotMachine(t, client, id))
}

func TestCDLReadBounds(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	for _, bounds := range [][2]int{{-1, 1}, {0, 0}, {0, -1}, {0, 4097}, {created.ROM.PRGSize, 1}} {
		assert.NotEmpty(t, callError(t, client, cdlReadToolName, map[string]any{
			machineIDKey: created.MachineID,
			spaceKey:     spacePRG,
			offsetKey:    bounds[0],
			lengthKey:    bounds[1],
		}))
	}
	assert.Contains(t, callError(t, client, cdlReadToolName, map[string]any{
		machineIDKey: created.MachineID,
		spaceKey:     spaceRAM,
	}), "use prg or chr")
	result := callTool[cdlReadResult](t, client, cdlReadToolName, map[string]any{
		machineIDKey: created.MachineID,
		spaceKey:     spacePRG,
		offsetKey:    created.ROM.PRGSize - 1,
		lengthKey:    1,
	})
	assert.Equal(t, created.ROM.PRGSize-1, result.Entries[0].Offset)
	assert.Equal(t, byte(0), result.Entries[0].Raw)
}

func TestDecodeCDLFlags(t *testing.T) {
	prg := decodeCDLFlags([]codedatalog.PrgFlag{0xff}, 7, codedatalog.PrgFlag.Names)
	assert.Equal(t, []string{"code", "data", "rom_bank_mapped_low", "rom_bank_mapped_high", "indirect_code",
		"indirect_data", "pcm_audio", "sub_entry_point"}, prg[0].Flags)
	chr := decodeCDLFlags([]codedatalog.ChrFlag{3}, 7, codedatalog.ChrFlag.Names)
	assert.Equal(t, []string{"drawn_on_screen", "read_programmatically"}, chr[0].Flags)
}

func TestCDLEnableRetainsRecordedFlags(t *testing.T) {
	_, client, rom := counterServer(t)
	created := callTool[createMachineResult](t, client, createMachineToolName, map[string]any{
		romKey:        rom,
		entrypointKey: 0x8000,
	})
	id := created.MachineID
	state := callTool[cdlState](t, client, cdlSetEnabledToolName, map[string]any{
		machineIDKey: id,
		enabledKey:   true,
	})
	assert.True(t, state.Enabled)
	callTool[stepResultOut](t, client, stepToolName, map[string]any{
		machineIDKey:    id,
		modeKey:         modeInstructions,
		instructionsKey: 2,
	})
	before := callTool[coverageResult](t, client, coverageToolName, map[string]any{machineIDKey: id})
	assert.True(t, before.Segments[0].Code > 0)
	for _, enabled := range []bool{true, true, false, false} {
		state := callTool[cdlState](t, client, cdlSetEnabledToolName, map[string]any{
			machineIDKey: id,
			enabledKey:   enabled,
		})
		assert.Equal(t, enabled, state.Enabled)
		after := callTool[coverageResult](t, client, coverageToolName, map[string]any{machineIDKey: id})
		assert.Equal(t, before, after)
	}
}
