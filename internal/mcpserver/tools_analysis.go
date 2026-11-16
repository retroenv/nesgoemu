package mcpserver

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	neslib "github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/mcp"
)

const (
	maxBankWrites     = 256
	defaultSearchPage = 50

	// defaultEventLimit and maxEventLimit bound the events of watch_writes.
	defaultEventLimit = 1000
	maxEventLimit     = 10000

	// maxAccessorValues limits the distinct written values kept per instruction.
	maxAccessorValues = 8
)

// registerAnalysisTools registers the reverse-engineering tools.
func (s *Server) registerAnalysisTools() error {
	tools := []func() error{
		s.registerCDLSetEnabled,
		s.registerCDLReset,
		s.registerCDLExport,
		s.registerCDLRead,
		s.registerCoverage,
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

func (s *Server) registerFindAccessors() error {
	return addTool(s, mcp.Tool{
		Name: findAccessorsToolName,
		Description: "Run frames with a read and write observer on an address range and return " +
			"the instructions that accessed it.",
		Annotations: writeAnnotations(),
	}, s.findAccessors)
}

func (s *Server) registerMemorySearch() error {
	schema, err := memorySearchInputSchema()
	if err != nil {
		return err
	}
	return addTool(s, mcp.Tool{
		Name: memorySearchToolName,
		Description: "Search internal RAM. start returns search_id; filter, list, and release require that handle. " +
			"Each machine permits 16 searches. Handles last until release, restore, hard reset, or machine expiry. " +
			"Filters: equal, changed, unchanged, increased, decreased, value.",
		InputSchema: schema,
		Annotations: writeAnnotations(),
	}, s.memorySearch)
}

func (s *Server) registerRAMDiff() error {
	return addTool(s, mcp.Tool{
		Name:        ramDiffToolName,
		Description: "Snapshot RAM, run frames, and return the changed bytes.",
		Annotations: writeAnnotations(),
	}, s.ramDiff)
}

func (s *Server) registerWatchWrites() error {
	return addTool(s, mcp.Tool{
		Name:        watchWritesToolName,
		Description: "Run frames and return the write log of an address range with frame and scanline.",
		Annotations: writeAnnotations(),
	}, s.watchWrites)
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
	end, err := rangeEnd(in.Address, in.End)
	if err != nil {
		return findAccessorsResult{}, err
	}
	hits, run, err := machine.findAccessors(ctx, uint16(in.Address), end, in.Frames, timeout)
	if err != nil {
		return findAccessorsResult{}, err
	}
	return findAccessorsResult{
		Hits: hits,
		Run:  run,
	}, nil
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
	changed, run, err := machine.ramDiff(ctx, in.Frames, inputs, timeout)
	if err != nil {
		return ramDiffResult{}, err
	}
	return ramDiffResult{
		Changed: changed,
		Run:     run,
	}, nil
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
	end, err := rangeEnd(in.Address, in.End)
	if err != nil {
		return watchWritesResult{}, err
	}
	limit, err := eventLimit(in.Limit)
	if err != nil {
		return watchWritesResult{}, err
	}
	return machine.watchWrites(ctx, uint16(in.Address), end, in.Frames, limit, timeout)
}

// bankState returns the mapper windows and the recent bank switches.
func (m *Machine) bankState() bankStateResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.sys.Bus.Mapper.State()
	switches := append([]bankWrite{}, m.bankWrites...)

	return bankStateResult{
		CHRWindows: state.ChrWindows,
		Mirroring:  mirrorName(m.sys.Bus.Mapper.MirrorMode()),
		PRGWindows: state.PrgWindows,
		Switches:   switches,
	}
}

// findAccessors runs frames and returns the instructions that accessed a range.
// The count is the number of bus accesses, so a read-modify-write instruction
// counts a read and a write.
func (m *Machine) findAccessors(ctx context.Context, start, end uint16, frames uint64,
	timeout time.Duration) ([]accessorHit, runSummary, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	accesses := map[nes.AddressLocation]*accessorStats{}
	var location nes.AddressLocation
	removePre := m.sys.AddPreExecutionHook(func(cpu *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) {
		bank, offset, _ := m.sys.PRGLocation(cpu.PC)
		location = nes.AddressLocation{
			Address:   cpu.PC,
			Bank:      bank,
			PRGOffset: offset,
		}
	})
	removeBus := m.sys.AddBusCycleObserver(func(cycle cpu6502.BusCycle) {
		if cycle.Address < start || cycle.Address > end {
			return
		}
		stats := accesses[location]
		if stats == nil {
			stats = &accessorStats{}
			accesses[location] = stats
		}

		stats.add(cycle)
	})
	defer removePre()
	defer removeBus()

	result, err := m.runFramesLocked(ctx, frames, nil, timeout)
	if err != nil {
		return nil, runSummary{}, err
	}
	return accessorHits(accesses), summarizeRun(result), nil
}

// watchWrites runs frames and returns the writes to a range. It runs one frame
// at a time and stops after the frame in which the event count reaches the
// limit, so a busy range cannot use unbounded memory.
func (m *Machine) watchWrites(ctx context.Context, start, end uint16, frames uint64, limit int,
	timeout time.Duration) (watchWritesResult, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	var events []writeEvent
	truncated := false
	remove := m.sys.AddBusCycleObserver(func(cycle cpu6502.BusCycle) {
		if !cycle.Write || cycle.Address < start || cycle.Address > end {
			return
		}
		if len(events) >= limit {
			truncated = true
			return
		}
		events = append(events, writeEvent{
			Address:  Address(cycle.Address),
			Frame:    m.sys.Bus.PPU.Frame(),
			Scanline: ppuScanline(m.sys),
			Value:    int(cycle.Value),
		})
	})
	defer remove()

	deadline := time.Now().Add(timeout)
	summary := runSummary{
		Reason: nes.StopFrames,
	}

	for summary.CompletedFrames < frames && !truncated {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			summary.TimedOut = true
			break
		}
		result, err := m.runFramesLocked(ctx, 1, nil, remaining)
		if err != nil {
			return watchWritesResult{}, err
		}
		summary.CompletedFrames += result.CompletedFrames
		if result.TimedOut {
			summary.TimedOut = true
			break
		}
	}

	switch {
	case truncated:
		summary.Reason = stopEventLimit
	case summary.TimedOut:
		summary.Reason = nes.StopTimeout
	}

	return watchWritesResult{
		Run:       summary,
		Truncated: truncated,
		Writes:    events,
	}, nil
}

