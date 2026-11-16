package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/log"
	"github.com/retroenv/retrogolib/mcp"
)

// registerDebugTools registers the debugging tools on the MCP server.
func (s *Server) registerDebugTools() error {
	tools := []func() error{
		s.registerAddBreakpoint,
		s.registerRemoveBreakpoint,
		s.registerListBreakpoints,
		s.registerRunUntil,
		s.registerDisassemble,
		s.registerTraceStart,
		s.registerTraceStop,
		s.registerGetTrace,
		s.registerCallStack,
		s.registerCheckpoint,
		s.registerRestore,
		s.registerForkMachine,
		s.registerRestoreMachine,
		s.registerEventLog,
	}

	for _, register := range tools {
		if err := register(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) registerAddBreakpoint() error {
	return addTool(s, mcp.Tool{
		Name: addBreakpointToolName,
		Description: "Add an exec, read, write, or access breakpoint on an address range. " +
			"Returns the breakpoint id.",
		Annotations: writeAnnotations(),
	}, s.addBreakpoint)
}

func (s *Server) registerRemoveBreakpoint() error {
	return addTool(s, mcp.Tool{
		Name:        removeBreakpointToolName,
		Description: "Remove a breakpoint by id.",
		Annotations: writeAnnotations(),
	}, s.removeBreakpoint)
}

func (s *Server) registerListBreakpoints() error {
	return addTool(s, mcp.Tool{
		Name:        listBreakpointsToolName,
		Description: "List the breakpoints with their hit counts.",
		Annotations: readAnnotations(),
	}, s.listBreakpoints)
}

func (s *Server) registerRunUntil() error {
	return addTool(s, mcp.Tool{
		Name: runUntilToolName,
		Description: "Run with the breakpoints until one hits or a frame, instruction, cycle, " +
			"or wall-clock limit is reached. Returns the stop reason and the trace tail.",
		Annotations: writeAnnotations(),
	}, s.runUntil)
}

func (s *Server) registerDisassemble() error {
	return addTool(s, mcp.Tool{
		Name:        disassembleToolName,
		Description: "Disassemble instructions from a CPU address with side-effect-free reads.",
		Annotations: readAnnotations(),
	}, s.disassemble)
}

func (s *Server) registerTraceStart() error {
	return addTool(s, mcp.Tool{
		Name:        traceStartToolName,
		Description: "Start capturing nestest-format trace lines into the machine trace ring.",
		Annotations: writeAnnotations(),
	}, s.traceStart)
}

func (s *Server) registerTraceStop() error {
	return addTool(s, mcp.Tool{
		Name:        traceStopToolName,
		Description: "Stop capturing trace lines.",
		Annotations: writeAnnotations(),
	}, s.traceStop)
}

func (s *Server) registerGetTrace() error {
	return addTool(s, mcp.Tool{
		Name:        getTraceToolName,
		Description: "Return a page of the captured trace lines.",
		Annotations: readAnnotations(),
	}, s.getTrace)
}

func (s *Server) registerCallStack() error {
	return addTool(s, mcp.Tool{
		Name:        callStackToolName,
		Description: "Return the call stack from the CPU stack events.",
		Annotations: readAnnotations(),
	}, s.callStack)
}

func (s *Server) registerCheckpoint() error {
	return addTool(s, mcp.Tool{
		Name:        checkpointToolName,
		Description: "Name the current point of the machine event log, write the log to <machine>.events.json, and return the checkpoint_id.",
		Annotations: writeAnnotations(),
	}, s.checkpoint)
}

func (s *Server) registerRestore() error {
	return addTool(s, mcp.Tool{
		Name:        restoreToolName,
		Description: "Replay the machine event log to a checkpoint_id or a frame. Later events are dropped.",
		Annotations: writeAnnotations(),
	}, s.restore)
}

func (s *Server) registerForkMachine() error {
	return addTool(s, mcp.Tool{
		Name:        forkMachineToolName,
		Description: "Create a new machine that replays another machine event log to a frame.",
		Annotations: writeAnnotations(),
	}, s.forkMachine)
}

func (s *Server) registerRestoreMachine() error {
	return addTool(s, mcp.Tool{
		Name:        restoreMachineToolName,
		Description: "Recreate a machine from a ROM and an event log file after a server restart.",
		Annotations: writeAnnotations(),
	}, s.restoreMachine)
}

func (s *Server) registerEventLog() error {
	return addTool(s, mcp.Tool{
		Name:        eventLogToolName,
		Description: "Return a page of the recorded inputs and pokes.",
		Annotations: readAnnotations(),
	}, s.eventLog)
}

func (s *Server) addBreakpoint(_ context.Context, _ mcp.ToolRequest,
	in addBreakpointArgs) (addBreakpointResult, error) {

	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return addBreakpointResult{}, err
	}
	kind, err := parseBreakpointKind(in.Kind)
	if err != nil {
		return addBreakpointResult{}, err
	}

	end := in.Address
	if in.End != nil {
		end = *in.End
	}

	if end < in.Address {
		return addBreakpointResult{}, errors.New("end address is before the start address")
	}
	var value *byte
	if in.Value != nil {
		if *in.Value < 0 || *in.Value > math.MaxUint8 {
			return addBreakpointResult{}, fmt.Errorf("value %d is outside 0 to 255", *in.Value)
		}
		v := byte(*in.Value)
		value = &v
	}

	id := machine.addBreakpoint(nes.Breakpoint{
		Bank:  in.Bank,
		End:   uint16(end),
		Kind:  kind,
		Once:  in.Once != nil && *in.Once,
		Start: uint16(in.Address),
		Value: value,
	})

	return addBreakpointResult{
		ID: int(id),
	}, nil
}

func (s *Server) removeBreakpoint(_ context.Context, _ mcp.ToolRequest,
	in removeBreakpointArgs) (removeBreakpointResult, error) {

	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return removeBreakpointResult{}, err
	}
	return removeBreakpointResult{
		Removed: machine.removeBreakpoint(nes.BreakpointID(in.ID)),
	}, nil
}

