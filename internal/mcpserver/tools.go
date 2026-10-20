package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/mcp"
)

// registerTools registers every tool on the MCP server.
func (s *Server) registerTools() error {
	groups := []func() error{
		s.registerBaseTools,
		s.registerDebugTools,
		s.registerAnalysisTools,
		s.registerPPUTools,
	}
	for _, register := range groups {
		if err := register(); err != nil {
			return err
		}
	}
	return nil
}

// registerBaseTools registers the machine, memory, and display tools.
func (s *Server) registerBaseTools() error {
	tools := []func() error{
		s.registerCreateMachine,
		s.registerDestroyMachine,
		s.registerListMachines,
		s.registerReset,
		s.registerStep,
		s.registerRunFrames,
		s.registerCPUState,
		s.registerReadMemory,
		s.registerWriteMemory,
		s.registerSetButtons,
		s.registerScreenshot,
		s.registerMachineInfo,
	}
	for _, register := range tools {
		if err := register(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) registerCreateMachine() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "create_machine",
		Description: "Load a ROM and create a headless machine. Returns the machine_id and the ROM summary.",
		Annotations: writeAnnotations(),
	}, s.createMachine)
	return registrationError(err, "create_machine")
}

func (s *Server) registerDestroyMachine() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "destroy_machine",
		Description: "Free a machine and release its resources.",
		Annotations: destructiveAnnotations(),
	}, s.destroyMachine)
	return registrationError(err, "destroy_machine")
}

func (s *Server) registerListMachines() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "list_machines",
		Description: "List the live machines with their frame counters and idle time.",
		Annotations: readAnnotations(),
	}, s.listMachines)
	return registrationError(err, "list_machines")
}

func (s *Server) registerReset() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "reset",
		Description: "Reset a machine to its power-on state. A hard reset recreates the system.",
		Annotations: writeAnnotations(),
	}, s.reset)
	return registrationError(err, "reset")
}

func (s *Server) registerStep() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "step",
		Description: "Execute a number of CPU instructions. Returns the CPU state before and after.",
		Annotations: writeAnnotations(),
	}, s.step)
	return registrationError(err, "step")
}

func (s *Server) registerRunFrames() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name: "run_frames",
		Description: "Run frames with optional controller inputs. This is the only way to advance " +
			"game time for a picture.",
		Annotations: writeAnnotations(),
	}, s.runFrames)
	return registrationError(err, "run_frames")
}

func (s *Server) registerCPUState() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "cpu_state",
		Description: "Return the CPU registers, flags, cycle count, pending interrupts, and the PRG location of PC.",
		Annotations: readAnnotations(),
	}, s.cpuState)
	return registrationError(err, "cpu_state")
}

func (s *Server) registerReadMemory() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "read_memory",
		Description: "Read a side-effect-free memory range as a hex dump and a byte array.",
		Annotations: readAnnotations(),
	}, s.readMemory)
	return registrationError(err, "read_memory")
}

func (s *Server) registerWriteMemory() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "write_memory",
		Description: "Write bytes to CPU RAM or PRG-RAM. Register and ROM ranges are refused.",
		Annotations: writeAnnotations(),
	}, s.writeMemory)
	return registrationError(err, "write_memory")
}

func (s *Server) registerSetButtons() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "set_buttons",
		Description: "Set the buttons of one controller. A hold count runs frames with the buttons held.",
		Annotations: writeAnnotations(),
	}, s.setButtons)
	return registrationError(err, "set_buttons")
}

func (s *Server) registerScreenshot() error {
	schema, err := mcp.SchemaFor(reflect.TypeFor[screenshotArgs]())
	if err != nil {
		return fmt.Errorf("screenshot schema: %w", err)
	}
	err = s.mcp.AddTool(mcp.Tool{
		InputSchema: schema,
		Name:        "screenshot",
		Description: "Render the current display as PNG. Does not advance time.",
		Annotations: readAnnotations(),
	}, s.screenshotTool)
	return registrationError(err, "screenshot")
}

func (s *Server) registerMachineInfo() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "machine_info",
		Description: "Return the mapper windows, mirroring, RAM sizes, feature usage, and battery status.",
		Annotations: readAnnotations(),
	}, s.machineInfo)
	return registrationError(err, "machine_info")
}