// ramDiff snapshots RAM, runs frames, and returns the changed bytes.
func (m *Machine) ramDiff(ctx context.Context, frames uint64, inputs []controllerInput,
	timeout time.Duration) ([]memorySearchEntry, runSummary, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	before := m.ramSnapshot()
	result, err := m.runFramesLocked(ctx, frames, inputs, timeout)
	if err != nil {
		return nil, runSummary{}, err
	}
	return ramDiff(before, m.ramSnapshot()), summarizeRun(result), nil
}

// observeWrite records bank switches and watched writes.
func (m *Machine) observeWrite(event memory.WriteEvent) {
	if event.Address >= neslib.CodeBaseAddress {
		m.bankWrites = append(m.bankWrites, bankWrite{
			Address: Address(event.Address),
			Frame:   event.Frame,
			Value:   int(event.Value),
		})
		if len(m.bankWrites) > maxBankWrites {
			m.bankWrites = m.bankWrites[len(m.bankWrites)-maxBankWrites:]
		}
	}
}

// bankStateResult reports the mapper windows and recent switches.
type bankStateResult struct {
	CHRWindows []int       `json:"chr_windows"`
	Mirroring  string      `json:"mirroring"`
	PRGWindows []int       `json:"prg_windows"`
	Switches   []bankWrite `json:"switches"`
}

// accessorHit is one instruction that accessed a watched range.
type accessorHit struct {
	Bank      int     `json:"bank"`
	PC        Address `json:"pc"`
	PRGOffset int     `json:"prg_offset"`

	Reads  uint64 `json:"reads"`
	Values []int  `json:"values_written,omitempty"`
	Writes uint64 `json:"writes"`
}

// accessorStats counts the accesses of one instruction.
type accessorStats struct {
	reads  uint64
	values []int
	writes uint64
}