func (s *Server) listBreakpoints(_ context.Context, _ mcp.ToolRequest,
	in machineIDArgs) (listBreakpointsResult, error) {

	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return listBreakpointsResult{}, err
	}
	statuses := machine.listBreakpoints()
	out := make([]breakpointInfo, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, breakpointInfo{
			Address: Address(status.Start),
			Bank:    status.Bank,
			End:     Address(status.End),
			Hits:    status.Hits,
			ID:      int(status.ID),
			Kind:    status.Kind.String(),
			Once:    status.Once,
		})
	}

	return listBreakpointsResult{
		Breakpoints: out,
	}, nil
}

func (s *Server) runUntil(ctx context.Context, _ mcp.ToolRequest, in runUntilArgs) (runUntilResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return runUntilResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return runUntilResult{}, err
	}
	inputs, err := parseInputs(in.Inputs)
	if err != nil {
		return runUntilResult{}, err
	}

	opts := nes.RunUntilOptions{
		Frames:          uint64Value(in.Frames),
		MaxCycles:       uint64Value(in.Cycles),
		MaxInstructions: uint64Value(in.Instructions),
	}
	outcome, err := machine.runUntil(ctx, opts, inputs, timeout)
	if err != nil {
		return runUntilResult{}, err
	}
	return runUntilToResult(machine, outcome), nil
}

func (s *Server) disassemble(_ context.Context, _ mcp.ToolRequest, in disassembleArgs) (disassembleResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return disassembleResult{}, err
	}
	if in.Count <= 0 || in.Count > maxDisassemble {
		return disassembleResult{}, fmt.Errorf("count %d is outside 1 to %d", in.Count, maxDisassemble)
	}
	instructions := machine.disassemble(uint16(in.Address), in.Count)

	return disassembleResult{
		Instructions: instructions,
	}, nil
}

func (s *Server) traceStart(_ context.Context, _ mcp.ToolRequest, in traceStartArgs) (traceState, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return traceState{}, err
	}
	return machine.traceStart(), nil
}

func (s *Server) traceStop(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (traceState, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return traceState{}, err
	}
	return machine.traceStop(), nil
}

