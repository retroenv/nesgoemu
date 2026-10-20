package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/mcp"
)

// registerAnalysisTools registers the reverse-engineering tools.
func (s *Server) registerAnalysisTools() error {
	tools := []func() error{
		s.registerCDLStart,
		s.registerCDLStop,
		s.registerCDLReset,
		s.registerCDLExport,
		s.registerCoverage,
		s.registerBankState,
		s.registerFindAccessors,
		s.registerMemorySearch,
		s.registerRAMDiff,
		s.registerWatchWrites,
	}
	for _, register := range tools {
		if err := register(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) registerCDLStart() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "cdl_start",
		Description: "Start code and data logging for the machine.",
		Annotations: writeAnnotations(),
	}, s.cdlStart)
	return registrationError(err, "cdl_start")
}

func (s *Server) registerCDLStop() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "cdl_stop",
		Description: "Stop code and data logging.",
		Annotations: writeAnnotations(),
	}, s.cdlStop)
	return registrationError(err, "cdl_stop")
}

func (s *Server) registerCDLReset() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "cdl_reset",
		Description: "Clear the recorded code and data flags.",
		Annotations: writeAnnotations(),
	}, s.cdlReset)
	return registrationError(err, "cdl_reset")
}

func (s *Server) registerCDLExport() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "cdl_export",
		Description: "Write a FCEUX-format .cdl file inside the roots for retrodisasm.",
		Annotations: writeAnnotations(),
	}, s.cdlExport)
	return registrationError(err, "cdl_export")
}

func (s *Server) registerCoverage() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "coverage",
		Description: "Report the executed and read bytes per PRG bank.",
		Annotations: readAnnotations(),
	}, s.coverage)
	return registrationError(err, "coverage")
}

func (s *Server) registerBankState() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "bank_state",
		Description: "Report the current PRG and CHR windows and the bank switches since the last call.",
		Annotations: readAnnotations(),
	}, s.bankState)
	return registrationError(err, "bank_state")
}

func (s *Server) registerFindAccessors() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name: "find_accessors",
		Description: "Run frames with a read and write observer on an address range and return " +
			"the instructions that accessed it.",
		Annotations: writeAnnotations(),
	}, s.findAccessors)
	return registrationError(err, "find_accessors")
}

func (s *Server) registerMemorySearch() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name: "memory_search",
		Description: "Cheat search. Start with a snapshot, then filter with equal, changed, " +
			"unchanged, increased, decreased, or value.",
		Annotations: writeAnnotations(),
	}, s.memorySearch)
	return registrationError(err, "memory_search")
}

func (s *Server) registerRAMDiff() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "ram_diff",
		Description: "Snapshot RAM, run frames, and return the changed bytes.",
		Annotations: writeAnnotations(),
	}, s.ramDiff)
	return registrationError(err, "ram_diff")
}

func (s *Server) registerWatchWrites() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "watch_writes",
		Description: "Run frames and return the write log of an address range with frame and scanline.",
		Annotations: writeAnnotations(),
	}, s.watchWrites)
	return registrationError(err, "watch_writes")
}

func (s *Server) cdlStart(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (cdlState, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return cdlState{}, err
	}
	return machine.cdlStart(), nil
}

func (s *Server) cdlStop(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (cdlState, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return cdlState{}, err
	}
	return machine.cdlStop(), nil
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
	path, err := s.roots.resolve(in.Path)
	if err != nil {
		return cdlExportResult{}, err
	}
	if err := machine.cdlExport(path); err != nil {
		return cdlExportResult{}, err
	}
	return cdlExportResult{Path: path}, nil
}

func (s *Server) coverage(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (coverageResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return coverageResult{}, err
	}
	return coverageResult{Banks: machine.cdlCoverage()}, nil
}

func (s *Server) bankState(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (bankStateResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return bankStateResult{}, err
	}
	return machine.bankState(), nil
}

func (s *Server) findAccessors(ctx context.Context, _ mcp.ToolRequest,
	in findAccessorsArgs) (findAccessorsResult, error) {

	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return findAccessorsResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return findAccessorsResult{}, err
	}
	hits, err := machine.findAccessors(ctx, uint16(in.Address), endAddress(in.Address, in.End), in.Frames, timeout)
	if err != nil {
		return findAccessorsResult{}, err
	}
	return findAccessorsResult{Hits: hits}, nil
}

