package mcpserver

import (
	"context"
	"fmt"
	"hash/crc32"
	"slices"
	"strings"
	"time"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/log"
)

// stepOverLimit bounds a step-over or step-out run.
const stepOverLimit = 1_000_000

// stopReturned is the stop reason of a step-over or step-out run that reached
// the return address.
const (
	stopReturned   nes.RunUntilReason = "returned"
	stopNoProgress nes.RunUntilReason = "no_progress"
	stopStopped    nes.RunUntilReason = "stopped"
	stopEventLimit nes.RunUntilReason = "event_limit"
)

// addBreakpoint stores a breakpoint on the machine.
func (m *Machine) addBreakpoint(breakpoint nes.Breakpoint) nes.BreakpointID {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.breakpoints.Add(breakpoint)
}

// removeBreakpoint deletes a breakpoint from the machine.
func (m *Machine) removeBreakpoint(id nes.BreakpointID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.breakpoints.Remove(id)
}

// listBreakpoints returns the machine breakpoints.
func (m *Machine) listBreakpoints() []nes.BreakpointStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.breakpoints.List()
}

// runUntil runs with the machine breakpoints and returns the trace tail. The
// trace tail is empty when the trace ring is off.
func (m *Machine) runUntil(ctx context.Context, opts nes.RunUntilOptions, inputs []controllerInput,
	timeout time.Duration) (runUntilOutcome, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkHistoryCapacity(len(inputs), 0); err != nil {
		return runUntilOutcome{}, err
	}
	opts.Breakpoints = m.breakpoints
	opts.Timeout = timeout
	opts.Inputs = toInputEvents(inputs)
	opts.OnInput = m.recordAppliedInput
	opts.SkipExecAtStart = m.resumingFromBreakpoint()
	before := m.traceRing.Total()

	result := m.sys.RunUntil(ctx, opts)
	m.noteStop(result, 0)

	return runUntilOutcome{
		Result: result,
		Trace:  m.traceRing.LinesSince(before),
	}, nil
}

// recordAppliedInput records an input that a run applied. The caller holds the
// machine mutex.
func (m *Machine) recordAppliedInput(event nes.InputEvent) {
	m.recordInput(max(event.Controller, 1), event.Buttons)
}

// resumingFromBreakpoint reports whether the machine stopped at an exec
// breakpoint at its current program counter. A run from there must execute that
// instruction first, or it would stop at the same breakpoint again.
func (m *Machine) resumingFromBreakpoint() bool {
	return m.stoppedAtExec && m.stopPC == m.sys.PC
}

// noteStop records whether a run stopped at an exec breakpoint. The breakpoint
// with the identifier ignored is a temporary one of the server and does not
// count.
func (m *Machine) noteStop(result nes.RunUntilResult, ignored nes.BreakpointID) {
	m.stoppedAtExec = result.HasHit && result.Hit.Kind == nes.BreakpointExec && result.Hit.ID != ignored
	m.stopPC = result.PC
}

// callFrames returns the reconstructed call stack.
func (m *Machine) callFrames() []callFrame {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.callStack.framesCopy()
}

// stepOver runs the current call or a single instruction.
func (m *Machine) stepOver(ctx context.Context, timeout time.Duration) (runUntilOutcome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instructions := m.sys.Disassemble(m.sys.PC, 1)
	if len(instructions) == 0 || !strings.EqualFold(instructions[0].Mnemonic, cpu6502.JsrName) {
		return m.singleStep(ctx, timeout)
	}
	next := m.sys.PC + uint16(instructions[0].Size)

	return m.runToReturn(ctx, next, int(m.sys.SP), timeout)
}

// stepOut runs until the current function returns.
func (m *Machine) stepOut(ctx context.Context, timeout time.Duration) (runUntilOutcome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	frames := m.callStack.framesCopy()
	if len(frames) == 0 {
		return m.singleStep(ctx, timeout)
	}
	// The return address is on the stack, so the return completes when the
	// stack pointer rises above its current value.
	return m.runToReturn(ctx, uint16(frames[len(frames)-1].ReturnAddress), int(m.sys.SP)+1, timeout)
}

// singleStep executes one instruction and captures the trace tail.
func (m *Machine) singleStep(ctx context.Context, timeout time.Duration) (runUntilOutcome, error) {
	before := m.traceRing.Total()
	m.stoppedAtExec = false
	result := m.sys.RunUntil(ctx, nes.RunUntilOptions{
		MaxInstructions: 1,
		Timeout:         timeout,
	})

	return runUntilOutcome{
		Result: result,
		Trace:  m.traceRing.LinesSince(before),
	}, nil
}

