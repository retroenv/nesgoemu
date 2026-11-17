package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/mcp"
)

const allControllerButtons = controller.A | controller.B | controller.Select | controller.Start |
	controller.Up | controller.Down | controller.Left | controller.Right

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
		s.registerReadMemory,
		s.registerWriteMemory,
		s.registerSetButtons,
		s.registerScreenshot,
		s.registerInspectMachine,
	}

	for _, register := range tools {
		if err := register(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) registerCreateMachine() error {
	return addTool(s, mcp.Tool{
		Name:        createMachineToolName,
		Description: "Load a ROM and create a headless machine. Returns the machine_id and the ROM summary.",
		Annotations: writeAnnotations(),
	}, s.createMachine)
}

func (s *Server) registerDestroyMachine() error {
	return addTool(s, mcp.Tool{
		Name:        destroyMachineToolName,
		Description: "Free a machine and release its resources.",
		Annotations: destructiveAnnotations(),
	}, s.destroyMachine)
}

func (s *Server) registerListMachines() error {
	return addTool(s, mcp.Tool{
		Name:        listMachinesToolName,
		Description: "List the live machines with their frame counters and idle time.",
		Annotations: readAnnotations(),
	}, s.listMachines)
}

func (s *Server) registerReset() error {
	return addTool(s, mcp.Tool{
		Name:        resetToolName,
		Description: "Reset a machine to its power-on state. A hard reset recreates the system.",
		Annotations: writeAnnotations(),
	}, s.reset)
}

func (s *Server) registerRunFrames() error {
	schema, err := runFramesInputSchema()
	if err != nil {
		return err
	}
	return addTool(s, mcp.Tool{
		Name: runFramesToolName,
		Description: "Run frames with optional controller inputs, ignoring breakpoints. " +
			"An optional capture returns a screenshot and bounded memory reads from the exact stopped state. " +
			"Capture arguments are checked before execution.",
		InputSchema: schema,
		Annotations: writeAnnotations(),
	}, s.runFrames, runCaptureContent)
}

func (s *Server) registerReadMemory() error {
	return addTool(s, mcp.Tool{
		Name:        readMemoryToolName,
		Description: "Read a side-effect-free memory range as a hex dump and a byte array.",
		Annotations: readAnnotations(),
	}, s.readMemory)
}

func (s *Server) registerWriteMemory() error {
	schema, err := inputSchema(reflect.TypeFor[writeMemoryArgs](), configureWriteSchema)
	if err != nil {
		return err
	}
	return addTool(s, mcp.Tool{
		Name:        writeMemoryToolName,
		Description: "Write bytes to CPU RAM ($0000-$1FFF) or the cartridge range $6000-$7FFF. Writes to $6000-$7FFF go through the cartridge bus, so a mapper that decodes registers there sees a register write. Hardware register and ROM ranges are refused.",
		InputSchema: schema,
		Annotations: writeAnnotations(),
	}, s.writeMemory)
}

func (s *Server) registerSetButtons() error {
	return addTool(s, mcp.Tool{
		Name:        setButtonsToolName,
		Description: "Set the buttons of one controller. A hold count runs frames with the buttons held.",
		Annotations: writeAnnotations(),
	}, s.setButtons)
}

func (s *Server) registerScreenshot() error {
	return addImageTool[screenshotArgs](s, mcp.Tool{
		Name:        screenshotToolName,
		Description: "Render the current display as PNG. Does not advance time.",
		Annotations: readAnnotations(),
	}, s.screenshotToolName)
}

func (s *Server) createMachine(ctx context.Context, _ mcp.ToolRequest,
	in createMachineArgs) (createMachineResult, error) {

	if err := s.acquireLoad(ctx); err != nil {
		return createMachineResult{}, err
	}
	defer s.releaseLoad()

	if err := s.registry.checkCapacity(); err != nil {
		return createMachineResult{}, err
	}
	path, err := s.roots.resolve(in.ROM)
	if err != nil {
		return createMachineResult{}, err
	}
	entrypoint := -1
	if in.Entrypoint != nil {
		entrypoint = *in.Entrypoint
		if entrypoint < 0 || entrypoint > math.MaxUint16 {
			return createMachineResult{}, fmt.Errorf("entrypoint %d is outside 0 to 65535", entrypoint)
		}
	}

	battery := in.Battery != nil && *in.Battery

	palette, err := s.readDisplayPalette(ctx, in.Palette)
	if err != nil {
		return createMachineResult{}, err
	}

	machine, summary, err := loadMachine(ctx, s.logger, path, entrypoint, battery, palette, nil)
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

// readDisplayPalette reads a palette file under the allowed roots and checks
// its size. An empty path selects the default palette. The read stops one byte
// after the largest valid size, so a large file cannot cause a large read.
func (s *Server) readDisplayPalette(ctx context.Context, path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	resolved, err := s.roots.resolve(path)
	if err != nil {
		return nil, err
	}

	const maxPaletteBytes = 8 * 64 * 3 // Eight emphasis sets of 64 RGB colors.
	data, err := readRegularFile(ctx, resolved, maxPaletteBytes)
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
	return destroyMachineResult{
		Destroyed: true,
	}, nil
}

func (s *Server) listMachines(ctx context.Context, _ mcp.ToolRequest, _ struct{}) (listMachinesResult, error) {
	machines := s.registry.list()
	out := make([]machineSummary, 0, len(machines))
	for _, machine := range machines {
		release, err := machine.acquireRequest(ctx)
		if err != nil {
			return listMachinesResult{}, err
		}
		out = append(out, machineSummary{
			Frame:       machine.frame(),
			ID:          machine.id,
			IdleSeconds: time.Since(machine.lastUsed()).Seconds(),
			ROM:         machine.rom,
		})
		release()
	}

	return listMachinesResult{
		Machines: out,
	}, nil
}

func (s *Server) reset(ctx context.Context, _ mcp.ToolRequest, in resetArgs) (resetResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return resetResult{}, err
	}
	if in.Hard != nil && *in.Hard {
		if err := machine.hardReset(ctx, s.logger); err != nil {
			return resetResult{}, err
		}
	} else {
		if err := machine.reset(); err != nil {
			return resetResult{}, err
		}
	}

	return resetResult{
		Reset: true,
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

	for _, input := range inputs {
		if input.Frame >= in.Frames {
			return runFramesOut{}, fmt.Errorf("input frame %d is outside the requested %d frames", input.Frame, in.Frames)
		}
	}

	capture, err := prepareCapture(in.Capture)
	if err != nil {
		return runFramesOut{}, err
	}
	result, err := machine.runFrames(ctx, in.Frames, inputs, timeout)
	if err != nil {
		return runFramesOut{}, err
	}
	return machine.captureRun(runFramesToOut(result), capture)
}

func (s *Server) readMemory(_ context.Context, _ mcp.ToolRequest, in readMemoryArgs) (readMemoryResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return readMemoryResult{}, err
	}
	read, err := prepareMemoryRead(in.Space, in.Address, in.Offset, in.Length)
	if err != nil {
		return readMemoryResult{}, err
	}
	return machine.readMemory(read.space, read.start, read.length), nil
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
	return writeMemoryResult{
		Written: len(data),
	}, nil
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

func (s *Server) screenshotToolName(_ context.Context, req mcp.ToolRequest) (*mcp.ToolResult, error) {
	var in screenshotArgs
	if len(req.Arguments) > 0 {
		if err := json.Unmarshal(req.Arguments, &in); err != nil {
			return mcp.Errorf(invalidArgumentsFormat, err), nil
		}
	}

	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	scale := defaultImageScale
	if in.Scale != nil {
		scale = *in.Scale
	}

	if scale < 1 || scale > maxImageScale {
		return mcp.Errorf("Scale %d is outside 1 to %d", scale, maxImageScale), nil
	}

	image, err := machine.screenshot(scale)
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	state := machine.cpuState()
	text := fmt.Sprintf("frame %d, PC %s, A:%02X X:%02X Y:%02X P:%02X",
		state.Frame, state.PC, state.A, state.X, state.Y, state.P)

	return &mcp.ToolResult{
		Content: []mcp.Content{mcp.Text(text), mcp.Image(image, mimeTypePNG)},
	}, nil
}

// createMachineArgs selects a ROM and its boot options.
type createMachineArgs struct {
	Battery    *bool  `json:"battery,omitempty" description:"Load <rom>.sav from the ROM directory at creation. The server never writes the file. Default false."`
	Entrypoint *int   `json:"entrypoint,omitempty" description:"Start address of the CPU. Omit to use the reset vector."`
	Palette    string `json:"palette,omitempty" description:"Optional 192- or 1536-byte RGB .pal file under an allowed root."`
	ROM        string `json:"rom" description:"Path to a .nes ROM inside the allowed roots."`
}

// createMachineResult reports the new machine and its ROM identity.
type createMachineResult struct {
	CPU       cpuStateResult `json:"cpu"`
	MachineID string         `json:"machine_id" description:"Handle for every other tool."`
	ROM       romSummary     `json:"rom"`
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
	Frame       uint64  `json:"frame"`
	ID          string  `json:"machine_id"`
	IdleSeconds float64 `json:"idle_seconds"`
	ROM         string  `json:"rom"`
}

// listMachinesResult lists the live machines.
type listMachinesResult struct {
	Machines []machineSummary `json:"machines"`
}

// resetArgs selects a soft or hard reset.
type resetArgs struct {
	Hard      *bool  `json:"hard,omitempty" description:"Recreate the system instead of resetting it. Default false."`
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
}

// resetResult reports a completed reset.
type resetResult struct {
	Reset bool `json:"reset"`
}

// inputArg changes one controller before a relative frame starts.
type inputArg struct {
	Buttons    []string `json:"buttons" description:"Pressed buttons: A, B, Select, Start, Up, Down, Left, Right."`
	Controller *int     `json:"controller,omitempty" description:"Controller 1 or 2. Default 1."`
	Frame      uint64   `json:"frame" description:"Zero-based frame number."`
}

// runFramesArgs runs a frame count with optional inputs.
type runFramesArgs struct {
	Capture   *runCaptureArgs `json:"capture,omitempty" description:"Optional screenshot and bounded memory reads after the run. Observations do not advance time."`
	Frames    uint64          `json:"frames" description:"Number of frames to run."`
	Inputs    []inputArg      `json:"inputs,omitempty" description:"Controller changes at frame boundaries."`
	MachineID string          `json:"machine_id" description:"Identifier returned by create_machine."`
	Timeout   string          `json:"timeout,omitempty" description:"Wall-clock limit, for example \"10s\". Default 10s."`
}

// runFramesOut reports the result of a bounded run.
type runFramesOut struct {
	Capture         *runCaptureResult  `json:"capture,omitempty"`
	CompletedFrames uint64             `json:"completed_frames"`
	CPUCycles       uint64             `json:"cpu_cycles"`
	Frame           uint64             `json:"frame"`
	PC              Address            `json:"pc"`
	Reason          nes.RunUntilReason `json:"reason"`
	TimedOut        bool               `json:"timed_out"`

	image []byte
}

// readMemoryArgs selects a side-effect-free memory view.
type readMemoryArgs struct {
	Address   Address `json:"address,omitempty" description:"CPU or PPU address. Default 0. Use offset for prg and chr."`
	Length    *int    `json:"length,omitempty" description:"Byte count, maximum 4096. Default 256."`
	MachineID string  `json:"machine_id" description:"Identifier returned by create_machine."`
	Offset    *int    `json:"offset,omitempty" description:"Physical offset in the prg or chr space. Replaces address."`
	Space     string  `json:"space" description:"One of cpu, ppu, oam, prg, chr, ram."`
}

// writeMemoryArgs selects bytes to write.
type writeMemoryArgs struct {
	Address   Address `json:"address" description:"CPU address of the first byte."`
	Bytes     []int   `json:"bytes" description:"From 1 to 4096 byte values, each from 0 to 255."`
	MachineID string  `json:"machine_id" description:"Identifier returned by create_machine."`
	Space     string  `json:"space" description:"cpu for RAM and PRG-RAM."`
}

// writeMemoryResult reports the written byte count.
type writeMemoryResult struct {
	Written int `json:"written"`
}

// setButtonsArgs sets a controller state.
type setButtonsArgs struct {
	Buttons    []string `json:"buttons" description:"Pressed buttons: A, B, Select, Start, Up, Down, Left, Right."`
	Controller *int     `json:"controller,omitempty" description:"Controller 1 or 2. Default 1."`
	HoldFrames *uint64  `json:"hold_frames,omitempty" description:"Frames to run with the buttons held. Default 0 holds until changed."`
	MachineID  string   `json:"machine_id" description:"Identifier returned by create_machine."`
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

// parseInputs converts tool inputs into controller inputs in frame order.
func parseInputs(inputs []inputArg) ([]controllerInput, error) {
	parsed := make([]controllerInput, 0, len(inputs))
	for _, input := range inputs {
		controllerID := 1
		if input.Controller != nil {
			controllerID = *input.Controller
		}

		if controllerID != 1 && controllerID != 2 {
			return nil, fmt.Errorf(controllerNumberFormat, controllerID)
		}
		buttons, err := parseButtons(input.Buttons)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, controllerInput{
			Buttons:    buttons,
			Controller: controllerID,
			Frame:      input.Frame,
		})
	}

	sort.SliceStable(parsed, func(i, j int) bool {
		return parsed[i].Frame < parsed[j].Frame
	})

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
		if value < 0 || value > math.MaxUint8 {
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
		CompletedFrames: result.CompletedFrames,
		CPUCycles:       result.CPUCycles,
		Frame:           result.Frame,
		PC:              Address(result.PC),
		Reason:          result.Reason,
		TimedOut:        result.TimedOut,
	}
}

// readAnnotations marks a tool as side-effect free.
func readAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		ReadOnlyHint: true,
	}
}

// writeAnnotations marks a tool as a state change.
func writeAnnotations() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{}
}

// destructiveAnnotations marks a tool as destructive.
func destructiveAnnotations() *mcp.ToolAnnotations {
	value := true
	return &mcp.ToolAnnotations{
		DestructiveHint: &value,
	}
}
