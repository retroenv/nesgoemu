package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/retroenv/nesgoemu/pkg/nes"
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
		s.registerStepOver,
		s.registerStepOut,
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
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name: "add_breakpoint",
		Description: "Add an exec, read, write, or access breakpoint on an address range. " +
			"Returns the breakpoint id.",
		Annotations: writeAnnotations(),
	}, s.addBreakpoint)
	return registrationError(err, "add_breakpoint")
}

func (s *Server) registerRemoveBreakpoint() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "remove_breakpoint",
		Description: "Remove a breakpoint by id.",
		Annotations: writeAnnotations(),
	}, s.removeBreakpoint)
	return registrationError(err, "remove_breakpoint")
}

func (s *Server) registerListBreakpoints() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "list_breakpoints",
		Description: "List the breakpoints with their hit counts.",
		Annotations: readAnnotations(),
	}, s.listBreakpoints)
	return registrationError(err, "list_breakpoints")
}

func (s *Server) registerRunUntil() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name: "run_until",
		Description: "Run with the breakpoints until one hits or a frame, instruction, cycle, " +
			"or wall-clock limit is reached. Returns the stop reason and the trace tail.",
		Annotations: writeAnnotations(),
	}, s.runUntil)
	return registrationError(err, "run_until")
}

func (s *Server) registerDisassemble() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "disassemble",
		Description: "Disassemble instructions from a CPU address with side-effect-free reads.",
		Annotations: readAnnotations(),
	}, s.disassemble)
	return registrationError(err, "disassemble")
}

func (s *Server) registerTraceStart() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "trace_start",
		Description: "Start capturing nestest-format trace lines into the machine trace ring.",
		Annotations: writeAnnotations(),
	}, s.traceStart)
	return registrationError(err, "trace_start")
}

func (s *Server) registerTraceStop() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "trace_stop",
		Description: "Stop capturing trace lines.",
		Annotations: writeAnnotations(),
	}, s.traceStop)
	return registrationError(err, "trace_stop")
}

func (s *Server) registerGetTrace() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "get_trace",
		Description: "Return a page of the captured trace lines.",
		Annotations: readAnnotations(),
	}, s.getTrace)
	return registrationError(err, "get_trace")
}

func (s *Server) registerCallStack() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "call_stack",
		Description: "Return the call stack from the CPU stack events.",
		Annotations: readAnnotations(),
	}, s.callStack)
	return registrationError(err, "call_stack")
}

func (s *Server) registerStepOver() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "step_over",
		Description: "Run the current call or a single instruction.",
		Annotations: writeAnnotations(),
	}, s.stepOver)
	return registrationError(err, "step_over")
}

func (s *Server) registerStepOut() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "step_out",
		Description: "Run until the current function returns.",
		Annotations: writeAnnotations(),
	}, s.stepOut)
	return registrationError(err, "step_out")
}

func (s *Server) registerCheckpoint() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "checkpoint",
		Description: "Name the current point of the machine event log, write the log to <machine>.events.json, and return the checkpoint_id.",
		Annotations: writeAnnotations(),
	}, s.checkpoint)
	return registrationError(err, "checkpoint")
}

func (s *Server) registerRestore() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "restore",
		Description: "Replay the machine event log to a checkpoint_id or a frame. Later events are dropped.",
		Annotations: writeAnnotations(),
	}, s.restore)
	return registrationError(err, "restore")
}

func (s *Server) registerForkMachine() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "fork_machine",
		Description: "Create a new machine that replays another machine event log to a frame.",
		Annotations: writeAnnotations(),
	}, s.forkMachine)
	return registrationError(err, "fork_machine")
}

func (s *Server) registerRestoreMachine() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "restore_machine",
		Description: "Recreate a machine from a ROM and an event log file after a server restart.",
		Annotations: writeAnnotations(),
	}, s.restoreMachine)
	return registrationError(err, "restore_machine")
}