// runToReturn runs until the program counter reaches the address with a stack
// pointer of at least returnedSP. A recursive call that returns to the same
// address on a deeper stack level does not end the run. The machine
// breakpoints stay active, so a user breakpoint stops the run first.
func (m *Machine) runToReturn(ctx context.Context, address uint16, returnedSP int,
	timeout time.Duration) (runUntilOutcome, error) {

	before := m.traceRing.Total()
	temporary := m.breakpoints.Add(nes.Breakpoint{
		End:   address,
		Kind:  nes.BreakpointExec,
		Start: address,
	})
	defer m.breakpoints.Remove(temporary)

	deadline := time.Now().Add(timeout)
	startCycles := m.sys.CPU.Cycles()
	startFrame := m.sys.Bus.PPU.Frame()
	instructions := uint64(0)
	skipExec := m.resumingFromBreakpoint()
	var result nes.RunUntilResult
	defer func() { m.noteStop(result, temporary) }()

	for {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			result.Reason = nes.StopTimeout
			break
		}
		result = m.sys.RunUntil(ctx, nes.RunUntilOptions{
			MaxInstructions: uint64(stepOverLimit) - instructions,
			Timeout:         timeout,

			Breakpoints:     m.breakpoints,
			SkipExecAtStart: skipExec,
		})
		instructions += result.Instructions
		if result.Reason != nes.StopBreakpoint || result.Hit.ID != temporary {
			break
		}
		if int(m.sys.SP) >= returnedSP {
			result.HasHit = false
			result.Reason = stopReturned
			break
		}
		// The address was reached on a deeper stack level; continue the run.
		if instructions >= uint64(stepOverLimit) {
			result.Reason = nes.StopInstructions
			break
		}
		skipExec = true
	}

	result.Instructions = instructions
	result.CPUCycles = m.sys.CPU.Cycles() - startCycles
	result.Frame = m.sys.Bus.PPU.Frame()
	result.CompletedFrames = result.Frame - startFrame
	result.PC = m.sys.PC
	result.SP = m.sys.SP

	return runUntilOutcome{
		Result: result,
		Trace:  m.traceRing.LinesSince(before),
	}, nil
}

// checkpoint names the current point of the event log.
func (m *Machine) checkpoint() (checkpointResult, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.checkpoints) >= maxCheckpoints {
		return checkpointResult{}, nil, fmt.Errorf("checkpoint limit %d reached; use a hard reset or create a new machine", maxCheckpoints)
	}
	logFile := m.eventLogFileLocked()
	data, err := encodeEventLog(logFile, maxEventLogFileBytes)
	if err != nil {
		return checkpointResult{}, nil, err
	}
	id, frame := m.addCheckpoint()

	return checkpointResult{
		CheckpointID: id,
		Frame:        frame,
	}, data, nil
}

func (m *Machine) captureCheckpoint() *checkpointSnapshot {
	data, err := m.sys.SaveState()
	if err != nil || len(data) > nes.MaxStateSize {
		return nil
	}

	return &checkpointSnapshot{
		data: data,

		frames:    m.callStack.framesCopy(),
		lastCycle: m.callStack.lastCycle,
	}
}

func (m *Machine) loadTarget(ctx context.Context, logger *log.Logger, target replayTarget, events []machineEvent,
	point replayPoint, timeout time.Duration, ring *nes.TraceRing) (*nes.System, romSummary, *callStackRecorder, error) {

	load := func() (*nes.System, romSummary, error) {
		return loadSystem(ctx, logger, m.rom, m.romHash, m.entrypoint, m.battery, m.palette, ring, &m.batteryData)
	}
	sys, summary, err := load()
	if err != nil {
		return nil, romSummary{}, nil, err
	}

	if target.Checkpoint != nil {
		if snapshot := m.checkpoints[*target.Checkpoint-1].snapshot; snapshot != nil {
			if err := sys.LoadState(snapshot.data); err == nil {
				stack := newCallStackRecorder(sys)
				stack.frames = slices.Clone(snapshot.frames)
				stack.lastCycle = snapshot.lastCycle

				return sys, summary, stack, nil
			}

			sys, summary, err = load()
			if err != nil {
				return nil, romSummary{}, nil, err
			}
		}
	}

	stack := newCallStackRecorder(sys)
	if err := replayTo(ctx, sys, events, point, timeout, stack.clear); err != nil {
		return nil, romSummary{}, nil, err
	}

	return sys, summary, stack, nil
}

