package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
)

// Event kinds recorded in the machine event log.
const (
	eventInput = "input"
	eventPoke  = "poke"
	eventReset = "reset"
)

// Search filter operations.
const (
	searchEqual     = "equal"
	searchChanged   = "changed"
	searchUnchanged = "unchanged"
	searchIncreased = "increased"
	searchDecreased = "decreased"
	searchValue     = "value"
)

// Constants for the debug tools.
const (
	ramSize   = 0x800
	jsrOpcode = 0x20

	// jsrSize and brkSize are the sizes of the instructions that push a return
	// address.
	jsrSize = 3
	brkSize = 2

	// jsrPushSize and interruptPushSize are the bytes that a JSR and an
	// interrupt push on the stack.
	jsrPushSize       = 2
	interruptPushSize = 3

	// replayCheckInterval is the number of replay steps between two checks of
	// the context and the deadline.
	replayCheckInterval = 4096
)

// recordEvent appends an event with the current frame and cycle count. The
// caller holds the machine mutex.
func (m *Machine) recordEvent(event machineEvent) {
	event.Frame = m.sys.Bus.PPU.Frame()
	event.Cycles = m.sys.CPU.Cycles()
	m.events = append(m.events, event)
}

// recordInput appends a controller input event. An empty button list records
// the release of the controller.
func (m *Machine) recordInput(controllerID int, buttons controller.Button) {
	id := controllerID
	m.recordEvent(machineEvent{
		Kind:       eventInput,
		Controller: &id,
		Buttons:    buttonNameList(buttons),
	})
}

// recordPoke appends a memory write event.
func (m *Machine) recordPoke(address Address, data []byte) {
	values := make([]int, len(data))
	for index, value := range data {
		values[index] = int(value)
	}
	addr := address
	m.recordEvent(machineEvent{
		Kind:    eventPoke,
		Address: &addr,
		Bytes:   values,
	})
}

// recordReset appends a reset event.
func (m *Machine) recordReset() {
	m.recordEvent(machineEvent{Kind: eventReset})
}

// eventLog returns a page of the recorded events.
func (m *Machine) eventLog(limit, offset int) []machineEvent {
	if offset >= len(m.events) {
		return nil
	}
	end := min(offset+limit, len(m.events))
	return append([]machineEvent(nil), m.events[offset:end]...)
}

// addCheckpoint names the current point of the event log and returns its
// identifier. The caller holds the machine mutex.
func (m *Machine) addCheckpoint() (int, uint64) {
	frame := m.sys.Bus.PPU.Frame()
	m.checkpoints = append(m.checkpoints, checkpointRecord{
		Frame:  frame,
		Events: len(m.events),
	})
	return len(m.checkpoints), frame
}

// resolveTarget returns the events and the frame that a target selects. An empty
// target selects the current frame. The caller holds the machine mutex.
func (m *Machine) resolveTarget(target replayTarget) ([]machineEvent, uint64, error) {
	switch {
	case target.Checkpoint != nil && target.Frame != nil:
		return nil, 0, errors.New("use either checkpoint_id or frame, not both")

	case target.Checkpoint != nil:
		id := *target.Checkpoint
		if id < 1 || id > len(m.checkpoints) {
			return nil, 0, fmt.Errorf("unknown checkpoint_id %d; the machine has %d checkpoints", id, len(m.checkpoints))
		}
		record := m.checkpoints[id-1]
		if record.Events > len(m.events) {
			return nil, 0, fmt.Errorf("checkpoint_id %d is not part of the current event log", id)
		}
		return slices.Clone(m.events[:record.Events]), record.Frame, nil

	case target.Frame != nil:
		return eventsUpTo(m.events, *target.Frame), *target.Frame, nil

	default:
		return slices.Clone(m.events), m.sys.Bus.PPU.Frame(), nil
	}
}