func (s *Server) registerEventLog() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "event_log",
		Description: "Return a page of the recorded inputs and pokes.",
		Annotations: readAnnotations(),
	}, s.eventLog)
	return registrationError(err, "event_log")
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
		if *in.Value < 0 || *in.Value > 0xFF {
			return addBreakpointResult{}, fmt.Errorf("value %d is outside 0 to 255", *in.Value)
		}
		v := byte(*in.Value)
		value = &v
	}

	id := machine.addBreakpoint(nes.Breakpoint{
		Kind:  kind,
		Start: uint16(in.Address),
		End:   uint16(end),
		Bank:  in.Bank,
		Value: value,
		Once:  in.Once != nil && *in.Once,
	})
	return addBreakpointResult{ID: int(id)}, nil
}

func (s *Server) removeBreakpoint(_ context.Context, _ mcp.ToolRequest,
	in removeBreakpointArgs) (removeBreakpointResult, error) {

	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return removeBreakpointResult{}, err
	}
	return removeBreakpointResult{Removed: machine.removeBreakpoint(nes.BreakpointID(in.ID))}, nil
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
			ID:      int(status.ID),
			Kind:    status.Kind.String(),
			Address: Address(status.Start),
			End:     Address(status.End),
			Bank:    status.Bank,
			Once:    status.Once,
			Hits:    status.Hits,
		})
	}
	return listBreakpointsResult{Breakpoints: out}, nil
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
		MaxInstructions: uint64Value(in.Instructions),
		MaxCycles:       uint64Value(in.Cycles),
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
	return disassembleResult{Instructions: instructions}, nil
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
	return callStackResult{Frames: machine.callFrames()}, nil
}

func (s *Server) stepOver(ctx context.Context, _ mcp.ToolRequest, in machineIDArgs) (runUntilResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return runUntilResult{}, err
	}
	outcome, err := machine.stepOver(ctx)
	if err != nil {
		return runUntilResult{}, err
	}
	return runUntilToResult(machine, outcome), nil
}

func (s *Server) stepOut(ctx context.Context, _ mcp.ToolRequest, in machineIDArgs) (runUntilResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return runUntilResult{}, err
	}
	outcome, err := machine.stepOut(ctx)
	if err != nil {
		return runUntilResult{}, err
	}
	return runUntilToResult(machine, outcome), nil
}

func (s *Server) checkpoint(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (checkpointResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return checkpointResult{}, err
	}
	id, _ := machine.checkpoint()
	logFile := machine.eventLogFile()
	path, err := s.writeEventLog(machine.id, logFile)
	if err != nil {
		return checkpointResult{}, err
	}
	return checkpointResult{
		CheckpointID: id,
		EventsFile:   path,
		Frame:        logFile.Frame,
	}, nil
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
		Frame:      in.Frame,
	}
	if err := machine.restore(ctx, target, timeout); err != nil {
		return restoreResult{}, err
	}
	return restoreResult{
		Restored: true,
		Frame:    machine.frame(),
	}, nil
}

func (s *Server) forkMachine(ctx context.Context, _ mcp.ToolRequest, in forkMachineArgs) (createMachineResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return createMachineResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return createMachineResult{}, err
	}
	target := replayTarget{
		Checkpoint: in.CheckpointID,
		Frame:      in.Frame,
	}
	fork, summary, err := machine.forkMachine(ctx, s.logger, target, timeout)
	if err != nil {
		return createMachineResult{}, err
	}
	if err := s.registry.add(fork); err != nil {
		return createMachineResult{}, err
	}
	return createMachineResult{
		MachineID: fork.id,
		ROM:       summary,
		CPU:       fork.cpuState(),
	}, nil
}

