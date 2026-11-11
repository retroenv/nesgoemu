package nes

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/log"
)

// BreakpointKind identifies what a breakpoint matches.
type BreakpointKind uint8

const (
	// BreakpointExec stops before a CPU instruction at an address executes.
	BreakpointExec BreakpointKind = iota
	// BreakpointRead stops after a read of an address.
	BreakpointRead
	// BreakpointWrite stops after a write to an address.
	BreakpointWrite
	// BreakpointAccess stops after a read or a write of an address.
	BreakpointAccess
)

// BreakpointID identifies a breakpoint in a set.
type BreakpointID int

// Breakpoint describes one execution or memory breakpoint. Start and End
// select an inclusive address range. Bank limits a match to one PRG bank.
// Value limits a write to one byte. Once removes the breakpoint after its
// first hit.
type Breakpoint struct {
	Bank  *int
	End   uint16
	Kind  BreakpointKind
	Once  bool
	Start uint16
	Value *byte
}

// BreakpointStatus reports one breakpoint and its hit count.
type BreakpointStatus struct {
	Breakpoint
	Hits uint64
	ID   BreakpointID
}

// BreakpointHit reports one matched breakpoint. Address and Value describe the
// access. PC is the address of the instruction that made the access.
type BreakpointHit struct {
	Address uint16
	Frame   uint64
	ID      BreakpointID
	Kind    BreakpointKind
	PC      uint16
	Value   byte
}

// breakpointEntry holds one breakpoint and its hit count.
type breakpointEntry struct {
	bp   Breakpoint
	hits uint64
	id   BreakpointID
}

// BreakpointSet is a concurrency-safe list of breakpoints.
type BreakpointSet struct {
	items []breakpointEntry
	mu    sync.Mutex
	next  BreakpointID
}

// RunUntilReason identifies why a bounded run stopped.
type RunUntilReason string

const (
	// StopBreakpoint means a breakpoint matched.
	StopBreakpoint RunUntilReason = "breakpoint"
	// StopFrames means the frame limit was reached.
	StopFrames RunUntilReason = "frames"
	// StopInstructions means the instruction limit was reached.
	StopInstructions RunUntilReason = "instructions"
	// StopCycles means the cycle limit was reached.
	StopCycles RunUntilReason = "cycles"
	// StopTimeout means the wall-clock limit was reached.
	StopTimeout RunUntilReason = "timeout"
	// StopContext means the caller cancelled the run.
	StopContext RunUntilReason = "context"
	// StopInvalidInstruction means the CPU returned an error or panicked.
	StopInvalidInstruction RunUntilReason = "invalid_instruction"
)

// RunUntilOptions defines inputs and independent limits for a debug run.
type RunUntilOptions struct {
	Boot BootPolicy

	Frames          uint64
	MaxCycles       uint64
	MaxInstructions uint64
	Timeout         time.Duration

	Inputs []InputEvent
	// OnInput, when set, runs after the run applies an input event.
	OnInput func(InputEvent)

	Breakpoints *BreakpointSet
	// SkipExecAtStart runs the first instruction without an exec breakpoint
	// check at the current program counter. A caller sets it to continue from
	// a stop at an exec breakpoint, so the run does not stop there again.
	SkipExecAtStart bool
}

// RunUntilResult contains the state at the stop boundary.
type RunUntilResult struct {
	HasHit bool
	Hit    BreakpointHit
	Reason RunUntilReason

	CompletedFrames uint64
	CPUCycles       uint64
	Error           string
	Frame           uint64
	Instructions    uint64
	PC              uint16
	SP              uint8
}

// NewBreakpointSet creates an empty breakpoint set.
func NewBreakpointSet() *BreakpointSet {
	return &BreakpointSet{}
}

// String returns the name of the breakpoint kind.
func (k BreakpointKind) String() string {
	switch k {
	case BreakpointExec:
		return "exec"
	case BreakpointRead:
		return "read"
	case BreakpointWrite:
		return "write"
	case BreakpointAccess:
		return "access"
	default:
		return "unknown"
	}
}

// Add stores a breakpoint and returns its identifier.
func (s *BreakpointSet) Add(bp Breakpoint) BreakpointID {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.next++
	s.items = append(s.items, breakpointEntry{
		bp: bp,
		id: s.next,
	})

	return s.next
}