// restore replays the event log to a checkpoint or a frame and replaces the
// system. The events after the target belong to the discarded timeline and are
// dropped, with the checkpoints that point into them.
func (m *Machine) restore(ctx context.Context, target replayTarget, timeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if target.Checkpoint == nil && target.Frame == nil {
		return errors.New("restore needs a checkpoint_id or a frame")
	}
	events, frame, err := m.resolveTarget(target)
	if err != nil {
		return err
	}

	sys, _, err := loadSystem(m.logger, m.rom, m.entrypoint, m.battery, m.palette, m.traceRing)
	if err != nil {
		return err
	}
	if err := replayTo(ctx, sys, events, frame, timeout); err != nil {
		return err
	}
	m.sys = sys
	m.events = events
	m.checkpoints = slices.DeleteFunc(m.checkpoints, func(record checkpointRecord) bool {
		return record.Events > len(events) || record.Frame > frame
	})
	m.bankWrites = nil
	m.search = newMemorySearch()
	m.attach()
	return nil
}

// attach connects the recorders and observers to the current system.
func (m *Machine) attach() {
	m.cdl = m.sys.NewCDLRecorder()
	m.callStack = newCallStackRecorder(m.sys)
	m.sys.ObserveCPUWrites(m.observeWrite)
}

// applyInputs applies the inputs of a frame and records them. The caller holds
// the machine mutex.
func (m *Machine) applyInputs(inputs []controllerInput, index int, frame uint64) int {
	for index < len(inputs) && inputs[index].Frame == frame {
		input := inputs[index]
		device := controllerDevice(m.sys.Bus.Controller1)
		if input.Controller == 2 {
			device = m.sys.Bus.Controller2
		}
		device.SetButtonState(controller.Button(0xff), false)
		device.SetButtonState(input.Buttons, true)
		m.recordInput(input.Controller, input.Buttons)
		index++
	}
	return index
}

// searchStart snapshots internal RAM as the initial candidate set.
func (m *Machine) searchStart() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	candidates := make([]memorySearchEntry, 0, ramSize)
	for address := range ramSize {
		value, ok := m.sys.InspectRAM(uint16(address))
		if !ok {
			continue
		}
		candidates = append(candidates, memorySearchEntry{
			Address: uint16(address),
			Value:   value,
		})
	}
	m.search = newMemorySearch()
	m.search.active = true
	m.search.candidates = candidates
	return len(candidates)
}

// searchFilter narrows the candidate set with one operation.
func (m *Machine) searchFilter(operation string, value int) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.search == nil || !m.search.active {
		return -1
	}
	matches := make([]memorySearchEntry, 0, len(m.search.candidates))
	for _, entry := range m.search.candidates {
		current, ok := m.sys.InspectRAM(entry.Address)
		if !ok {
			continue
		}
		if searchMatches(operation, entry.Value, current, byte(value)) {
			matches = append(matches, memorySearchEntry{
				Address: entry.Address,
				Value:   current,
			})
		}
	}
	m.search.candidates = matches
	return len(matches)
}

// searchCandidates returns a page of the current candidates.
func (m *Machine) searchCandidates(limit, offset int) []memorySearchEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.search == nil || offset >= len(m.search.candidates) {
		return nil
	}
	end := min(offset+limit, len(m.search.candidates))
	return append([]memorySearchEntry(nil), m.search.candidates[offset:end]...)
}

// ramSnapshot copies internal RAM into a map.
func (m *Machine) ramSnapshot() map[uint16]byte {
	snapshot := make(map[uint16]byte, ramSize)
	for address := range ramSize {
		value, ok := m.sys.InspectRAM(uint16(address))
		if ok {
			snapshot[uint16(address)] = value
		}
	}
	return snapshot
}

// replayTarget selects the point of the event log that a restore or a fork
// replays to. A checkpoint is exact. A frame includes every event recorded up to
// that frame, also the events that came after a checkpoint of the same frame.
type replayTarget struct {
	Checkpoint *int
	Frame      *uint64
}

// checkpointRecord is a named point of the event log.
type checkpointRecord struct {
	Frame  uint64
	Events int
}

