package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/mcp"
)

func (s *Server) registerStep() error {
	schema, err := inputSchema(reflect.TypeFor[stepArgs](), configureStepSchema)
	if err != nil {
		return err
	}
	return addTool(s, mcp.Tool{
		Name: stepToolName,
		Description: "Step instructions, over a call, or out of a function. Mode is required. " +
			"Returns before/after CPU state, counts, stop reason, breakpoint details, and the trace tail. " +
			"Only mode instructions accepts an instruction count and ignores breakpoints.",
		InputSchema: schema,
		Annotations: writeAnnotations(),
	}, s.step)
}

func (s *Server) step(ctx context.Context, _ mcp.ToolRequest, in stepArgs) (stepResultOut, error) {
	if in.Mode != modeInstructions && in.Mode != modeOver && in.Mode != modeOut {
		return stepResultOut{}, errors.New("mode must be instructions, over, or out")
	}
	if in.Mode != modeInstructions && in.Instructions != nil {
		return stepResultOut{}, errors.New("instructions applies to mode instructions only")
	}
	count := uint64(1)
	if in.Instructions != nil {
		count = *in.Instructions
	}

	if count == 0 || count > maxStepInstructions {
		return stepResultOut{}, fmt.Errorf("instructions must be from 1 to %d", maxStepInstructions)
	}
	timeout, err := parseTimeout(in.Timeout)
	if err != nil {
		return stepResultOut{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return stepResultOut{}, err
	}
	if in.Mode == modeInstructions {
		return machine.instructionStep(ctx, count)
	}
	before := machine.cpuState()
	var outcome runUntilOutcome
	if in.Mode == modeOver {
		outcome, err = machine.stepOver(ctx, timeout)
	} else {
		outcome, err = machine.stepOut(ctx, timeout)
	}

	if err != nil {
		return stepResultOut{}, err
	}
	return debugStepResult(before, runUntilToResult(machine, outcome)), nil
}

func (m *Machine) instructionStep(ctx context.Context, count uint64) (stepResultOut, error) {
	traceBefore := m.traceRing.Total()
	result, err := m.step(ctx, count)
	if err != nil {
		return stepResultOut{}, err
	}
	reason := nes.StopInstructions
	if result.Instructions < count {
		reason = stopNoProgress
	}

	return stepResultOut{
		After:        stateResult(result.After, result.AfterFrame, result.AfterBank, result.AfterOffset),
		Before:       stateResult(result.Before, result.BeforeFrame, result.BeforeBank, result.BeforeOffset),
		Reason:       reason,
		Cycles:       result.After.Cycles - result.Before.Cycles,
		Frames:       result.Frames,
		Instructions: result.Instructions,
		Trace:        m.traceRing.LinesSince(traceBefore),
	}, nil
}

type stepMode string

const (
	modeInstructions stepMode = "instructions"
	modeOver         stepMode = "over"
	modeOut          stepMode = "out"
)

type stepArgs struct {
	Instructions *uint64  `json:"instructions,omitempty" description:"Instruction count for mode instructions only. Default 1; maximum 1000000."`
	MachineID    string   `json:"machine_id" description:"Identifier returned by create_machine."`
	Mode         stepMode `json:"mode" enum:"instructions,over,out" description:"instructions ignores breakpoints. over and out honor breakpoints during call-return runs; their single-instruction fallback ignores breakpoints."`
	Timeout      string   `json:"timeout,omitempty" description:"Wall-clock limit including queue time. Default 10s; maximum 2m."`
}

type stepResultOut struct {
	After  cpuStateResult     `json:"after"`
	Before cpuStateResult     `json:"before"`
	Reason nes.RunUntilReason `json:"reason"`

	Cycles       uint64 `json:"cycles"`
	Frames       uint64 `json:"frames"`
	Instructions uint64 `json:"instructions"`

	BreakpointID  int     `json:"breakpoint_id"`
	HasBreakpoint bool    `json:"has_breakpoint"`
	HitAddress    Address `json:"hit_address"`
	HitPC         Address `json:"hit_pc"`
	HitValue      int     `json:"hit_value"`

	Error string   `json:"error,omitempty"`
	Trace []string `json:"trace,omitempty"`
}

func debugStepResult(before cpuStateResult, result runUntilResult) stepResultOut {
	return stepResultOut{
		After:         result.CPU,
		Before:        before,
		Reason:        result.Reason,
		Cycles:        result.CPUCycles,
		Frames:        result.CompletedFrames,
		Instructions:  result.Instructions,
		BreakpointID:  result.BreakpointID,
		HasBreakpoint: result.HasBreakpoint,
		HitAddress:    result.HitAddress,
		HitPC:         result.HitPC,
		HitValue:      result.HitValue,
		Error:         result.Error,
		Trace:         result.Trace,
	}
}

func configureStepSchema(schema map[string]any) {
	instructions := schemaProperty(schema, fieldInstructions)
	instructions[schemaMinimum] = 1
	instructions[schemaMaximum] = maxStepInstructions
	schema[schemaOneOf] = []any{
		map[string]any{
			schemaProperties: map[string]any{fieldMode: map[string]any{schemaEnum: []stepMode{modeInstructions}}},
		},
		map[string]any{
			schemaProperties: map[string]any{
				fieldInstructions: false,
				fieldMode:         map[string]any{schemaEnum: []stepMode{modeOver, modeOut}},
			},
		},
	}
}