func (s *Server) memorySearch(ctx context.Context, _ mcp.ToolRequest, in memorySearchArgs) (memorySearchResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return memorySearchResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return memorySearchResult{}, err
	}
	inputs, err := parseInputs(in.Inputs)
	if err != nil {
		return memorySearchResult{}, err
	}

	action := strings.ToLower(in.Action)
	switch action {
	case "start":
		return memorySearchResult{Candidates: machine.searchStart()}, nil

	case "filter":
		if !validSearchOperation(in.Op) {
			return memorySearchResult{}, fmt.Errorf("unknown filter %q; use equal, changed, unchanged, "+
				"increased, decreased, or value", in.Op)
		}
		if in.Value < 0 || in.Value > 0xFF {
			return memorySearchResult{}, fmt.Errorf("value %d is outside 0 to 255", in.Value)
		}
		if in.Frames > 0 {
			if _, err := machine.runFrames(ctx, in.Frames, inputs, timeout); err != nil {
				return memorySearchResult{}, err
			}
		}
		count := machine.searchFilter(in.Op, in.Value)
		if count < 0 {
			return memorySearchResult{}, errors.New("no active search; call memory_search with action start first")
		}
		return memorySearchResult{Candidates: count}, nil

	case "list":
		limit, offset, err := pageArguments(in.Limit, in.Offset, defaultSearchPage)
		if err != nil {
			return memorySearchResult{}, err
		}
		return memorySearchResult{
			Candidates: machine.searchCandidateCount(),
			Results:    machine.searchCandidates(limit, offset),
		}, nil

	default:
		return memorySearchResult{}, fmt.Errorf("unknown action %q; use start, filter, or list", in.Action)
	}
}

func (s *Server) ramDiff(ctx context.Context, _ mcp.ToolRequest, in ramDiffArgs) (ramDiffResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return ramDiffResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return ramDiffResult{}, err
	}
	inputs, err := parseInputs(in.Inputs)
	if err != nil {
		return ramDiffResult{}, err
	}
	changed, err := machine.ramDiff(ctx, in.Frames, inputs, timeout)
	if err != nil {
		return ramDiffResult{}, err
	}
	return ramDiffResult{Changed: changed}, nil
}

func (s *Server) watchWrites(ctx context.Context, _ mcp.ToolRequest, in watchWritesArgs) (watchWritesResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return watchWritesResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return watchWritesResult{}, err
	}
	events, err := machine.watchWrites(ctx, uint16(in.Address), endAddress(in.Address, in.End), in.Frames, timeout)
	if err != nil {
		return watchWritesResult{}, err
	}
	return watchWritesResult{Writes: events}, nil
}

// cdlStart starts the recorder.
func (m *Machine) cdlStart() cdlState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cdl.Start()
	return m.cdlStateLocked()
}

// cdlStop stops the recorder.
func (m *Machine) cdlStop() cdlState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cdl.Stop()
	return m.cdlStateLocked()
}

// cdlReset clears the recorder flags.
func (m *Machine) cdlReset() cdlState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cdl.Reset()
	return m.cdlStateLocked()
}

// cdlStateLocked reports the recorder coverage summary.
func (m *Machine) cdlStateLocked() cdlState {
	return cdlState{Banks: len(m.cdl.Coverage())}
}

// cdlCoverage returns the per-bank coverage.
func (m *Machine) cdlCoverage() []nes.CDLCoverage {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.cdl.Coverage()
}

// cdlExport writes the CDL file to a path.
func (m *Machine) cdlExport(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating cdl file: %w", err)
	}
	if err := m.cdl.Write(file); err != nil {
		_ = file.Close()
		return fmt.Errorf("writing cdl file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing cdl file: %w", err)
	}
	return nil
}

// bankState returns the mapper windows and the recent bank switches.
func (m *Machine) bankState() bankStateResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.sys.Bus.Mapper.State()
	switches := append([]bankWrite(nil), m.bankWrites...)
	m.bankWrites = nil
	return bankStateResult{
		PRGWindows: state.PrgWindows,
		CHRWindows: state.ChrWindows,
		Mirroring:  mirrorName(m.sys.Bus.Mapper.MirrorMode()),
		Switches:   switches,
	}
}

