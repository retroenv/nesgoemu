package nes

import (
	"context"
	"testing"
	"time"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const testMarkerAddress = 0x07f0

func TestRunCapturesCompletedFrameAndRAM(t *testing.T) {
	sys := newRunnerTestSystem(t, true)
	condition := RAMCondition{
		Address: testMarkerAddress,
		Value:   0xa5,
	}

	result := sys.Run(context.Background(), RunOptions{
		Frames:     2,
		MaxCycles:  100_000,
		Completion: &condition,
		Assertions: []RAMCondition{{Address: testMarkerAddress, Value: 0xa5}},
		CaptureRAM: []RAMRange{{Start: testMarkerAddress, Length: 1}},
	})

	assert.Equal(t, ExitSuccess, result.Reason)
	assert.Equal(t, uint64(1), result.CompletedFrames)
	assert.NotNil(t, result.Image)
	assert.Equal(t, []byte{0xa5}, result.RAM[0].Data)
}

func TestRunReportsFrameTimeoutWhenCompletionIsMissing(t *testing.T) {
	sys := newRunnerTestSystem(t, false)
	condition := RAMCondition{
		Address: testMarkerAddress,
		Value:   0xa5,
	}

	result := sys.Run(context.Background(), RunOptions{
		Frames:     1,
		MaxCycles:  100_000,
		Completion: &condition,
	})

	assert.Equal(t, ExitTimeout, result.Reason)
	assert.Equal(t, TimeoutFrames, result.Timeout)
	assert.Equal(t, uint64(1), result.CompletedFrames)
}

func TestRunReportsAssertionFailure(t *testing.T) {
	sys := newRunnerTestSystem(t, true)

	result := sys.Run(context.Background(), RunOptions{
		Frames:     1,
		MaxCycles:  100_000,
		Assertions: []RAMCondition{{Address: testMarkerAddress, Value: 0x5a}},
	})

	assert.Equal(t, ExitAssertionFailed, result.Reason)
	assert.NotEqual(t, "", result.Error)
}

func TestRunReportsCycleTimeout(t *testing.T) {
	sys := newRunnerTestSystem(t, false)

	result := sys.Run(context.Background(), RunOptions{
		Frames:    1,
		MaxCycles: 1,
	})

	assert.Equal(t, ExitTimeout, result.Reason)
	assert.Equal(t, TimeoutCycles, result.Timeout)
	assert.Equal(t, uint64(0), result.CompletedFrames)
}

func TestRunDoesNotCompletePastCycleLimit(t *testing.T) {
	baselineSystem := newRunnerTestSystem(t, false)
	baseline := baselineSystem.Run(context.Background(), RunOptions{
		Frames:    1,
		MaxCycles: 100_000,
	})
	assert.Equal(t, ExitSuccess, baseline.Reason)

	limitedSystem := newRunnerTestSystem(t, false)
	limited := limitedSystem.Run(context.Background(), RunOptions{
		Frames:    1,
		MaxCycles: baseline.CPUCycles - 1,
	})

	assert.Equal(t, ExitTimeout, limited.Reason)
	assert.Equal(t, TimeoutCycles, limited.Timeout)
}

func TestRunReportsCallerDeadlineAsContextTimeout(t *testing.T) {
	sys := newRunnerTestSystem(t, false)
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()

	result := sys.Run(ctx, RunOptions{
		Frames:    1,
		MaxCycles: 100_000,
	})

	assert.Equal(t, ExitTimeout, result.Reason)
	assert.Equal(t, TimeoutContext, result.Timeout)
}

func TestRunReportsConfiguredWallClockTimeout(t *testing.T) {
	sys := newRunnerTestSystem(t, false)

	result := sys.Run(context.Background(), RunOptions{
		Frames:    1,
		MaxCycles: 100_000,
		Timeout:   time.Nanosecond,
	})

	assert.Equal(t, ExitTimeout, result.Reason)
	assert.Equal(t, TimeoutWall, result.Timeout)
}

func TestRunReportsInvalidOptions(t *testing.T) {
	sys := newRunnerTestSystem(t, false)

	result := sys.Run(context.Background(), RunOptions{})

	assert.Equal(t, ExitInvalidOptions, result.Reason)
	assert.Equal(t, TimeoutNone, result.Timeout)
	assert.NotEqual(t, "", result.Error)
}

func TestRunAppliesInputAtFrameBoundary(t *testing.T) {
	sys := newRunnerTestSystem(t, false)

	result := sys.Run(context.Background(), RunOptions{
		Frames:    2,
		MaxCycles: 100_000,
		Inputs: []InputEvent{
			{Frame: 0, Buttons: controller.A},
			{Frame: 1, Buttons: controller.B},
		},
	})

	assert.Equal(t, ExitSuccess, result.Reason)
	sys.Bus.Controller1.SetStrobeMode(1)
	sys.Bus.Controller1.SetStrobeMode(0)
	assert.Equal(t, uint8(0), sys.Bus.Controller1.Read())
	assert.Equal(t, uint8(1), sys.Bus.Controller1.Read())
}

func TestRunResetPolicyUsesResetVector(t *testing.T) {
	sys := newRunnerTestSystem(t, true)
	sys.PC = 0x9000

	result := sys.Run(context.Background(), RunOptions{
		Boot:      BootReset,
		Frames:    1,
		MaxCycles: 100_000,
	})

	assert.Equal(t, ExitSuccess, result.Reason)
	value, ok := sys.InspectRAM(testMarkerAddress)
	assert.True(t, ok)
	assert.Equal(t, byte(0xa5), value)
}

func newRunnerTestSystem(t *testing.T, writeMarker bool) *System {
	t.Helper()

	cart := cartridge.New()
	cart.PRG = make([]byte, 32*1024)
	cart.CHR = make([]byte, 8*1024)
	if writeMarker {
		copy(cart.PRG, []byte{0xa9, 0xa5, 0x8d, 0xf0, 0x07, 0x4c, 0x05, 0x80})
	} else {
		copy(cart.PRG, []byte{0x4c, 0x00, 0x80})
	}
	cart.PRG[0x7ffc] = 0x00
	cart.PRG[0x7ffd] = 0x80

	sys, err := NewSystem(NewOptions(WithCartridge(cart), WithSavePath("")))
	assert.NoError(t, err)
	return sys
}