// Remove deletes a breakpoint. It reports false when the identifier is unknown.
func (s *BreakpointSet) Remove(id BreakpointID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index, entry := range s.items {
		if entry.id == id {
			s.items = append(s.items[:index], s.items[index+1:]...)
			return true
		}
	}

	return false
}

// List returns the breakpoints and their hit counts.
func (s *BreakpointSet) List() []BreakpointStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := make([]BreakpointStatus, 0, len(s.items))
	for _, entry := range s.items {
		status = append(status, BreakpointStatus{
			Breakpoint: entry.bp,
			Hits:       entry.hits,
			ID:         entry.id,
		})
	}

	return status
}

// Len returns the number of breakpoints.
func (s *BreakpointSet) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.items)
}

// RunUntil executes with breakpoints until a stop condition is reached.
func (sys *System) RunUntil(ctx context.Context, opts RunUntilOptions) RunUntilResult {
	result, _ := sys.runUntil(ctx, opts)
	return result
}

func (s *BreakpointSet) matchExec(pc uint16, bank int, frame uint64) (BreakpointHit, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.items {
		entry := &s.items[index]
		if entry.bp.Kind != BreakpointExec || !entry.matches(pc, bank, 0, false) {
			continue
		}
		hit := entry.record(pc, 0, pc, frame)
		s.removeOnce(index, entry)

		return hit, true
	}

	return BreakpointHit{}, false
}

func (s *BreakpointSet) matchMemory(write bool, address uint16, value byte, bank int, pc uint16,
	frame uint64) (BreakpointHit, bool) {

	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.items {
		entry := &s.items[index]
		if !matchesKind(entry.bp.Kind, write) || !entry.matches(address, bank, value, write) {
			continue
		}
		hit := entry.record(address, value, pc, frame)
		s.removeOnce(index, entry)

		return hit, true
	}

	return BreakpointHit{}, false
}

func (s *BreakpointSet) removeOnce(index int, entry *breakpointEntry) {
	if entry.bp.Once {
		s.items = append(s.items[:index], s.items[index+1:]...)
	}
}

// runUntil drives the run loop and converts a panic into a stop reason.
func (sys *System) runUntil(ctx context.Context, opts RunUntilOptions) (result RunUntilResult, err error) {
	var run untilRun
	run.startCycles = sys.CPU.Cycles()
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic at PC 0x%04x: %v", sys.PC, recovered)
			result = run.finish(sys, run.current, StopInvalidInstruction, err)
			sys.opts.logger.Error("Emulator panic", log.Err(err))
		}
	}()

	if opts.Boot == BootReset {
		sys.Reset()
		run.startCycles = sys.CPU.Cycles()
	}

	runContext := ctx
	cancel := func() {}
	if opts.Timeout > 0 {
		runContext, cancel = context.WithTimeout(ctx, opts.Timeout)
	}

	defer cancel()

	inputIndex := applyInputEvents(sys, opts.Inputs, 0, 0, opts.OnInput)
	defer sys.observeBreakpoints(opts.Breakpoints, &run.current, &run.instructionPC)()

	skipExec := opts.SkipExecAtStart
	for {
		cycles := sys.CPU.Cycles() - run.startCycles
		reason, runErr, stop := checkUntilLimits(runContext, opts, cycles, run.instructions, run.completedFrames)
		if stop {
			return run.finish(sys, run.current, reason, runErr), nil
		}

		if !skipExec {
			if execHit, ok := sys.execBreakpoint(opts.Breakpoints); ok {
				return run.finish(sys, execHit, StopBreakpoint, nil), nil
			}
		}

		skipExec = false

		run.instructionPC = sys.PC
		step, stepErr := sys.StepTraced()
		if stepErr != nil {
			return run.finish(sys, run.current, StopInvalidInstruction, stepErr), nil
		}
		if step.InstructionExecuted {
			run.instructions++
		}

		if step.FrameCompleted {
			run.completedFrames++
			inputIndex = applyInputEvents(sys, opts.Inputs, inputIndex, run.completedFrames, opts.OnInput)
		}

		if run.current.HasHit {
			return run.finish(sys, run.current, StopBreakpoint, nil), nil
		}
	}
}