// findAccessors runs frames and returns the instructions that accessed a range.
// The count is the number of bus accesses, so a read-modify-write instruction
// counts a read and a write.
func (m *Machine) findAccessors(ctx context.Context, start, end uint16, frames uint64,
	timeout time.Duration) ([]accessorHit, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	accesses := map[uint16]*accessorStats{}
	var currentPC uint16
	removePre := m.sys.AddPreExecutionHook(func(cpu *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) {
		currentPC = cpu.PC
	})
	removeBus := m.sys.AddBusCycleObserver(func(cycle cpu6502.BusCycle) {
		if cycle.Address < start || cycle.Address > end {
			return
		}
		stats := accesses[currentPC]
		if stats == nil {
			stats = &accessorStats{}
			accesses[currentPC] = stats
		}
		stats.add(cycle)
	})
	defer removePre()
	defer removeBus()

	if _, err := m.runFramesLocked(ctx, frames, nil, timeout); err != nil {
		return nil, err
	}
	return accessorHits(m, accesses), nil
}

// watchWrites runs frames and returns the writes to a range.
func (m *Machine) watchWrites(ctx context.Context, start, end uint16, frames uint64,
	timeout time.Duration) ([]writeEvent, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	var events []writeEvent
	remove := m.sys.AddBusCycleObserver(func(cycle cpu6502.BusCycle) {
		if !cycle.Write || cycle.Address < start || cycle.Address > end {
			return
		}
		events = append(events, writeEvent{
			Frame:    m.sys.Bus.PPU.Frame(),
			Scanline: ppuScanline(m.sys),
			Address:  Address(cycle.Address),
			Value:    int(cycle.Value),
		})
	})
	defer remove()

	if _, err := m.runFramesLocked(ctx, frames, nil, timeout); err != nil {
		return nil, err
	}
	return events, nil
}

// ramDiff snapshots RAM, runs frames, and returns the changed bytes.
func (m *Machine) ramDiff(ctx context.Context, frames uint64, inputs []controllerInput,
	timeout time.Duration) ([]memorySearchEntry, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	before := m.ramSnapshot()
	if _, err := m.runFramesLocked(ctx, frames, inputs, timeout); err != nil {
		return nil, err
	}
	return ramDiff(before, m.ramSnapshot()), nil
}

// searchCandidateCount returns the current candidate count.
func (m *Machine) searchCandidateCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.search == nil {
		return 0
	}
	return len(m.search.candidates)
}

// observeWrite records bank switches and watched writes.
func (m *Machine) observeWrite(event memory.WriteEvent) {
	if event.Address >= codeWriteBase {
		m.bankWrites = append(m.bankWrites, bankWrite{
			Frame:   event.Frame,
			Address: Address(event.Address),
			Value:   int(event.Value),
		})
		if len(m.bankWrites) > maxBankWrites {
			m.bankWrites = m.bankWrites[len(m.bankWrites)-maxBankWrites:]
		}
	}
}