func (s *Server) createMachine(_ context.Context, _ mcp.ToolRequest,
	in createMachineArgs) (createMachineResult, error) {

	path, err := s.roots.resolve(in.ROM)
	if err != nil {
		return createMachineResult{}, err
	}
	entrypoint := -1
	if in.Entrypoint != nil {
		entrypoint = *in.Entrypoint
		if entrypoint < 0 || entrypoint > 0xFFFF {
			return createMachineResult{}, fmt.Errorf("entrypoint %d is outside 0 to 65535", entrypoint)
		}
	}
	battery := in.Battery != nil && *in.Battery

	palette, err := s.readDisplayPalette(in.Palette)
	if err != nil {
		return createMachineResult{}, err
	}

	machine, summary, err := loadMachine(s.logger, path, entrypoint, battery, palette)
	if err != nil {
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

// readDisplayPalette reads a palette file under the allowed roots and checks
// its size. An empty path selects the default palette. The read stops one byte
// after the largest valid size, so a large file cannot cause a large read.
func (s *Server) readDisplayPalette(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	resolved, err := s.roots.resolve(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("opening display palette: %w", err)
	}
	defer func() { _ = file.Close() }()

	const maxPaletteBytes = 8 * 64 * 3 // Eight emphasis sets of 64 RGB colors.
	data, err := io.ReadAll(io.LimitReader(file, maxPaletteBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading display palette: %w", err)
	}

	if _, err := ppu.ParseDisplayPalette(data); err != nil {
		return nil, fmt.Errorf("display palette: %w", err)
	}
	return data, nil
}

func (s *Server) destroyMachine(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (destroyMachineResult, error) {
	if !s.registry.remove(in.MachineID) {
		return destroyMachineResult{}, unknownMachineError(in.MachineID, s.registry.ttl)
	}
	return destroyMachineResult{Destroyed: true}, nil
}

func (s *Server) listMachines(_ context.Context, _ mcp.ToolRequest, _ struct{}) (listMachinesResult, error) {
	machines := s.registry.list()
	out := make([]machineSummary, 0, len(machines))
	for _, machine := range machines {
		out = append(out, machineSummary{
			ID:          machine.id,
			ROM:         machine.rom,
			Frame:       machine.frame(),
			IdleSeconds: time.Since(machine.lastUsed()).Seconds(),
		})
	}
	return listMachinesResult{Machines: out}, nil
}

func (s *Server) reset(_ context.Context, _ mcp.ToolRequest, in resetArgs) (resetResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return resetResult{}, err
	}
	if in.Hard != nil && *in.Hard {
		if err := machine.hardReset(s.logger); err != nil {
			return resetResult{}, err
		}
	} else {
		machine.reset()
	}
	return resetResult{Reset: true}, nil
}

func (s *Server) step(_ context.Context, _ mcp.ToolRequest, in stepArgs) (stepResultOut, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return stepResultOut{}, err
	}
	count := uint64(1)
	if in.Instructions != nil {
		count = *in.Instructions
	}
	if count == 0 {
		count = 1
	}
	if count > maxStepInstructions {
		return stepResultOut{}, fmt.Errorf("instructions %d exceeds the limit %d", count, maxStepInstructions)
	}

	result, err := machine.step(count)
	if err != nil {
		return stepResultOut{}, err
	}
	return stepResultOut{
		Before:       stateResult(result.Before, result.BeforeFrame, result.BeforeBank, result.BeforeOffset),
		After:        stateResult(result.After, result.AfterFrame, result.AfterBank, result.AfterOffset),
		Instructions: result.Instructions,
		Cycles:       result.After.Cycles - result.Before.Cycles,
		Frames:       result.Frames,
	}, nil
}

func (s *Server) runFrames(ctx context.Context, _ mcp.ToolRequest, in runFramesArgs) (runFramesOut, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return runFramesOut{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return runFramesOut{}, err
	}
	inputs, err := parseInputs(in.Inputs)
	if err != nil {
		return runFramesOut{}, err
	}

	result, err := machine.runFrames(ctx, in.Frames, inputs, timeout)
	if err != nil {
		return runFramesOut{}, err
	}
	return runFramesToOut(result), nil
}

func (s *Server) cpuState(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (cpuStateResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return cpuStateResult{}, err
	}
	return machine.cpuState(), nil
}

func (s *Server) readMemory(_ context.Context, _ mcp.ToolRequest, in readMemoryArgs) (readMemoryResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return readMemoryResult{}, err
	}
	if !validSpace(in.Space) {
		return readMemoryResult{}, fmt.Errorf("space %q is not one of cpu, ppu, oam, prg, chr, ram", in.Space)
	}
	length := 256
	if in.Length != nil {
		length = *in.Length
	}
	if length <= 0 || length > maxReadLength {
		return readMemoryResult{}, fmt.Errorf("length %d is outside 1 to %d", length, maxReadLength)
	}

	start := int(in.Address)
	if in.Offset != nil {
		if in.Space != spacePRG && in.Space != spaceCHR {
			return readMemoryResult{}, errors.New("offset applies to the prg and chr spaces only; use address")
		}
		start = *in.Offset
	}
	if start < 0 {
		return readMemoryResult{}, fmt.Errorf("offset %d is negative", start)
	}
	return machine.readMemory(in.Space, start, length), nil
}

func (s *Server) writeMemory(_ context.Context, _ mcp.ToolRequest, in writeMemoryArgs) (writeMemoryResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return writeMemoryResult{}, err
	}
	data, err := toBytes(in.Bytes)
	if err != nil {
		return writeMemoryResult{}, err
	}
	if err := machine.writeMemory(in.Space, in.Address, data); err != nil {
		return writeMemoryResult{}, err
	}
	return writeMemoryResult{Written: len(data)}, nil
}

