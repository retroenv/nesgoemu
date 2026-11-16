package mcpserver

import (
	"context"
	"fmt"
	"reflect"

	"github.com/retroenv/retrogolib/mcp"
	"github.com/retroenv/retrogolib/set"
)

const (
	sectionCPU       = "cpu"
	sectionPPU       = "ppu"
	sectionAPU       = "apu"
	sectionCartridge = "cartridge"
	sectionBanks     = "banks"
	sectionPalette   = "palette"
	sectionOAM       = "oam"
)

var inspectionSections = []string{
	sectionCPU, sectionPPU, sectionAPU, sectionCartridge, sectionBanks, sectionPalette, sectionOAM,
}

var allowedInspectionSections = set.NewFromSlice(inspectionSections)

func (s *Server) registerInspectMachine() error {
	schema, err := inputSchema(reflect.TypeFor[inspectMachineArgs](), configureInspectSchema)
	if err != nil {
		return err
	}
	return addTool(s, mcp.Tool{
		Name: inspectMachineToolName,
		Description: "Read selected sections of one machine without advancing time. " +
			"Sections share the same stopped state. Default cpu; request only the sections needed.",
		InputSchema: schema,
		Annotations: readAnnotations(),
	}, s.inspectMachine)
}

func (s *Server) inspectMachine(_ context.Context, _ mcp.ToolRequest,
	in inspectMachineArgs) (inspectMachineResult, error) {

	sections := in.Sections
	if sections == nil {
		sections = []string{sectionCPU}
	}

	if len(sections) == 0 || len(sections) > len(inspectionSections) {
		return inspectMachineResult{}, fmt.Errorf("sections must contain from 1 to %d entries", len(inspectionSections))
	}

	seen := set.New[string]()
	for _, section := range sections {
		if !allowedInspectionSections.Contains(section) || seen.Contains(section) {
			return inspectMachineResult{}, fmt.Errorf("unknown or repeated section %q", section)
		}
		seen.Add(section)
	}

	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return inspectMachineResult{}, err
	}
	result := inspectMachineResult{
		Frame: machine.frame(),
	}
	for _, section := range sections {
		if err := result.readSection(machine, section); err != nil {
			return inspectMachineResult{}, err
		}
	}

	return result, nil
}

type inspectMachineArgs struct {
	MachineID string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Sections  []string `json:"sections,omitempty" description:"Sections to return: cpu, ppu, apu, cartridge, banks, palette, oam. Default cpu."`
}

type inspectMachineResult struct {
	Frame uint64 `json:"frame"`

	APU       *apuStateResult    `json:"apu,omitempty"`
	Banks     *bankStateResult   `json:"banks,omitempty"`
	Cartridge *machineInfoResult `json:"cartridge,omitempty"`
	CPU       *cpuStateResult    `json:"cpu,omitempty"`
	OAM       *oamResult         `json:"oam,omitempty"`
	Palette   *paletteResult     `json:"palette,omitempty"`
	PPU       *ppuSnapshotOut    `json:"ppu,omitempty"`
}

func (r *inspectMachineResult) readSection(machine *Machine, section string) error {
	switch section {
	case sectionCPU:
		value := machine.cpuState()
		r.CPU = &value

	case sectionPPU:
		value, err := machine.ppuState()
		if err != nil {
			return err
		}
		r.PPU = &value

	case sectionAPU:
		value := machine.apuState()
		r.APU = &value

	case sectionCartridge:
		value := machine.machineInfo()
		r.Cartridge = &value

	case sectionBanks:
		value := machine.bankState()
		r.Banks = &value

	case sectionPalette:
		value := machine.paletteEntries()
		r.Palette = &value

	case sectionOAM:
		value := machine.oamEntries()
		r.OAM = &value
	}

	return nil
}

func configureInspectSchema(schema map[string]any) {
	sections := schemaProperty(schema, fieldSections)
	sections[schemaMinItems] = 1
	sections[schemaMaxItems] = len(inspectionSections)
	sections[schemaUniqueItems] = true
	sections[schemaItems] = map[string]any{
		schemaType: "string",
		schemaEnum: inspectionSections,
	}
}
