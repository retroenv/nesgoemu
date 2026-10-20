package mcpserver

import (
	"context"
	"fmt"
	"hash/crc32"
	"time"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/log"
)

// stepOverLimit bounds a step-over or step-out run.
const stepOverLimit = 1_000_000

// stopReturned is the stop reason of a step-over or step-out run that reached
// the return address.
const stopReturned nes.RunUntilReason = "returned"

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

	opts.Breakpoints = m.breakpoints
	opts.Timeout = timeout
	opts.Inputs = toInputEvents(inputs)
	opts.OnInput = m.recordAppliedInput
	before := m.traceRing.Total()

	result := m.sys.RunUntil(ctx, opts)
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

// callFrames returns the reconstructed call stack.
func (m *Machine) callFrames() []callFrame {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.callStack.framesCopy()
}

// stepOver runs the current call or a single instruction.
func (m *Machine) stepOver(ctx context.Context) (runUntilOutcome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	instructions := m.sys.Disassemble(m.sys.PC, 1)
	if len(instructions) == 0 || instructions[0].Mnemonic != "JSR" {
		return m.singleStep()
	}
	next := m.sys.PC + uint16(instructions[0].Size)
	return m.runToReturn(ctx, next, int(m.sys.SP))
}

// stepOut runs until the current function returns.
func (m *Machine) stepOut(ctx context.Context) (runUntilOutcome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	frames := m.callStack.framesCopy()
	if len(frames) == 0 {
		return m.singleStep()
	}
	// The return address is on the stack, so the return completes when the
	// stack pointer rises above its current value.
	return m.runToReturn(ctx, uint16(frames[len(frames)-1].ReturnAddress), int(m.sys.SP)+1)
}

// singleStep executes one instruction and captures the trace tail.
func (m *Machine) singleStep() (runUntilOutcome, error) {
	before := m.traceRing.Total()
	result := nes.RunUntilResult{Reason: nes.StopInstructions}
	err := m.sys.Guard(func() error {
		if _, stepErr := m.sys.StepTraced(); stepErr != nil {
			return fmt.Errorf("executing step: %w", stepErr)
		}
		return nil
	})
	if err != nil {
		return runUntilOutcome{}, fmt.Errorf("stepping: %w", err)
	}
	result.Instructions = 1
	result.PC = m.sys.PC
	result.SP = m.sys.SP
	result.Frame = m.sys.Bus.PPU.Frame()
	return runUntilOutcome{
		Result: result,
		Trace:  m.traceRing.LinesSince(before),
	}, nil
}

// runToReturn runs until the program counter reaches the address with a stack
// pointer of at least returnedSP. A recursive call that returns to the same
// address on a deeper stack level does not end the run. The machine
// breakpoints stay active, so a user breakpoint stops the run first.
func (m *Machine) runToReturn(ctx context.Context, address uint16, returnedSP int) (runUntilOutcome, error) {
	before := m.traceRing.Total()
	temporary := m.breakpoints.Add(nes.Breakpoint{
		Kind:  nes.BreakpointExec,
		Start: address,
		End:   address,
	})
	defer m.breakpoints.Remove(temporary)

	deadline := time.Now().Add(defaultRunTimeout)
	remaining := uint64(stepOverLimit)
	var result nes.RunUntilResult

	for {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			result.Reason = nes.StopTimeout
			break
		}
		result = m.sys.RunUntil(ctx, nes.RunUntilOptions{
			Breakpoints:     m.breakpoints,
			MaxInstructions: remaining,
			Timeout:         timeout,
		})
		if result.Reason != nes.StopBreakpoint || result.Hit.ID != temporary {
			break
		}
		if int(m.sys.SP) >= returnedSP {
			result.HasHit = false
			result.Reason = stopReturned
			break
		}
		// The address was reached on a deeper stack level; continue the run.
		if result.Instructions >= remaining {
			result.Reason = nes.StopInstructions
			break
		}
		remaining -= result.Instructions
		if _, err := m.stepPastBreakpoint(); err != nil {
			return runUntilOutcome{}, err
		}
	}
	return runUntilOutcome{
		Result: result,
		Trace:  m.traceRing.LinesSince(before),
	}, nil
}

// stepPastBreakpoint executes one instruction so that a repeated run does not
// stop again at the same exec breakpoint.
func (m *Machine) stepPastBreakpoint() (nes.StepResult, error) {
	var step nes.StepResult
	err := m.sys.Guard(func() error {
		var stepErr error
		step, stepErr = m.sys.StepTraced()
		if stepErr != nil {
			return fmt.Errorf("executing step: %w", stepErr)
		}
		return nil
	})
	if err != nil {
		return step, fmt.Errorf("stepping: %w", err)
	}
	return step, nil
}

// checkpoint names the current point of the event log.
func (m *Machine) checkpoint() (int, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.addCheckpoint()
}

// forkMachine creates a new machine that replays the log to a checkpoint or a
// frame.
func (m *Machine) forkMachine(ctx context.Context, logger *log.Logger, target replayTarget,
	timeout time.Duration) (*Machine, romSummary, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	events, frame, err := m.resolveTarget(target)
	if err != nil {
		return nil, romSummary{}, err
	}

	ring := nes.NewTraceRing(0)
	sys, summary, err := loadSystem(logger, m.rom, m.entrypoint, m.battery, m.palette, ring)
	if err != nil {
		return nil, romSummary{}, err
	}
	if err := replayTo(ctx, sys, events, frame, timeout); err != nil {
		return nil, romSummary{}, err
	}

	id, err := newMachineID()
	if err != nil {
		return nil, romSummary{}, err
	}
	fork := newMachine(id, m.rom, sys, m.entrypoint, m.battery, m.palette)
	fork.logger = logger
	fork.traceRing = ring
	fork.breakpoints = nes.NewBreakpointSet()
	fork.search = newMemorySearch()
	fork.events = events
	fork.attach()
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
			Bytes:     bytesToInts(instruction.Bytes),
			Mnemonic:  instruction.Mnemonic,
			Operand:   instruction.Operand,
			Size:      instruction.Size,
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
		Enabled:  m.traceRing.Enabled(),
		Capacity: m.traceRing.Capacity(),
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
		return getTraceResult{Total: total}
	}
	end := min(offset+limit, total)
	return getTraceResult{
		Total: total,
		Lines: lines[offset:end],
	}
}

// eventLogPage returns a page of recorded events.
func (m *Machine) eventLogPage(limit, offset int) eventLogResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	return eventLogResult{
		Total:  len(m.events),
		Events: m.eventLog(limit, offset),
	}
}

// eventLogFile returns the event log with the data that a restore needs to
// check.
func (m *Machine) eventLogFile() eventLogFile {
	m.mu.Lock()
	defer m.mu.Unlock()

	logFile := eventLogFile{
		Version:  eventLogVersion,
		PRGCRC32: crc32.ChecksumIEEE(m.sys.Bus.Cartridge.PRG),
		CHRCRC32: crc32.ChecksumIEEE(m.sys.Bus.Cartridge.CHR),
		Battery:  m.battery,
		Palette:  m.palette,
		Frame:    m.sys.Bus.PPU.Frame(),
		Events:   append([]machineEvent(nil), m.events...),
	}
	if m.entrypoint >= 0 {
		entrypoint := m.entrypoint
		logFile.Entrypoint = &entrypoint
	}
	return logFile
}

// setEvents replaces the recorded events.
func (m *Machine) setEvents(events []machineEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = events
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