func (s *Server) setButtons(ctx context.Context, _ mcp.ToolRequest, in setButtonsArgs) (setButtonsResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return setButtonsResult{}, err
	}
	controllerID := 1
	if in.Controller != nil {
		controllerID = *in.Controller
	}
	buttons, err := parseButtons(in.Buttons)
	if err != nil {
		return setButtonsResult{}, err
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return setButtonsResult{}, err
	}
	hold := uint64(0)
	if in.HoldFrames != nil {
		hold = *in.HoldFrames
	}

	result, err := machine.setButtons(ctx, controllerID, buttons, hold, timeout)
	if err != nil {
		return setButtonsResult{}, err
	}
	return setButtonsResult{
		HeldFrames: result.CompletedFrames,
		TimedOut:   result.TimedOut,
	}, nil
}

func (s *Server) screenshotTool(_ context.Context, req mcp.ToolRequest) (*mcp.ToolResult, error) {
	var in screenshotArgs
	if len(req.Arguments) > 0 {
		if err := json.Unmarshal(req.Arguments, &in); err != nil {
			return mcp.Errorf("Invalid arguments: %v", err), nil
		}
	}
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	scale := 1
	if in.Scale != nil {
		scale = *in.Scale
	}
	if scale < 1 || scale > 4 {
		return mcp.Errorf("Scale %d is outside 1 to 4", scale), nil
	}

	image, err := machine.screenshot(scale)
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	state := machine.cpuState()
	text := fmt.Sprintf("frame %d, PC %s, A:%02X X:%02X Y:%02X P:%02X",
		state.Frame, state.PC, state.A, state.X, state.Y, state.P)
	return &mcp.ToolResult{
		Content: []mcp.Content{mcp.Text(text), mcp.Image(image, "image/png")},
	}, nil
}

func (s *Server) machineInfo(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (machineInfoResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return machineInfoResult{}, err
	}
	return machine.machineInfo(), nil
}

// createMachineArgs selects a ROM and its boot options.
type createMachineArgs struct {
	ROM        string `json:"rom" description:"Path to a .nes ROM inside the allowed roots."`
	Entrypoint *int   `json:"entrypoint,omitempty" description:"Start address of the CPU. Omit to use the reset vector."`
	Battery    *bool  `json:"battery,omitempty" description:"Read and write <rom>.sav inside the roots. Default false."`
	Palette    string `json:"palette,omitempty" description:"Optional 192- or 1536-byte RGB .pal file under an allowed root."`
}

// createMachineResult reports the new machine and its ROM identity.
type createMachineResult struct {
	MachineID string         `json:"machine_id" description:"Handle for every other tool."`
	ROM       romSummary     `json:"rom"`
	CPU       cpuStateResult `json:"cpu"`
}