// machineEvent records one input, poke, or reset of a machine.
type machineEvent struct {
	Frame      uint64   `json:"frame"`
	Cycles     uint64   `json:"cycles"`
	Kind       string   `json:"kind"`
	Controller *int     `json:"controller,omitempty"`
	Buttons    []string `json:"buttons,omitempty"`
	Address    *Address `json:"address,omitempty"`
	Bytes      []int    `json:"bytes,omitempty"`
}

// bankWrite records one write to a mapper register range.
type bankWrite struct {
	Frame   uint64  `json:"frame"`
	Address Address `json:"address"`
	Value   int     `json:"value"`
}

// memorySearchEntry is one candidate address of a cheat search.
type memorySearchEntry struct {
	Address uint16 `json:"address"`
	Value   byte   `json:"value"`
}

// memorySearch holds the candidate set of a cheat search.
type memorySearch struct {
	candidates []memorySearchEntry
	active     bool
}

// callFrame is one reconstructed call or interrupt frame. ReturnAddress is the
// address where execution continues after the return. The caller fields locate
// the instruction that made the call, or the instruction that the interrupt
// stopped.
type callFrame struct {
	ReturnAddress Address `json:"return_address"`
	CallerAddress Address `json:"caller_address"`
	CallerBank    int     `json:"caller_bank"`
	CallerOffset  int     `json:"caller_prg_offset"`
	Kind          string  `json:"kind"`

	// stackPointer is the stack pointer after the push of the frame. The frame is
	// live while the stack pointer stays at or below this value.
	stackPointer byte
}

// callStackRecorder rebuilds the call stack from the CPU stack events.
type callStackRecorder struct {
	sys *nes.System

	mu        sync.Mutex
	frames    []callFrame
	lastCycle uint64
	remove    func()
}

// newCallStackRecorder attaches the stack hook and returns the recorder.
func newCallStackRecorder(sys *nes.System) *callStackRecorder {
	recorder := &callStackRecorder{sys: sys}
	recorder.remove = sys.AddStackHook(recorder.observe)
	return recorder
}

// newMemorySearch creates an inactive cheat search.
func newMemorySearch() *memorySearch {
	return &memorySearch{}
}

// observe updates the call stack from one stack event. A JSR and an interrupt
// push several bytes, and all events of one instruction share a cycle value, so
// only the first event of an instruction counts. A frame ends when the stack
// pointer rises above the frame. This also drops the frame of a routine that
// pulls its return address and jumps, as a jump table routine does.
func (c *callStackRecorder) observe(event cpu6502.StackEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	first := event.Cycle != c.lastCycle
	c.lastCycle = event.Cycle
	if !first {
		return
	}

	if event.Operation == cpu6502.StackPush {
		c.prune(event.Before)
	} else {
		c.prune(event.After)
	}

	switch {
	case event.Operation == cpu6502.StackPush && event.Interrupt != cpu6502.InterruptNone:
		frame := c.newFrame(event.PC, interruptReturnAddress(event), interruptKind(event.Interrupt))
		frame.stackPointer = event.Before - interruptPushSize
		c.frames = append(c.frames, frame)

	case event.Operation == cpu6502.StackPush && event.Opcode == jsrOpcode:
		frame := c.newFrame(event.PC, event.PC+jsrSize, "jsr")
		frame.stackPointer = event.Before - jsrPushSize
		c.frames = append(c.frames, frame)
	}
}

// prune drops the frames that are above the stack pointer.
func (c *callStackRecorder) prune(stackPointer byte) {
	for len(c.frames) > 0 && c.frames[len(c.frames)-1].stackPointer < stackPointer {
		c.frames = c.frames[:len(c.frames)-1]
	}
}

// newFrame builds a frame and locates the calling instruction.
func (c *callStackRecorder) newFrame(caller, returnAddress uint16, kind string) callFrame {
	bank, offset, _ := c.sys.PRGLocation(caller)
	return callFrame{
		ReturnAddress: Address(returnAddress),
		CallerAddress: Address(caller),
		CallerBank:    bank,
		CallerOffset:  offset,
		Kind:          kind,
	}
}