// cdlState reports the recorder state.
type cdlState struct {
	Banks int `json:"banks"`
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

// coverageResult lists the per-bank coverage.
type coverageResult struct {
	Banks []nes.CDLCoverage `json:"banks"`
}

// bankStateResult reports the mapper windows and recent switches.
type bankStateResult struct {
	PRGWindows []int       `json:"prg_windows"`
	CHRWindows []int       `json:"chr_windows"`
	Mirroring  string      `json:"mirroring"`
	Switches   []bankWrite `json:"switches"`
}

// accessorHit is one instruction that accessed a watched range.
type accessorHit struct {
	PC        Address `json:"pc"`
	Bank      int     `json:"bank"`
	PRGOffset int     `json:"prg_offset"`
	Reads     uint64  `json:"reads"`
	Writes    uint64  `json:"writes"`
	Values    []int   `json:"values_written,omitempty"`
}

// accessorStats counts the accesses of one instruction.
type accessorStats struct {
	reads  uint64
	writes uint64
	values []int
}

// findAccessorsArgs selects the range and run length.
type findAccessorsArgs struct {
	MachineID string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Address   Address  `json:"address" description:"Start of the range."`
	End       *Address `json:"end,omitempty" description:"End of the range. Default the start address."`
	Frames    uint64   `json:"frames" description:"Frames to run."`
	Timeout   string   `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
}

// findAccessorsResult lists the accessors.
type findAccessorsResult struct {
	Hits []accessorHit `json:"hits"`
}

// memorySearchArgs selects a cheat search step.
type memorySearchArgs struct {
	MachineID string     `json:"machine_id" description:"Identifier returned by create_machine."`
	Action    string     `json:"action" description:"start, filter, or list."`
	Op        string     `json:"op,omitempty" description:"Filter: equal, changed, unchanged, increased, decreased, value."`
	Value     int        `json:"value,omitempty" description:"Value for the value filter."`
	Frames    uint64     `json:"frames,omitempty" description:"Frames to run before the filter."`
	Inputs    []inputArg `json:"inputs,omitempty" description:"Controller changes for the run."`
	Timeout   string     `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
	Limit     *int       `json:"limit,omitempty" description:"Candidate page size for list. Default 50."`
	Offset    *int       `json:"offset,omitempty" description:"Start index for list."`
}

// memorySearchResult reports the candidate count and page.
type memorySearchResult struct {
	Candidates int                 `json:"candidates"`
	Results    []memorySearchEntry `json:"results,omitempty"`
}

// ramDiffArgs selects the run.
type ramDiffArgs struct {
	MachineID string     `json:"machine_id" description:"Identifier returned by create_machine."`
	Frames    uint64     `json:"frames" description:"Frames to run."`
	Inputs    []inputArg `json:"inputs,omitempty" description:"Controller changes."`
	Timeout   string     `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
}

// ramDiffResult reports the changed bytes.
type ramDiffResult struct {
	Changed []memorySearchEntry `json:"changed"`
}

// watchWritesArgs selects the range and run length.
type watchWritesArgs struct {
	MachineID string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Address   Address  `json:"address" description:"Start of the range."`
	End       *Address `json:"end,omitempty" description:"End of the range. Default the start address."`
	Frames    uint64   `json:"frames" description:"Frames to run."`
	Timeout   string   `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
}

// watchWritesResult lists the writes.
type watchWritesResult struct {
	Writes []writeEvent `json:"writes"`
}

// writeEvent is one observed CPU write with its PPU position.
type writeEvent struct {
	Frame    uint64  `json:"frame"`
	Scanline int     `json:"scanline"`
	Address  Address `json:"address"`
	Value    int     `json:"value"`
}

// add counts one bus access. It keeps the first distinct written values.
func (a *accessorStats) add(cycle cpu6502.BusCycle) {
	if !cycle.Write {
		a.reads++
		return
	}
	a.writes++
	value := int(cycle.Value)
	if len(a.values) < maxAccessorValues && !slices.Contains(a.values, value) {
		a.values = append(a.values, value)
	}
}

// helpers.
const (
	codeWriteBase     = 0x8000
	maxBankWrites     = 256
	defaultSearchPage = 50

	// maxAccessorValues limits the distinct written values kept per instruction.
	maxAccessorValues = 8
)

// endAddress returns the end of a range or the start address.
func endAddress(start Address, end *Address) uint16 {
	if end == nil {
		return uint16(start)
	}
	return uint16(*end)
}

// accessorHits converts the per-PC access statistics into hits sorted by PC.
func accessorHits(machine *Machine, accesses map[uint16]*accessorStats) []accessorHit {
	hits := make([]accessorHit, 0, len(accesses))
	for pc, stats := range accesses {
		bank, offset, _ := machine.sys.PRGLocation(pc)
		hits = append(hits, accessorHit{
			PC:        Address(pc),
			Bank:      bank,
			PRGOffset: offset,
			Reads:     stats.reads,
			Writes:    stats.writes,
			Values:    stats.values,
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].PC < hits[j].PC })
	return hits
}

// ppuScanline returns the current PPU scanline.
func ppuScanline(sys *nes.System) int {
	if device, ok := sys.Bus.PPU.(*ppu.PPU); ok {
		return device.Snapshot().Scanline
	}
	return 0
}