func (s *Server) restoreMachine(ctx context.Context, _ mcp.ToolRequest,
	in restoreMachineArgs) (createMachineResult, error) {

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
	logFile, err := readEventLog(eventsPath)
	if err != nil {
		return createMachineResult{}, err
	}

	entrypoint := -1
	if logFile.Entrypoint != nil {
		entrypoint = *logFile.Entrypoint
	}
	machine, summary, err := loadMachine(s.logger, romPath, entrypoint, logFile.Battery, logFile.Palette)
	if err != nil {
		return createMachineResult{}, err
	}
	if summary.PRGCRC32 != logFile.PRGCRC32 || summary.CHRCRC32 != logFile.CHRCRC32 {
		return createMachineResult{}, errors.New("the ROM does not match the ROM of the event log")
	}

	machine.setEvents(logFile.Events)
	target := replayTarget{Frame: &logFile.Frame}
	if err := machine.restore(ctx, target, timeout); err != nil {
		return createMachineResult{}, err
	}
	if err := s.registry.add(machine); err != nil {
		return createMachineResult{}, err
	}
	return createMachineResult{
		MachineID: machine.id,
		ROM:       summary,
		CPU:       machine.cpuState(),
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
func (s *Server) writeEventLog(machineID string, logFile eventLogFile) (string, error) {
	path := filepath.Join(s.roots.first(), machineID+".events.json")
	data, err := json.MarshalIndent(logFile, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding event log: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing event log: %w", err)
	}
	return path, nil
}

// breakpointInfo describes one breakpoint.
type breakpointInfo struct {
	ID      int     `json:"id"`
	Kind    string  `json:"kind"`
	Address Address `json:"address"`
	End     Address `json:"end"`
	Bank    *int    `json:"bank,omitempty"`
	Once    bool    `json:"once"`
	Hits    uint64  `json:"hits"`
}

// instructionOut is one disassembled instruction.
type instructionOut struct {
	Address   Address `json:"address"`
	Bank      int     `json:"bank"`
	PRGOffset int     `json:"prg_offset"`
	Bytes     []int   `json:"bytes"`
	Mnemonic  string  `json:"mnemonic"`
	Operand   string  `json:"operand"`
	Size      int     `json:"size"`
}

// addBreakpointArgs selects the breakpoint.
type addBreakpointArgs struct {
	MachineID string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Kind      string   `json:"kind" description:"exec, read, write, or access. Default exec."`
	Address   Address  `json:"address" description:"Start address."`
	End       *Address `json:"end,omitempty" description:"End of an inclusive range. Default the address."`
	Bank      *int     `json:"bank,omitempty" description:"PRG bank that must match."`
	Value     *int     `json:"value,omitempty" description:"Byte value that a write must match."`
	Once      *bool    `json:"once,omitempty" description:"Remove the breakpoint after its first hit."`
}

// addBreakpointResult reports the new breakpoint.
type addBreakpointResult struct {
	ID int `json:"id"`
}

// removeBreakpointArgs selects a breakpoint.
type removeBreakpointArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	ID        int    `json:"id" description:"Breakpoint id from add_breakpoint."`
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
	MachineID    string     `json:"machine_id" description:"Identifier returned by create_machine."`
	Frames       *uint64    `json:"frames,omitempty" description:"Frame limit."`
	Instructions *uint64    `json:"instructions,omitempty" description:"Instruction limit."`
	Cycles       *uint64    `json:"cycles,omitempty" description:"CPU cycle limit."`
	Timeout      string     `json:"timeout,omitempty" description:"Wall-clock limit. Default 10s."`
	Inputs       []inputArg `json:"inputs,omitempty" description:"Controller changes."`
}

// runUntilResult reports the stop boundary.
type runUntilResult struct {
	Reason          string         `json:"reason"`
	BreakpointID    int            `json:"breakpoint_id,omitempty"`
	HasBreakpoint   bool           `json:"has_breakpoint"`
	HitPC           Address        `json:"hit_pc,omitempty"`
	HitAddress      Address        `json:"hit_address,omitempty"`
	HitValue        int            `json:"hit_value,omitempty"`
	CPU             cpuStateResult `json:"cpu"`
	Trace           []string       `json:"trace,omitempty"`
	Instructions    uint64         `json:"instructions"`
	CompletedFrames uint64         `json:"completed_frames"`
	Frame           uint64         `json:"frame"`
	CPUCycles       uint64         `json:"cpu_cycles"`
	PC              Address        `json:"pc"`
	Error           string         `json:"error,omitempty"`
}

// disassembleArgs selects the code range.
type disassembleArgs struct {
	MachineID string  `json:"machine_id" description:"Identifier returned by create_machine."`
	Address   Address `json:"address" description:"First instruction address."`
	Count     int     `json:"count" description:"Number of instructions."`
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
	Enabled  bool `json:"enabled"`
	Capacity int  `json:"capacity"`
	Lines    int  `json:"lines"`
}

// getTraceArgs selects a page of trace lines.
type getTraceArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Limit     *int   `json:"limit,omitempty" description:"Line count. Default 100."`
	Offset    *int   `json:"offset,omitempty" description:"Start index. Default 0."`
}

