package nes

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"time"

	"github.com/retroenv/nesgoemu/pkg/controller"
)

// BootPolicy selects the state used at the start of a run.
type BootPolicy uint8

const (
	// BootCurrent continues from the current system state. A new System provides a cold boot.
	BootCurrent BootPolicy = iota
	// BootReset resets the mapper, PPU controls, and CPU before execution.
	BootReset
)

// ExitReason identifies why a bounded run stopped.
type ExitReason string

const (
	ExitSuccess         ExitReason = "success"
	ExitInvalidOptions  ExitReason = "invalid_options"
	ExitAssertionFailed ExitReason = "assertion_failure"
	ExitInvalidStep     ExitReason = "invalid_instruction"
	ExitTimeout         ExitReason = "timeout"
)

// TimeoutKind identifies which independent run limit stopped execution.
type TimeoutKind string

const (
	TimeoutNone    TimeoutKind = ""
	TimeoutFrames  TimeoutKind = "frames"
	TimeoutCycles  TimeoutKind = "cycles"
	TimeoutWall    TimeoutKind = "wall_clock"
	TimeoutContext TimeoutKind = "context"
)

// InputEvent changes controller one state before a relative frame starts.
type InputEvent struct {
	Frame   uint64
	Buttons controller.Button
}

// RAMCondition compares one internal CPU RAM byte.
type RAMCondition struct {
	Address uint16
	Value   byte
}

// RAMRange selects internal CPU RAM for side-effect-free capture.
type RAMRange struct {
	Start  uint16
	Length uint16
}

// RAMCapture contains one selected internal CPU RAM range.
type RAMCapture struct {
	Start uint16
	Data  []byte
}

// RunOptions defines deterministic inputs and independent execution limits.
type RunOptions struct {
	Boot BootPolicy

	Frames    uint64
	MaxCycles uint64
	Timeout   time.Duration

	Inputs     []InputEvent
	Completion *RAMCondition
	Assertions []RAMCondition
	CaptureRAM []RAMRange
}

// RunResult contains the state and captures at a deterministic stop boundary.
type RunResult struct {
	Reason  ExitReason
	Timeout TimeoutKind
	Error   string

	CPUCycles       uint64
	CompletedFrames uint64
	Frame           uint64
	PC              uint16
	SP              byte

	Image *image.RGBA
	RAM   []RAMCapture
}

// Run executes without pacing until success or an independent limit stops it.
func (sys *System) Run(ctx context.Context, opts RunOptions) RunResult {
	if err := validateRunOptions(opts); err != nil {
		return finishRun(sys, ExitInvalidOptions, TimeoutNone, err, sys.CPU.Cycles(), 0, nil, opts.CaptureRAM)
	}
	if opts.Boot == BootReset {
		sys.Reset()
	}

	runContext := ctx
	cancel := func() {}
	if opts.Timeout > 0 {
		runContext, cancel = context.WithTimeout(ctx, opts.Timeout)
	}
	defer cancel()

	startCycles := sys.CPU.Cycles()
	completedFrames := uint64(0)
	inputIndex := applyInputEvents(sys, opts.Inputs, 0, 0)
	var completedImage *image.RGBA

	for {
		cycles := sys.CPU.Cycles() - startCycles
		if kind, err, stopped := checkRunLimits(ctx, runContext, cycles, opts.MaxCycles, true); stopped {
			return finishRun(sys, ExitTimeout, kind, err, startCycles, completedFrames, completedImage,
				opts.CaptureRAM)
		}

		step, err := sys.StepSystem()
		if err != nil {
			return finishRun(sys, ExitInvalidStep, TimeoutNone, err, startCycles, completedFrames, completedImage,
				opts.CaptureRAM)
		}
		cycles = sys.CPU.Cycles() - startCycles
		if kind, err, stopped := checkRunLimits(ctx, runContext, cycles, opts.MaxCycles, false); stopped {
			return finishRun(sys, ExitTimeout, kind, err, startCycles, completedFrames, completedImage,
				opts.CaptureRAM)
		}
		if !step.FrameCompleted {
			continue
		}

		completedFrames++
		completedImage = cloneRGBA(sys.Image())
		if opts.Completion == nil && completedFrames == opts.Frames {
			return checkRunAssertions(sys, opts, startCycles, completedFrames, completedImage)
		}
		if opts.Completion != nil && matchesRAM(sys, *opts.Completion) {
			return checkRunAssertions(sys, opts, startCycles, completedFrames, completedImage)
		}
		if completedFrames == opts.Frames {
			return finishRun(sys, ExitTimeout, TimeoutFrames, nil, startCycles, completedFrames, completedImage,
				opts.CaptureRAM)
		}

		inputIndex = applyInputEvents(sys, opts.Inputs, inputIndex, completedFrames)
	}
}