// forkMachine creates a new machine with the state at the selected point.
func (m *Machine) forkMachine(ctx context.Context, logger *log.Logger, target replayTarget,
	timeout time.Duration) (*Machine, romSummary, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	events, point, err := m.resolveTarget(target)
	if err != nil {
		return nil, romSummary{}, err
	}

	ring := nes.NewTraceRing(0)
	sys, summary, stack, err := m.loadTarget(ctx, logger, target, events, point, timeout, ring)
	if err != nil {
		return nil, romSummary{}, err
	}

	id, err := newHandleID()
	if err != nil {
		return nil, romSummary{}, err
	}
	fork := newMachine(id, m.rom, sys, m.entrypoint, m.battery, m.palette)
	fork.romHash = m.romHash
	fork.batteryData = m.batteryData
	fork.logger = logger
	fork.traceRing = ring
	fork.breakpoints = nes.NewBreakpointSet()
	fork.events = events
	fork.historyBytes = m.historyBytes
	if target.Checkpoint != nil || target.Frame != nil {
		fork.historyBytes, err = historyBytes(events)
		if err != nil {
			return nil, romSummary{}, err
		}
	}

	fork.attach(stack)

	return fork, summary, nil
}

// disassemble decodes instructions from a CPU address.
func (m *Machine) disassemble(address uint16, count int) []instructionOut {
	m.mu.Lock()
	defer m.mu.Unlock()

	instructions := m.sys.Disassemble(address, count)
	out := make([]instructionOut, 0, len(instructions))
	for _, instruction := range instructions {
		out = append(out, instructionOut{
			Address:   Address(instruction.Location.Address),
			Bank:      instruction.Location.Bank,
			PRGOffset: instruction.Location.PRGOffset,

			Bytes:    bytesToInts(instruction.Bytes),
			Mnemonic: instruction.Mnemonic,
			Operand:  instruction.Operand,
			Size:     instruction.Size,
		})
	}

	return out
}

// traceStart clears and starts the trace ring.
func (m *Machine) traceStart() traceState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.traceRing.Clear()
	m.traceRing.SetEnabled(true)

	return m.traceStateLocked()
}

// traceStop stops the trace ring.
func (m *Machine) traceStop() traceState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.traceRing.SetEnabled(false)

	return m.traceStateLocked()
}

// traceStateLocked reports the trace ring state.
func (m *Machine) traceStateLocked() traceState {
	return traceState{
		Capacity: m.traceRing.Capacity(),
		Enabled:  m.traceRing.Enabled(),
		Lines:    m.traceRing.Len(),
	}
}

// getTrace returns a page of trace lines.
func (m *Machine) getTrace(limit, offset int) getTraceResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	lines := m.traceRing.Lines()
	total := len(lines)
	if offset >= total || limit <= 0 {
		return getTraceResult{
			Total: total,
		}
	}
	end := min(offset+limit, total)

	return getTraceResult{
		Lines: lines[offset:end],
		Total: total,
	}
}

// eventLogPage returns a page of recorded events.
func (m *Machine) eventLogPage(limit, offset int) eventLogResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	return eventLogResult{
		Events: m.eventLog(limit, offset),
		Total:  len(m.events),
	}
}

// eventLogFileLocked returns the event log and the replay data. The caller
// holds the machine mutex.
func (m *Machine) eventLogFileLocked() eventLogFile {
	logFile := eventLogFile{
		Version: eventLogVersion,

		CHRCRC32: crc32.ChecksumIEEE(m.sys.Bus.Cartridge.CHR),
		PRGCRC32: crc32.ChecksumIEEE(m.sys.Bus.Cartridge.PRG),
		ROMHash:  m.romHash,

		Battery:         m.battery,
		BatteryData:     m.batteryData,
		BatterySnapshot: true,
		Palette:         m.palette,

		Cycles: m.sys.CPU.Cycles(),
		Events: append([]machineEvent(nil), m.events...),
		Frame:  m.sys.Bus.PPU.Frame(),
	}
	if m.entrypoint >= 0 {
		entrypoint := m.entrypoint
		logFile.Entrypoint = &entrypoint
	}

	return logFile
}

type checkpointSnapshot struct {
	data []byte

	frames    []callFrame
	lastCycle uint64
}

// runUntilOutcome contains a run result and the new trace lines.
type runUntilOutcome struct {
	Result nes.RunUntilResult
	Trace  []string
}

// toInputEvents converts controller inputs into run input events.
func toInputEvents(inputs []controllerInput) []nes.InputEvent {
	events := make([]nes.InputEvent, 0, len(inputs))
	for _, input := range inputs {
		events = append(events, nes.InputEvent{
			Frame:      input.Frame,
			Controller: input.Controller,
			Buttons:    input.Buttons,
		})
	}

	return events
}

// eventsUpTo returns a copy of the events up to and including a frame.
func eventsUpTo(events []machineEvent, frame uint64) []machineEvent {
	kept := make([]machineEvent, 0, len(events))
	for _, event := range events {
		if event.Frame > frame {
			break
		}
		kept = append(kept, event)
	}

	return kept
}