// machineIDArgs selects one machine.
type machineIDArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
}

// destroyMachineResult reports the removal of a machine.
type destroyMachineResult struct {
	Destroyed bool `json:"destroyed"`
}

// machineSummary describes one machine in a list.
type machineSummary struct {
	ID          string  `json:"machine_id"`
	ROM         string  `json:"rom"`
	Frame       uint64  `json:"frame"`
	IdleSeconds float64 `json:"idle_seconds"`
}

// listMachinesResult lists the live machines.
type listMachinesResult struct {
	Machines []machineSummary `json:"machines"`
}

// resetArgs selects a soft or hard reset.
type resetArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Hard      *bool  `json:"hard,omitempty" description:"Recreate the system instead of resetting it. Default false."`
}

// resetResult reports a completed reset.
type resetResult struct {
	Reset bool `json:"reset"`
}

// stepArgs selects the instruction count.
type stepArgs struct {
	MachineID    string  `json:"machine_id" description:"Identifier returned by create_machine."`
	Instructions *uint64 `json:"instructions,omitempty" description:"Number of instructions to execute. Default 1."`
}

// stepResultOut reports the CPU state around a bounded step.
type stepResultOut struct {
	Before       cpuStateResult `json:"before"`
	After        cpuStateResult `json:"after"`
	Instructions uint64         `json:"instructions"`
	Cycles       uint64         `json:"cycles"`
	Frames       uint64         `json:"frames"`
}

// inputArg changes one controller before a relative frame starts.
type inputArg struct {
	Frame      uint64   `json:"frame" description:"Zero-based frame number."`
	Controller *int     `json:"controller,omitempty" description:"Controller 1 or 2. Default 1."`
	Buttons    []string `json:"buttons" description:"Pressed buttons: A, B, Select, Start, Up, Down, Left, Right."`
}

// runFramesArgs runs a frame count with optional inputs.
type runFramesArgs struct {
	MachineID string     `json:"machine_id" description:"Identifier returned by create_machine."`
	Frames    uint64     `json:"frames" description:"Number of frames to run."`
	Inputs    []inputArg `json:"inputs,omitempty" description:"Controller changes at frame boundaries."`
	Timeout   string     `json:"timeout,omitempty" description:"Wall-clock limit, for example \"10s\". Default 10s."`
}

// runFramesOut reports the result of a bounded run.
type runFramesOut struct {
	Reason          string  `json:"reason"`
	CompletedFrames uint64  `json:"completed_frames"`
	Frame           uint64  `json:"frame"`
	CPUCycles       uint64  `json:"cpu_cycles"`
	PC              Address `json:"pc"`
	TimedOut        bool    `json:"timed_out"`
}

// readMemoryArgs selects a side-effect-free memory view.
type readMemoryArgs struct {
	MachineID string  `json:"machine_id" description:"Identifier returned by create_machine."`
	Space     string  `json:"space" description:"One of cpu, ppu, oam, prg, chr, ram."`
	Address   Address `json:"address" description:"CPU or PPU address. Use offset for prg and chr."`
	Offset    *int    `json:"offset,omitempty" description:"Physical offset in the prg or chr space. Replaces address."`
	Length    *int    `json:"length,omitempty" description:"Byte count, maximum 4096. Default 256."`
}

// writeMemoryArgs selects bytes to write.
type writeMemoryArgs struct {
	MachineID string  `json:"machine_id" description:"Identifier returned by create_machine."`
	Space     string  `json:"space" description:"cpu for RAM and PRG-RAM."`
	Address   Address `json:"address" description:"CPU address of the first byte."`
	Bytes     []int   `json:"bytes" description:"Byte values 0 to 255."`
}

// writeMemoryResult reports the written byte count.
type writeMemoryResult struct {
	Written int `json:"written"`
}

// setButtonsArgs sets a controller state.
type setButtonsArgs struct {
	MachineID  string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Controller *int     `json:"controller,omitempty" description:"Controller 1 or 2. Default 1."`
	Buttons    []string `json:"buttons" description:"Pressed buttons: A, B, Select, Start, Up, Down, Left, Right."`
	HoldFrames *uint64  `json:"hold_frames,omitempty" description:"Frames to run with the buttons held. Default 0 holds until changed."`
	Timeout    string   `json:"timeout,omitempty" description:"Wall-clock limit for the hold run. Default 10s."`
}