func checkRunLimits(ctx, runContext context.Context, cycles, maximum uint64,
	inclusive bool) (TimeoutKind, error, bool) {

	if err := ctx.Err(); err != nil {
		return TimeoutContext, fmt.Errorf("run context stopped: %w", err), true
	}
	if err := runContext.Err(); err != nil {
		return TimeoutWall, fmt.Errorf("wall-clock timeout: %w", err), true
	}
	if cycles > maximum || (inclusive && cycles == maximum) {
		return TimeoutCycles, nil, true
	}

	return TimeoutNone, nil, false
}

func validateRunOptions(opts RunOptions) error {
	if opts.Boot != BootCurrent && opts.Boot != BootReset {
		return fmt.Errorf("invalid boot policy: %d", opts.Boot)
	}
	if opts.Frames == 0 {
		return errors.New("frame limit must be positive")
	}
	if opts.MaxCycles == 0 {
		return errors.New("cycle limit must be positive")
	}
	if opts.Timeout < 0 {
		return errors.New("wall-clock timeout cannot be negative")
	}
	if err := validateInputEvents(opts.Inputs, opts.Frames); err != nil {
		return err
	}
	if err := validateRAMSelections(opts); err != nil {
		return err
	}

	return nil
}

func validateInputEvents(events []InputEvent, frames uint64) error {
	for index, event := range events {
		if event.Frame >= frames {
			return fmt.Errorf("input event frame %d is outside the frame limit", event.Frame)
		}
		if index > 0 && event.Frame <= events[index-1].Frame {
			return errors.New("input event frames must be unique and increasing")
		}
	}

	return nil
}

func validateRAMSelections(opts RunOptions) error {
	if opts.Completion != nil {
		if err := validateRAMAddress(opts.Completion.Address); err != nil {
			return fmt.Errorf("completion marker: %w", err)
		}
	}
	for _, assertion := range opts.Assertions {
		if err := validateRAMAddress(assertion.Address); err != nil {
			return fmt.Errorf("assertion: %w", err)
		}
	}
	for _, selected := range opts.CaptureRAM {
		if selected.Length == 0 {
			return errors.New("RAM capture length must be positive")
		}
		end := uint32(selected.Start) + uint32(selected.Length)
		if end > 0x2000 {
			return fmt.Errorf("RAM capture 0x%04x+%d is outside internal RAM", selected.Start, selected.Length)
		}
	}

	return nil
}

func validateRAMAddress(address uint16) error {
	if address >= 0x2000 {
		return fmt.Errorf("address 0x%04x is outside internal RAM", address)
	}

	return nil
}

func applyInputEvents(sys *System, events []InputEvent, index int, frame uint64) int {
	if index >= len(events) || events[index].Frame != frame {
		return index
	}

	buttons := events[index].Buttons
	sys.Bus.Controller1.SetButtonState(controller.Button(0xff), false)
	sys.Bus.Controller1.SetButtonState(buttons, true)
	return index + 1
}

func checkRunAssertions(sys *System, opts RunOptions, startCycles, completedFrames uint64,
	completedImage *image.RGBA) RunResult {

	for _, assertion := range opts.Assertions {
		actual, _ := sys.InspectRAM(assertion.Address)
		if actual != assertion.Value {
			err := fmt.Errorf("RAM 0x%04x = 0x%02x, want 0x%02x", assertion.Address, actual, assertion.Value)
			return finishRun(sys, ExitAssertionFailed, TimeoutNone, err, startCycles, completedFrames,
				completedImage, opts.CaptureRAM)
		}
	}

	return finishRun(sys, ExitSuccess, TimeoutNone, nil, startCycles, completedFrames, completedImage,
		opts.CaptureRAM)
}

func matchesRAM(sys *System, condition RAMCondition) bool {
	actual, _ := sys.InspectRAM(condition.Address)
	return actual == condition.Value
}

func finishRun(sys *System, reason ExitReason, timeout TimeoutKind, runErr error, startCycles, completedFrames uint64,
	completedImage *image.RGBA, ranges []RAMRange) RunResult {

	result := RunResult{
		Reason:          reason,
		Timeout:         timeout,
		CPUCycles:       sys.CPU.Cycles() - startCycles,
		CompletedFrames: completedFrames,
		Frame:           sys.Bus.PPU.Frame(),
		PC:              sys.PC,
		SP:              sys.SP,
		Image:           completedImage,
		RAM:             captureRAM(sys, ranges),
	}
	if runErr != nil {
		result.Error = runErr.Error()
	}

	return result
}

func captureRAM(sys *System, ranges []RAMRange) []RAMCapture {
	captures := make([]RAMCapture, 0, len(ranges))
	for _, selected := range ranges {
		data := make([]byte, selected.Length)
		for offset := range selected.Length {
			data[offset], _ = sys.InspectRAM(selected.Start + offset)
		}
		captures = append(captures, RAMCapture{
			Start: selected.Start,
			Data:  data,
		})
	}

	return captures
}

func cloneRGBA(source *image.RGBA) *image.RGBA {
	cloned := image.NewRGBA(source.Bounds())
	draw.Draw(cloned, cloned.Bounds(), source, source.Bounds().Min, draw.Src)
	return cloned
}
