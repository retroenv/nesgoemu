package mcpserver

import (
	"context"
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
	"github.com/retroenv/retrogolib/mcp"
)

const readCDLFlagsFormat = "reading %s flags: %w"

func (s *Server) registerCDLRead() error {
	return addTool(s, mcp.Tool{
		Name: cdlReadToolName,
		Description: "Read raw and decoded CDL flags at physical PRG or CHR ROM offsets without advancing emulation. " +
			"CHR flags record program reads and rendering fetches at mapped CHR ROM offsets. " +
			"The complete range must fit in the selected ROM space.",
		Annotations: readAnnotations(),
	}, s.cdlRead)
}

func (s *Server) cdlRead(_ context.Context, _ mcp.ToolRequest, in cdlReadArgs) (cdlReadResult, error) {
	length := defaultReadLength
	if in.Length != nil {
		length = *in.Length
	}

	if length < 1 || length > maxReadLength {
		return cdlReadResult{}, fmt.Errorf("length must be from 1 to %d", maxReadLength)
	}
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return cdlReadResult{}, err
	}
	return machine.cdlRead(in.Space, in.Offset, length)
}

func (m *Machine) cdlRead(space string, offset, length int) (cdlReadResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := cdlReadResult{
		Offset: offset,
		Space:  space,
	}

	switch space {
	case spacePRG:
		flags, err := m.cdl.PRGRange(offset, length)
		if err != nil {
			return cdlReadResult{}, fmt.Errorf(readCDLFlagsFormat, space, err)
		}
		result.Entries = decodeCDLFlags(flags, offset, codedatalog.PrgFlag.Names)

	case spaceCHR:
		flags, err := m.cdl.CHRRange(offset, length)
		if err != nil {
			return cdlReadResult{}, fmt.Errorf(readCDLFlagsFormat, space, err)
		}
		result.Entries = decodeCDLFlags(flags, offset, codedatalog.ChrFlag.Names)

	default:
		return cdlReadResult{}, fmt.Errorf("unknown CDL space %q; use prg or chr", space)
	}

	return result, nil
}

func (s *Server) registerCDLSetEnabled() error {
	destructive := false
	openWorld := false
	return addTool(s, mcp.Tool{
		Name: cdlSetEnabledToolName,
		Description: "Enable or stop code and data logging without clearing recorded flags. " +
			"Repeated calls with the same enabled value have no additional effect. Use cdl_reset to clear flags.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			IdempotentHint:  true,
			OpenWorldHint:   &openWorld,
		},
	}, s.cdlSetEnabled)
}

func (s *Server) cdlSetEnabled(_ context.Context, _ mcp.ToolRequest, in cdlSetEnabledArgs) (cdlState, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return cdlState{}, err
	}
	return machine.setCDLEnabled(in.Enabled), nil
}

func (m *Machine) setCDLEnabled(enabled bool) cdlState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if enabled {
		m.cdl.Start()
	} else {
		m.cdl.Stop()
	}

	m.cdlEnabled = enabled

	return m.cdlStateLocked()
}

func (s *Server) registerCDLReset() error {
	return addTool(s, mcp.Tool{
		Name:        cdlResetToolName,
		Description: "Clear the recorded code and data flags.",
		Annotations: writeAnnotations(),
	}, s.cdlReset)
}

func (s *Server) registerCDLExport() error {
	return addTool(s, mcp.Tool{
		Name:        cdlExportToolName,
		Description: "Write a FCEUX-format .cdl file inside the roots for retrodisasm.",
		Annotations: writeAnnotations(),
	}, s.cdlExport)
}

func (s *Server) registerCoverage() error {
	return addTool(s, mcp.Tool{
		Name:        coverageToolName,
		Description: "Report the executed and read bytes in fixed 8 KiB PRG segments.",
		Annotations: readAnnotations(),
	}, s.coverage)
}

func (s *Server) cdlReset(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (cdlState, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return cdlState{}, err
	}
	return machine.cdlReset(), nil
}

func (s *Server) cdlExport(_ context.Context, _ mcp.ToolRequest, in cdlExportArgs) (cdlExportResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return cdlExportResult{}, err
	}
	path, err := s.roots.resolveOutput(in.Path, ".cdl")
	if err != nil {
		return cdlExportResult{}, err
	}
	if err := machine.cdlExport(path); err != nil {
		return cdlExportResult{}, err
	}
	return cdlExportResult{
		Path: path,
	}, nil
}

func (s *Server) coverage(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (coverageResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return coverageResult{}, err
	}
	return coverageResult{
		Segments: machine.cdlCoverage(),
	}, nil
}

// cdlStateLocked reports the recorder coverage summary.
func (m *Machine) cdlStateLocked() cdlState {
	return cdlState{
		Enabled:  m.cdlEnabled,
		Segments: len(m.cdl.Coverage()),
	}
}

// cdlCoverage returns the coverage in fixed 8 KiB PRG segments.
func (m *Machine) cdlCoverage() []nes.CDLCoverage {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.cdl.Coverage()
}

// cdlReset clears the recorder flags.
func (m *Machine) cdlReset() cdlState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cdl.Reset()

	return m.cdlStateLocked()
}

// cdlExport writes the CDL file to a path.
func (m *Machine) cdlExport(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := writeFileAtomic(path, m.cdl.Write)
	if err != nil {
		return fmt.Errorf("writing cdl file: %w", err)
	}
	return nil
}

type cdlReadArgs struct {
	Length    *int   `json:"length,omitempty" description:"Number of flags, from 1 to 4096. Default 256."`
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Offset    int    `json:"offset,omitempty" description:"Physical ROM byte offset. Default 0."`
	Space     string `json:"space" description:"ROM space: prg or chr."`
}

type cdlReadResult struct {
	Entries []cdlFlagEntry `json:"entries"`
	Offset  int            `json:"offset"`
	Space   string         `json:"space"`
}

type cdlFlagEntry struct {
	Flags  []string `json:"flags"`
	Offset int      `json:"offset"`
	Raw    byte     `json:"raw"`
}

type cdlSetEnabledArgs struct {
	Enabled   bool   `json:"enabled" description:"Enable or stop recording. Existing flags stay unchanged."`
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
}

// cdlState reports the recorder state.
type cdlState struct {
	Enabled  bool `json:"enabled"`
	Segments int  `json:"segments"`
}

// cdlExportArgs selects the output file.
type cdlExportArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Path      string `json:"path" description:"Output .cdl path inside the allowed roots."`
}

// cdlExportResult reports the written file.
type cdlExportResult struct {
	Path string `json:"path"`
}

// coverageResult lists the coverage in fixed 8 KiB PRG segments.
type coverageResult struct {
	Segments []nes.CDLCoverage `json:"segments"`
}

func decodeCDLFlags[T ~byte](flags []T, offset int, names func(T) []string) []cdlFlagEntry {
	entries := make([]cdlFlagEntry, len(flags))
	for index, flag := range flags {
		labels := names(flag)
		if labels == nil {
			labels = []string{}
		}

		entries[index] = cdlFlagEntry{
			Flags:  labels,
			Offset: offset + index,
			Raw:    byte(flag),
		}
	}

	return entries
}