// clear drops every recorded frame.
func (c *callStackRecorder) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.frames = nil
}

// framesCopy returns a copy of the current call stack.
func (c *callStackRecorder) framesCopy() []callFrame {
	c.mu.Lock()
	defer c.mu.Unlock()

	// A TXS changes the stack pointer without a stack event.
	c.prune(c.sys.SP)
	return append([]callFrame(nil), c.frames...)
}

// interruptReturnAddress returns the address where execution continues after an
// interrupt. BRK skips the byte after the opcode.
func interruptReturnAddress(event cpu6502.StackEvent) uint16 {
	if event.Interrupt == cpu6502.InterruptBRK {
		return event.PC + brkSize
	}
	return event.PC
}

// replayTo recreates the emulation and applies the event log up to a frame. The
// context and the wall-clock timeout bound the replay.
func replayTo(ctx context.Context, sys *nes.System, events []machineEvent, frame uint64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	completed := uint64(0)
	steps := 0
	step := func() error {
		steps++
		if steps%replayCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("replay stopped: %w", err)
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("replay timed out after %s", timeout)
			}
		}
		result, err := sys.StepSystem()
		if err != nil {
			return fmt.Errorf("replaying: %w", err)
		}
		if result.FrameCompleted {
			completed++
		}
		return nil
	}

	err := sys.Guard(func() error {
		for _, event := range events {
			if event.Frame > frame {
				break
			}
			for sys.CPU.Cycles() < event.Cycles {
				if err := step(); err != nil {
					return err
				}
			}
			applyEvent(sys, event)
		}
		for completed < frame {
			if err := step(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("replaying events: %w", err)
	}
	return nil
}

// applyEvent applies one event during replay.
func applyEvent(sys *nes.System, event machineEvent) {
	switch event.Kind {
	case eventReset:
		sys.Reset()

	case eventInput:
		device := controllerDevice(sys.Bus.Controller1)
		if event.Controller != nil && *event.Controller == 2 {
			device = sys.Bus.Controller2
		}
		buttons, err := parseButtons(event.Buttons)
		if err != nil {
			return
		}
		device.SetButtonState(controller.Button(0xff), false)
		device.SetButtonState(buttons, true)

	case eventPoke:
		if event.Address == nil {
			return
		}
		for index, value := range event.Bytes {
			sys.Bus.Memory.Write(uint16(*event.Address)+uint16(index), byte(value))
		}
	}
}

// buttonNameList returns the sorted names of a button bit mask.
func buttonNameList(buttons controller.Button) []string {
	var names []string
	for name, button := range buttonNames {
		if buttons&button != 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// interruptKind names an interrupt source.
func interruptKind(source cpu6502.InterruptSource) string {
	switch source {
	case cpu6502.InterruptNMI:
		return "nmi"
	case cpu6502.InterruptIRQ:
		return "irq"
	case cpu6502.InterruptBRK:
		return "brk"
	default:
		return "interrupt"
	}
}

// validSearchOperation reports whether a filter operation name is known.
func validSearchOperation(operation string) bool {
	switch operation {
	case searchEqual, searchChanged, searchUnchanged, searchIncreased, searchDecreased, searchValue:
		return true
	default:
		return false
	}
}

// searchMatches reports whether a candidate matches an operation.
func searchMatches(operation string, previous, current, value byte) bool {
	switch operation {
	case searchEqual, searchUnchanged:
		return current == previous
	case searchChanged:
		return current != previous
	case searchIncreased:
		return current > previous
	case searchDecreased:
		return current < previous
	case searchValue:
		return current == value
	default:
		return false
	}
}

// ramDiff returns the changed RAM bytes between two snapshots.
func ramDiff(before, after map[uint16]byte) []memorySearchEntry {
	changed := make([]memorySearchEntry, 0)
	for address, previous := range before {
		current, ok := after[address]
		if !ok || current == previous {
			continue
		}
		changed = append(changed, memorySearchEntry{
			Address: address,
			Value:   current,
		})
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Address < changed[j].Address })
	return changed
}