// setButtonsResult reports a hold run.
type setButtonsResult struct {
	HeldFrames uint64 `json:"held_frames"`
	TimedOut   bool   `json:"timed_out"`
}

// screenshotArgs selects the image scale.
type screenshotArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Scale     *int   `json:"scale,omitempty" description:"Pixel scale 1 to 4. Default 1."`
}

// registrationError wraps a tool or resource registration error.
func registrationError(err error, name string) error {
	if err != nil {
		return fmt.Errorf("registering %s: %w", name, err)
	}
	return nil
}

// parseInputs converts tool inputs into controller inputs in frame order.
func parseInputs(inputs []inputArg) ([]controllerInput, error) {
	parsed := make([]controllerInput, 0, len(inputs))
	for _, input := range inputs {
		controllerID := 1
		if input.Controller != nil {
			controllerID = *input.Controller
		}
		if controllerID != 1 && controllerID != 2 {
			return nil, fmt.Errorf("controller number %d is not 1 or 2", controllerID)
		}
		buttons, err := parseButtons(input.Buttons)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, controllerInput{
			Frame:      input.Frame,
			Controller: controllerID,
			Buttons:    buttons,
		})
	}
	sort.SliceStable(parsed, func(i, j int) bool { return parsed[i].Frame < parsed[j].Frame })
	return parsed, nil
}

// parseButtons converts button names into a controller bit mask.
func parseButtons(names []string) (controller.Button, error) {
	var buttons controller.Button
	for _, name := range names {
		button, ok := buttonNames[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return 0, fmt.Errorf("unknown button %q", name)
		}
		buttons |= button
	}
	return buttons, nil
}

// parseTimeout decodes an optional duration string. An empty or zero value
// selects the default. A value above the maximum is an error, so a call stays
// bounded.
func parseTimeout(value string) (time.Duration, error) {
	if value == "" {
		return defaultRunTimeout, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("decoding timeout %q: %w", value, err)
	}
	switch {
	case duration < 0:
		return 0, errors.New("timeout must not be negative")
	case duration == 0:
		return defaultRunTimeout, nil
	case duration > maxRunTimeout:
		return 0, fmt.Errorf("timeout %s exceeds the limit %s", duration, maxRunTimeout)
	default:
		return duration, nil
	}
}

// toBytes converts integer values into bytes and rejects out-of-range values.
func toBytes(values []int) ([]byte, error) {
	data := make([]byte, len(values))
	for index, value := range values {
		if value < 0 || value > 0xFF {
			return nil, fmt.Errorf("byte %d at index %d is outside 0 to 255", value, index)
		}
		data[index] = byte(value)
	}
	return data, nil
}

// validSpace reports whether a memory space name is supported.
func validSpace(space string) bool {
	switch space {
	case spaceCPU, spacePPU, spaceOAM, spacePRG, spaceCHR, spaceRAM:
		return true
	default:
		return false
	}
}

// runFramesToOut converts a run result into a tool result.
func runFramesToOut(result runFramesResult) runFramesOut {
	return runFramesOut{
		Reason:          result.Reason,
		CompletedFrames: result.CompletedFrames,
		Frame:           result.Frame,
		CPUCycles:       result.CPUCycles,
		PC:              Address(result.PC),
		TimedOut:        result.TimedOut,
	}
}

// readAnnotations marks a tool as side-effect free.
func readAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true}
}

// writeAnnotations marks a tool as a state change.
func writeAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{}
}

// destructiveAnnotations marks a tool as destructive.
func destructiveAnnotations() *mcp.ToolAnnotations {
	value := true
	return &mcp.ToolAnnotations{DestructiveHint: &value}
}

// buttonNames maps a button name to its controller bit.
var buttonNames = map[string]controller.Button{
	"a":      controller.A,
	"b":      controller.B,
	"select": controller.Select,
	"start":  controller.Start,
	"up":     controller.Up,
	"down":   controller.Down,
	"left":   controller.Left,
	"right":  controller.Right,
}