func (s *Server) getTrace(_ context.Context, _ mcp.ToolRequest, in getTraceArgs) (getTraceResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return getTraceResult{}, err
	}
	limit, offset, err := pageArguments(in.Limit, in.Offset, defaultTracePage)
	if err != nil {
		return getTraceResult{}, err
	}
	return machine.getTrace(limit, offset), nil
}

func (s *Server) callStack(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (callStackResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return callStackResult{}, err
	}
	return callStackResult{
		Frames: machine.callFrames(),
	}, nil
}

func (s *Server) checkpoint(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (checkpointResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return checkpointResult{}, err
	}
	result, data, err := machine.checkpoint()
	if err != nil {
		return checkpointResult{}, err
	}
	path, err := s.writeEventLog(machine.id, data)
	if err != nil {
		return checkpointResult{}, err
	}
	result.EventsFile = path

	return result, nil
}

func (s *Server) restore(ctx context.Context, _ mcp.ToolRequest, in restoreArgs) (restoreResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return restoreResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return restoreResult{}, err
	}
	target := replayTarget{
		Checkpoint: in.CheckpointID,

		Frame: in.Frame,
	}
	if err := machine.restore(ctx, target, timeout); err != nil {
		return restoreResult{}, err
	}
	return restoreResult{
		Frame:    machine.frame(),
		Restored: true,
	}, nil
}

func (s *Server) forkMachine(ctx context.Context, _ mcp.ToolRequest, in forkMachineArgs) (createMachineResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return createMachineResult{}, err
	}
	if err := s.acquireLoad(ctx); err != nil {
		return createMachineResult{}, err
	}
	defer s.releaseLoad()

	if err := s.registry.checkCapacity(); err != nil {
		return createMachineResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return createMachineResult{}, err
	}
	target := replayTarget{
		Checkpoint: in.CheckpointID,

		Frame: in.Frame,
	}
	fork, summary, err := machine.forkMachine(ctx, s.logger, target, timeout)
	if err != nil {
		return createMachineResult{}, err
	}
	if err := s.registry.add(fork); err != nil {
		return createMachineResult{}, err
	}
	return createMachineResult{
		CPU:       fork.cpuState(),
		MachineID: fork.id,
		ROM:       summary,
	}, nil
}

func (s *Server) restoreMachine(ctx context.Context, _ mcp.ToolRequest,
	in restoreMachineArgs) (createMachineResult, error) {

	if err := s.acquireLoad(ctx); err != nil {
		return createMachineResult{}, err
	}
	defer s.releaseLoad()

	if err := s.registry.checkCapacity(); err != nil {
		return createMachineResult{}, err
	}
	romPath, err := s.roots.resolve(in.ROM)
	if err != nil {
		return createMachineResult{}, err
	}
	eventsPath, err := s.roots.resolve(in.Events)
	if err != nil {
		return createMachineResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return createMachineResult{}, err
	}
	logFile, err := readEventLog(ctx, eventsPath)
	if err != nil {
		return createMachineResult{}, err
	}

	machine, summary, err := loadRestoredMachine(ctx, s.logger, romPath, logFile, timeout)
	if err != nil {
		return createMachineResult{}, err
	}
	if err := s.registry.add(machine); err != nil {
		return createMachineResult{}, err
	}
	return createMachineResult{
		CPU:       machine.cpuState(),
		MachineID: machine.id,
		ROM:       summary,
	}, nil
}

func (s *Server) eventLog(_ context.Context, _ mcp.ToolRequest, in eventLogArgs) (eventLogResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return eventLogResult{}, err
	}
	limit, offset, err := pageArguments(in.Limit, in.Offset, defaultEventPage)
	if err != nil {
		return eventLogResult{}, err
	}
	return machine.eventLogPage(limit, offset), nil
}