// getTraceResult contains a page of trace lines.
type getTraceResult struct {
	Total int      `json:"total"`
	Lines []string `json:"lines"`
}

// callStackResult contains the reconstructed call stack.
type callStackResult struct {
	Frames []callFrame `json:"frames"`
}

// checkpointResult reports a checkpoint.
type checkpointResult struct {
	CheckpointID int    `json:"checkpoint_id"`
	Frame        uint64 `json:"frame"`
	EventsFile   string `json:"events_file"`
}

// restoreArgs selects a checkpoint or a frame to restore.
type restoreArgs struct {
	MachineID    string  `json:"machine_id" description:"Identifier returned by create_machine."`
	CheckpointID *int    `json:"checkpoint_id,omitempty" description:"Checkpoint from the checkpoint tool. Restores the exact state."`
	Frame        *uint64 `json:"frame,omitempty" description:"Frame to replay to, instead of a checkpoint."`
	Timeout      string  `json:"timeout,omitempty" description:"Wall-clock limit for the replay. Default 10s."`
}

// restoreResult reports a restore.
type restoreResult struct {
	Restored bool   `json:"restored"`
	Frame    uint64 `json:"frame"`
}

// forkMachineArgs selects a machine and the point to fork at.
type forkMachineArgs struct {
	MachineID    string  `json:"machine_id" description:"Identifier returned by create_machine."`
	CheckpointID *int    `json:"checkpoint_id,omitempty" description:"Checkpoint to fork at. Default the current state."`
	Frame        *uint64 `json:"frame,omitempty" description:"Frame to replay to, instead of a checkpoint."`
	Timeout      string  `json:"timeout,omitempty" description:"Wall-clock limit for the replay. Default 10s."`
}

// restoreMachineArgs selects a ROM and event log file.
type restoreMachineArgs struct {
	ROM     string `json:"rom" description:"Path to the .nes ROM of the event log, inside the allowed roots."`
	Events  string `json:"events" description:"Path to an event log file inside the allowed roots."`
	Timeout string `json:"timeout,omitempty" description:"Wall-clock limit for the replay. Default 10s."`
}

// eventLogArgs selects a page of events.
type eventLogArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Limit     *int   `json:"limit,omitempty" description:"Event count. Default 100."`
	Offset    *int   `json:"offset,omitempty" description:"Start index. Default 0."`
}

// eventLogResult contains a page of events.
type eventLogResult struct {
	Total  int            `json:"total"`
	Events []machineEvent `json:"events"`
}

// eventLogFile is the stored form of a machine event log. The ROM checksums,
// the entrypoint, and the battery policy let a restore reject a mismatched
// replay setup. The palette bytes let a restore use the same display colors
// when the original palette file is not available.
type eventLogFile struct {
	Version    int            `json:"version"`
	PRGCRC32   uint32         `json:"prg_crc32"`
	CHRCRC32   uint32         `json:"chr_crc32"`
	Entrypoint *int           `json:"entrypoint,omitempty"`
	Battery    bool           `json:"battery"`
	Palette    []byte         `json:"palette,omitempty"`
	Frame      uint64         `json:"frame"`
	Events     []machineEvent `json:"events"`
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
		return 0, 0, fmt.Errorf("limit %d is outside 1 to %d", pageLimit, maxPage)
	}
	if pageOffset < 0 {
		return 0, 0, fmt.Errorf("offset %d is negative", pageOffset)
	}
	return pageLimit, pageOffset, nil
}

// runUntilToResult converts a run outcome into a tool result.
func runUntilToResult(machine *Machine, outcome runUntilOutcome) runUntilResult {
	result := outcome.Result
	out := runUntilResult{
		Reason:          string(result.Reason),
		CPU:             machine.cpuState(),
		Trace:           outcome.Trace,
		Instructions:    result.Instructions,
		CompletedFrames: result.CompletedFrames,
		Frame:           result.Frame,
		CPUCycles:       result.CPUCycles,
		PC:              Address(result.PC),
		Error:           result.Error,
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
func readEventLog(path string) (eventLogFile, error) {
	data, err := os.ReadFile(path)
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