// findAccessorsArgs selects the range and run length.
type findAccessorsArgs struct {
	Address   Address  `json:"address" description:"Start of the range."`
	End       *Address `json:"end,omitempty" description:"End of the range. Default the start address."`
	Frames    uint64   `json:"frames" description:"Frames to run."`
	MachineID string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Timeout   string   `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
}

// findAccessorsResult lists the accessors.
type findAccessorsResult struct {
	Hits []accessorHit `json:"hits"`
	Run  runSummary    `json:"run"`
}

// ramDiffArgs selects the run.
type ramDiffArgs struct {
	Frames    uint64     `json:"frames" description:"Frames to run."`
	Inputs    []inputArg `json:"inputs,omitempty" description:"Controller changes."`
	MachineID string     `json:"machine_id" description:"Identifier returned by create_machine."`
	Timeout   string     `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
}

// ramDiffResult reports the changed bytes.
type ramDiffResult struct {
	Changed []memorySearchEntry `json:"changed"`
	Run     runSummary          `json:"run"`
}

// watchWritesArgs selects the range and run length.
type watchWritesArgs struct {
	Address   Address  `json:"address" description:"Start of the range."`
	End       *Address `json:"end,omitempty" description:"End of the range. Default the start address."`
	Frames    uint64   `json:"frames" description:"Frames to run."`
	Limit     *int     `json:"limit,omitempty" description:"Maximum events to return. Default 1000, maximum 10000. The run stops after the frame that reaches it."`
	MachineID string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Timeout   string   `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
}

// watchWritesResult lists the writes.
type watchWritesResult struct {
	Run       runSummary   `json:"run"`
	Truncated bool         `json:"truncated" description:"True when more writes happened than the limit."`
	Writes    []writeEvent `json:"writes"`
}

// runSummary reports how a tool run ended, so an agent can tell a complete
// run from a timeout.
type runSummary struct {
	CompletedFrames uint64             `json:"completed_frames"`
	Reason          nes.RunUntilReason `json:"reason"`
	TimedOut        bool               `json:"timed_out"`
}

// writeEvent is one observed CPU write with its PPU position.
type writeEvent struct {
	Address  Address `json:"address"`
	Frame    uint64  `json:"frame"`
	Scanline int     `json:"scanline"`
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
// summarizeRun converts a run result into the summary that tools report.
func summarizeRun(result runFramesResult) runSummary {
	return runSummary{
		CompletedFrames: result.CompletedFrames,
		Reason:          result.Reason,
		TimedOut:        result.TimedOut,
	}
}

// rangeEnd returns the end of a range or the start address. It rejects an end
// below the start.
func rangeEnd(start Address, end *Address) (uint16, error) {
	if end == nil {
		return uint16(start), nil
	}
	if *end < start {
		return 0, fmt.Errorf("end %s is below start %s", *end, start)
	}
	return uint16(*end), nil
}

// eventLimit returns the event limit of a watch tool.
func eventLimit(limit *int) (int, error) {
	if limit == nil {
		return defaultEventLimit, nil
	}
	if *limit < 1 || *limit > maxEventLimit {
		return 0, fmt.Errorf(invalidLimitFormat, *limit, maxEventLimit)
	}
	return *limit, nil
}

// accessorHits converts the per-PC access statistics into hits sorted by PC.
func accessorHits(accesses map[nes.AddressLocation]*accessorStats) []accessorHit {
	hits := make([]accessorHit, 0, len(accesses))
	for location, stats := range accesses {
		hits = append(hits, accessorHit{
			Bank:      location.Bank,
			PC:        Address(location.Address),
			PRGOffset: location.PRGOffset,

			Reads:  stats.reads,
			Values: stats.values,
			Writes: stats.writes,
		})
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].PC != hits[j].PC {
			return hits[i].PC < hits[j].PC
		}
		return hits[i].PRGOffset < hits[j].PRGOffset
	})

	return hits
}

// ppuScanline returns the current PPU scanline.
func ppuScanline(sys *nes.System) int {
	if device, ok := sys.Bus.PPU.(*ppu.PPU); ok {
		return device.Snapshot().Scanline
	}
	return 0
}