// writeEventLog writes an event log file inside the first root.
func (s *Server) writeEventLog(machineID string, data []byte) (string, error) {
	path := filepath.Join(s.roots.first(), machineID+".events.json")
	err := writeFileAtomic(path, func(w io.Writer) error {
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("writing data: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("writing event log: %w", err)
	}
	return path, nil
}

// breakpointInfo describes one breakpoint.
type breakpointInfo struct {
	Address Address `json:"address"`
	Bank    *int    `json:"bank,omitempty"`
	End     Address `json:"end"`
	Hits    uint64  `json:"hits"`
	ID      int     `json:"id"`
	Kind    string  `json:"kind"`
	Once    bool    `json:"once"`
}

// instructionOut is one disassembled instruction.
type instructionOut struct {
	Address   Address `json:"address"`
	Bank      int     `json:"bank"`
	PRGOffset int     `json:"prg_offset"`

	Bytes    []int  `json:"bytes"`
	Mnemonic string `json:"mnemonic"`
	Operand  string `json:"operand"`
	Size     int    `json:"size"`
}

// addBreakpointArgs selects the breakpoint.
type addBreakpointArgs struct {
	Address   Address  `json:"address" description:"Start address."`
	Bank      *int     `json:"bank,omitempty" description:"PRG bank that must match."`
	End       *Address `json:"end,omitempty" description:"End of an inclusive range. Default the address."`
	Kind      string   `json:"kind,omitempty" description:"exec, read, write, or access. Default exec."`
	MachineID string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Once      *bool    `json:"once,omitempty" description:"Remove the breakpoint after its first hit."`
	Value     *int     `json:"value,omitempty" description:"Byte value that a write must match."`
}

// addBreakpointResult reports the new breakpoint.
type addBreakpointResult struct {
	ID int `json:"id"`
}

// removeBreakpointArgs selects a breakpoint.
type removeBreakpointArgs struct {
	ID        int    `json:"id" description:"Breakpoint id from add_breakpoint."`
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
}

// removeBreakpointResult reports the removal.
type removeBreakpointResult struct {
	Removed bool `json:"removed"`
}

// listBreakpointsResult lists the breakpoints.
type listBreakpointsResult struct {
	Breakpoints []breakpointInfo `json:"breakpoints"`
}

// runUntilArgs selects the run limits.
type runUntilArgs struct {
	Cycles       *uint64    `json:"cycles,omitempty" description:"CPU cycle limit."`
	Frames       *uint64    `json:"frames,omitempty" description:"Frame limit."`
	Inputs       []inputArg `json:"inputs,omitempty" description:"Controller changes."`
	Instructions *uint64    `json:"instructions,omitempty" description:"Instruction limit."`
	MachineID    string     `json:"machine_id" description:"Identifier returned by create_machine."`
	Timeout      string     `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
}

// runUntilResult reports the stop boundary.
type runUntilResult struct {
	Reason nes.RunUntilReason `json:"reason"`

	BreakpointID  int     `json:"breakpoint_id"`
	HasBreakpoint bool    `json:"has_breakpoint"`
	HitAddress    Address `json:"hit_address"`
	HitPC         Address `json:"hit_pc"`
	HitValue      int     `json:"hit_value"`

	CPU   cpuStateResult `json:"cpu"`
	Trace []string       `json:"trace,omitempty"`

	CompletedFrames uint64  `json:"completed_frames"`
	CPUCycles       uint64  `json:"cpu_cycles"`
	Frame           uint64  `json:"frame"`
	Instructions    uint64  `json:"instructions"`
	PC              Address `json:"pc"`

	Error string `json:"error,omitempty"`
}

// disassembleArgs selects the code range.
type disassembleArgs struct {
	Address   Address `json:"address" description:"First instruction address."`
	Count     int     `json:"count" description:"Number of instructions."`
	MachineID string  `json:"machine_id" description:"Identifier returned by create_machine."`
}

// disassembleResult lists the decoded instructions.
type disassembleResult struct {
	Instructions []instructionOut `json:"instructions"`
}

// traceStartArgs selects the ring capacity.
type traceStartArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
}

// traceState reports the trace ring state.
type traceState struct {
	Capacity int  `json:"capacity"`
	Enabled  bool `json:"enabled"`
	Lines    int  `json:"lines"`
}

// getTraceArgs selects a page of trace lines.
type getTraceArgs struct {
	Limit     *int   `json:"limit,omitempty" description:"Line count. Default 100."`
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Offset    *int   `json:"offset,omitempty" description:"Start index. Default 0."`
}

// getTraceResult contains a page of trace lines.
type getTraceResult struct {
	Lines []string `json:"lines"`
	Total int      `json:"total"`
}

// callStackResult contains the reconstructed call stack.
type callStackResult struct {
	Frames []callFrame `json:"frames"`
}

// checkpointResult reports a checkpoint.
type checkpointResult struct {
	CheckpointID int    `json:"checkpoint_id"`
	EventsFile   string `json:"events_file"`
	Frame        uint64 `json:"frame"`
}

// restoreArgs selects a checkpoint or a frame to restore.
type restoreArgs struct {
	CheckpointID *int    `json:"checkpoint_id,omitempty" description:"Checkpoint from the checkpoint tool. Restores the exact state."`
	Frame        *uint64 `json:"frame,omitempty" description:"Frame to replay to, instead of a checkpoint."`
	MachineID    string  `json:"machine_id" description:"Identifier returned by create_machine."`
	Timeout      string  `json:"timeout,omitempty" description:"Wall-clock limit for the replay. Default 10s."`
}

// restoreResult reports a restore.
type restoreResult struct {
	Frame    uint64 `json:"frame"`
	Restored bool   `json:"restored"`
}

// forkMachineArgs selects a machine and the point to fork at.
type forkMachineArgs struct {
	CheckpointID *int    `json:"checkpoint_id,omitempty" description:"Checkpoint to fork at. Default the current state."`
	Frame        *uint64 `json:"frame,omitempty" description:"Frame to replay to, instead of a checkpoint."`
	MachineID    string  `json:"machine_id" description:"Identifier returned by create_machine."`
	Timeout      string  `json:"timeout,omitempty" description:"Wall-clock limit for the replay. Default 10s."`
}

// restoreMachineArgs selects a ROM and event log file.
type restoreMachineArgs struct {
	Events  string `json:"events" description:"Path to an event log file inside the allowed roots."`
	ROM     string `json:"rom" description:"Path to the .nes ROM of the event log, inside the allowed roots."`
	Timeout string `json:"timeout,omitempty" description:"Wall-clock limit for the replay. Default 10s."`
}

// eventLogArgs selects a page of events.
type eventLogArgs struct {
	Limit     *int   `json:"limit,omitempty" description:"Event count. Default 100."`
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Offset    *int   `json:"offset,omitempty" description:"Start index. Default 0."`
}

// eventLogResult contains a page of events.
type eventLogResult struct {
	Events []machineEvent `json:"events"`
	Total  int            `json:"total"`
}

// eventLogFile is the stored form of a machine event log. The ROM checksums,
// the entrypoint, and the battery policy let a restore reject a mismatched
// replay setup. The palette bytes let a restore use the same display colors
// when the original palette file is not available.
type eventLogFile struct {
	Version int `json:"version"`

	CHRCRC32 uint32 `json:"chr_crc32"`
	PRGCRC32 uint32 `json:"prg_crc32"`
	ROMHash  string `json:"rom_sha256,omitempty"`

	Battery         bool   `json:"battery"`
	BatteryData     []byte `json:"battery_data,omitempty"`
	BatterySnapshot bool   `json:"battery_snapshot,omitempty"`
	Entrypoint      *int   `json:"entrypoint,omitempty"`
	Palette         []byte `json:"palette,omitempty"`

	Cycles uint64         `json:"cycles,omitempty"`
	Events []machineEvent `json:"events"`
	Frame  uint64         `json:"frame"`
}

// parseBreakpointKind decodes a breakpoint kind name.
func parseBreakpointKind(value string) (nes.BreakpointKind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "exec", "execute":
		return nes.BreakpointExec, nil
	case "read":
		return nes.BreakpointRead, nil
	case "write":
		return nes.BreakpointWrite, nil
	case "access":
		return nes.BreakpointAccess, nil
	default:
		return 0, fmt.Errorf("unknown breakpoint kind %q", value)
	}
}

// uint64Value returns a value or zero.
func uint64Value(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}

// valueOr returns a value or a fallback.
func valueOr(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

// bytesToInts converts bytes into an integer slice.
func bytesToInts(data []byte) []int {
	out := make([]int, len(data))
	for index, value := range data {
		out[index] = int(value)
	}

	return out
}

// Page limits for the trace, event, and disassembly tools.
const (
	maxDisassemble   = 256
	defaultTracePage = 100
	defaultEventPage = 100
	maxPage          = 10000

	eventLogVersion = 1
)

// pageArguments validates a page limit and offset and applies the default limit.
func pageArguments(limit, offset *int, defaultLimit int) (int, int, error) {
	pageLimit := valueOr(limit, defaultLimit)
	pageOffset := valueOr(offset, 0)
	if pageLimit <= 0 || pageLimit > maxPage {
		return 0, 0, fmt.Errorf(invalidLimitFormat, pageLimit, maxPage)
	}
	if pageOffset < 0 {
		return 0, 0, fmt.Errorf(negativeOffsetFormat, pageOffset)
	}
	return pageLimit, pageOffset, nil
}

// runUntilToResult converts a run outcome into a tool result.
func runUntilToResult(machine *Machine, outcome runUntilOutcome) runUntilResult {
	result := outcome.Result
	out := runUntilResult{
		Reason: result.Reason,

		CPU:   machine.cpuState(),
		Trace: outcome.Trace,

		CompletedFrames: result.CompletedFrames,
		CPUCycles:       result.CPUCycles,
		Frame:           result.Frame,
		Instructions:    result.Instructions,
		PC:              Address(result.PC),

		Error: result.Error,
	}
	if result.HasHit {
		out.BreakpointID = int(result.Hit.ID)
		out.HasBreakpoint = true
		out.HitPC = Address(result.Hit.PC)
		out.HitAddress = Address(result.Hit.Address)
		out.HitValue = int(result.Hit.Value)
	}

	return out
}

// readEventLog reads a machine event log file.
func readEventLog(ctx context.Context, path string) (eventLogFile, error) {
	data, err := readRegularFile(ctx, path, maxEventLogFileBytes)
	if err != nil {
		return eventLogFile{}, fmt.Errorf("reading event log: %w", err)
	}
	var logFile eventLogFile
	if err := json.Unmarshal(data, &logFile); err != nil {
		return eventLogFile{}, fmt.Errorf("decoding event log: %w", err)
	}
	if logFile.Version != eventLogVersion {
		return eventLogFile{}, fmt.Errorf("event log version %d is not %d", logFile.Version, eventLogVersion)
	}
	return logFile, nil
}

// loadRestoredMachine checks the recorded boot data before replay.
func loadRestoredMachine(ctx context.Context, logger *log.Logger, romPath string, logFile eventLogFile,
	timeout time.Duration) (*Machine, romSummary, error) {

	budget, err := historyBytes(logFile.Events)
	if err != nil {
		return nil, romSummary{}, err
	}
	entrypoint := -1
	if logFile.Entrypoint != nil {
		entrypoint = *logFile.Entrypoint
	}

	if logFile.Battery && !logFile.BatterySnapshot {
		return nil, romSummary{}, errors.New("battery event log has no initial save data; create a new checkpoint")
	}
	machine, summary, err := loadMachine(ctx, logger, romPath, entrypoint, logFile.Battery, logFile.Palette, &logFile.BatteryData)
	if err != nil {
		return nil, romSummary{}, err
	}
	if summary.PRGCRC32 != logFile.PRGCRC32 || summary.CHRCRC32 != logFile.CHRCRC32 ||
		(logFile.ROMHash != "" && summary.SHA256 != logFile.ROMHash) {

		return nil, romSummary{}, errors.New("the ROM does not match the ROM of the event log")
	}

	machine.events = logFile.Events
	machine.historyBytes = budget
	target := replayTarget{
		Cycles: logFile.Cycles,
		Frame:  &logFile.Frame,
	}
	if err := machine.restore(ctx, target, timeout); err != nil {
		return nil, romSummary{}, err
	}
	return machine, summary, nil
}