// observeBreakpoints installs the observer for read and write breakpoints.
// The run loop sets instructionPC to the address of the running instruction.
func (sys *System) observeBreakpoints(breakpoints *BreakpointSet, current *RunUntilResult,
	instructionPC *uint16) func() {

	if breakpoints == nil {
		return func() {}
	}

	return sys.AddBusCycleObserver(func(cycle cpu6502.BusCycle) {
		if current.HasHit {
			return
		}
		bank, _, _ := sys.PRGLocation(cycle.Address)
		hit, ok := breakpoints.matchMemory(cycle.Write, cycle.Address, cycle.Value, bank, *instructionPC,
			sys.Bus.PPU.Frame())
		if ok {
			*current = RunUntilResult{
				HasHit: true,
				Hit:    hit,
			}
		}
	})
}

// execBreakpoint returns an exec breakpoint at the current program counter.
func (sys *System) execBreakpoint(breakpoints *BreakpointSet) (RunUntilResult, bool) {
	if breakpoints == nil {
		return RunUntilResult{}, false
	}
	bank, _, _ := sys.PRGLocation(sys.PC)
	hit, ok := breakpoints.matchExec(sys.PC, bank, sys.Bus.PPU.Frame())
	if !ok {
		return RunUntilResult{}, false
	}
	return RunUntilResult{
		HasHit: true,
		Hit:    hit,
	}, true
}

// untilRun holds the counters of one RunUntil call.
type untilRun struct {
	completedFrames uint64
	current         RunUntilResult
	instructionPC   uint16
	instructions    uint64
	startCycles     uint64
}

// finish builds the result at the stop boundary.
func (r *untilRun) finish(sys *System, hit RunUntilResult, reason RunUntilReason, runErr error) RunUntilResult {
	return finishUntil(sys, hit, reason, runErr, r.startCycles, r.instructions, r.completedFrames)
}

// matches reports whether the entry matches one access.
func (e *breakpointEntry) matches(address uint16, bank int, value byte, write bool) bool {
	if address < e.bp.Start || address > e.bp.End {
		return false
	}
	if e.bp.Bank != nil && (bank < 0 || bank != *e.bp.Bank) {
		return false
	}
	if write && e.bp.Value != nil && value != *e.bp.Value {
		return false
	}
	return true
}

// record increments the hit count and builds a hit.
func (e *breakpointEntry) record(address uint16, value byte, pc uint16, frame uint64) BreakpointHit {
	e.hits++
	return BreakpointHit{
		Address: address,
		Frame:   frame,
		ID:      e.id,
		Kind:    e.bp.Kind,
		PC:      pc,
		Value:   value,
	}
}

// matchesKind reports whether a memory breakpoint kind matches a write flag.
func matchesKind(kind BreakpointKind, write bool) bool {
	switch kind {
	case BreakpointRead:
		return !write
	case BreakpointWrite:
		return write
	case BreakpointAccess:
		return true
	default:
		return false
	}
}

// checkUntilLimits reports whether an independent run limit was reached.
func checkUntilLimits(runContext context.Context, opts RunUntilOptions, cycles, instructions,
	completedFrames uint64) (RunUntilReason, error, bool) {

	if err := runContext.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return StopTimeout, fmt.Errorf("run timed out: %w", err), true
		}
		return StopContext, fmt.Errorf("run context stopped: %w", err), true
	}
	if opts.MaxCycles > 0 && cycles >= opts.MaxCycles {
		return StopCycles, nil, true
	}
	if opts.MaxInstructions > 0 && instructions >= opts.MaxInstructions {
		return StopInstructions, nil, true
	}
	if opts.Frames > 0 && completedFrames >= opts.Frames {
		return StopFrames, nil, true
	}
	return "", nil, false
}

// finishUntil builds the run result at the stop boundary.
func finishUntil(sys *System, current RunUntilResult, reason RunUntilReason, runErr error, startCycles, instructions,
	completedFrames uint64) RunUntilResult {

	current.Reason = reason
	current.CPUCycles = sys.CPU.Cycles() - startCycles
	current.Instructions = instructions
	current.CompletedFrames = completedFrames
	current.Frame = sys.Bus.PPU.Frame()
	current.PC = sys.PC
	current.SP = sys.SP
	if runErr != nil {
		current.Error = runErr.Error()
	}

	return current
}
