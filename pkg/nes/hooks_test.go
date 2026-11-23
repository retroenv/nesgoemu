package nes

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestAddPreExecutionHook(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	var calls int
	remove := sys.AddPreExecutionHook(func(*cpu6502.CPU, *cpu6502.Instruction, ...any) {
		calls++
	})

	sys.Bus.Memory.Write(0, 0xea)
	sys.PC = 0
	_, err = sys.StepSystem()
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)

	remove()

	sys.Bus.Memory.Write(0, 0xea)
	sys.PC = 0
	_, err = sys.StepSystem()
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestAddBusCycleObserver(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	var cycles []cpu6502.BusCycle
	remove := sys.AddBusCycleObserver(func(cycle cpu6502.BusCycle) {
		cycles = append(cycles, cycle)
	})
	defer remove()

	sys.Bus.Memory.Write(0, 0xea)
	sys.PC = 0
	_, err = sys.StepSystem()
	assert.NoError(t, err)
	assert.NotEmpty(t, cycles)
}

func TestAddStackHook(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	var events []cpu6502.StackEvent
	remove := sys.AddStackHook(func(event cpu6502.StackEvent) {
		events = append(events, event)
	})
	defer remove()

	// PHA pushes the accumulator onto the stack.
	sys.Bus.Memory.Write(0, 0x48)
	sys.PC = 0
	_, err = sys.StepSystem()
	assert.NoError(t, err)
	assert.NotEmpty(t, events)
	assert.Equal(t, cpu6502.StackPush, events[0].Operation)
}

// TestBusCycleObserverSkipsDMAStalledRead checks that a read which OAM DMA
// takes over is reported once. The CPU repeats the read until the DMA ends, so
// an observer on each attempt would count the opcode fetch many times.
func TestBusCycleObserverSkipsDMAStalledRead(t *testing.T) {
	cart := cartridge.New()
	// LDA #$02; STA $4014; LDA $10
	copy(cart.PRG, []byte{0xa9, 0x02, 0x8d, 0x14, 0x40, 0xa5, 0x10})
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	sys.PC = 0x8000

	const nextOpcode = 0x8005
	var fetches int
	remove := sys.AddBusCycleObserver(func(cycle cpu6502.BusCycle) {
		if cycle.Address == nextOpcode && !cycle.Write {
			fetches++
		}
	})
	defer remove()

	for range 3 {
		_, err = sys.StepSystem()
		assert.NoError(t, err)
	}
	assert.Equal(t, 1, fetches)
}

func TestReadBreakpointUsesBusValueAndSeesDMA(t *testing.T) {
	const (
		dmaAddress        = 0x210
		dmaPage           = 2
		ldaZeroPageOpcode = 0xa5
		nopOpcode         = 0xea
		readAddress       = 0x10
		readValue         = 0x42
	)

	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	// LDA reads the test byte from zero page.
	sys.Bus.Memory.Write(0, ldaZeroPageOpcode)
	sys.Bus.Memory.Write(1, readAddress)
	sys.Bus.Memory.Write(readAddress, readValue)
	sys.PC = 0

	points := NewBreakpointSet()
	value := byte(readValue)
	id := points.Add(Breakpoint{
		Kind:  BreakpointRead,
		Start: readAddress,
		End:   readAddress,
		Value: &value,
	})
	result := sys.RunUntil(t.Context(), RunUntilOptions{
		Breakpoints:     points,
		MaxInstructions: 1,
	})
	assert.Equal(t, id, result.Hit.ID)
	assert.Equal(t, byte(readValue), result.Hit.Value)

	// Start OAM DMA and check for a breakpoint on the source read.
	sys.Bus.Memory.Write(0, nopOpcode)
	sys.PC = 0
	sys.Bus.Memory.Write(register.OAM_DMA, dmaPage)
	sys.Bus.Memory.Write(dmaAddress, readValue)

	points = NewBreakpointSet()
	id = points.Add(Breakpoint{
		Kind:  BreakpointRead,
		Start: dmaAddress,
		End:   dmaAddress,
		Value: &value,
	})
	result = sys.RunUntil(t.Context(), RunUntilOptions{
		Breakpoints:     points,
		MaxInstructions: 1,
	})
	assert.Equal(t, id, result.Hit.ID)
	assert.Equal(t, byte(readValue), result.Hit.Value)
}

func TestAccessBreakpointValueFiltersOnlyWrites(t *testing.T) {
	const (
		readAddress = 0x10
		readValue   = 0x42
	)

	points := NewBreakpointSet()
	value := byte(readValue)
	points.Add(Breakpoint{
		Kind:  BreakpointAccess,
		Start: readAddress,
		End:   readAddress,
		Value: &value,
	})

	_, readHit := points.matchMemory(false, readAddress, 0, -1, 0, 0)
	_, wrongWriteHit := points.matchMemory(true, readAddress, 0, -1, 0, 0)
	_, writeHit := points.matchMemory(true, readAddress, value, -1, 0, 0)
	assert.True(t, readHit)
	assert.False(t, wrongWriteHit)
	assert.True(t, writeHit)
}

func TestBreakpointSetRejectsInvalidDefinitions(t *testing.T) {
	points := NewBreakpointSet()
	value := byte(0)
	for _, bp := range []Breakpoint{
		{
			Start: 2,
			End:   1,
		},
		{
			Kind: BreakpointKind(255),
		},
		{
			Kind:  BreakpointExec,
			Value: &value,
		},
	} {
		assert.Equal(t, BreakpointID(0), points.Add(bp))
	}
	assert.Empty(t, points.List())
}

func TestJammedCPURunStopsAndDoesNotCountInstructions(t *testing.T) {
	const kilOpcode = 0x02

	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, kilOpcode)
	sys.PC = 0
	result := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 10,
	})
	assert.Equal(t, StopJam, result.Reason)
	assert.Equal(t, uint64(1), result.Instructions)

	step, err := sys.StepSystem()
	assert.NoError(t, err)
	assert.False(t, step.InstructionExecuted)
}

func TestCPUHooksAreAbsentUntilNeeded(t *testing.T) {
	ring := NewTraceRing(0)
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New()), WithTraceRing(ring)))
	assert.NoError(t, err)
	assert.Nil(t, sys.CPU.PreExecutionHook())
	assert.Nil(t, sys.CPU.StackEventHook())
	remove := sys.AddPreExecutionHook(func(*cpu6502.CPU, *cpu6502.Instruction, ...any) {})
	assert.NotNil(t, sys.CPU.PreExecutionHook())
	remove()
	assert.Nil(t, sys.CPU.PreExecutionHook())
	ring.SetEnabled(true)
	assert.NotNil(t, sys.CPU.PreExecutionHook())
	ring.SetEnabled(false)
	assert.Nil(t, sys.CPU.PreExecutionHook())
}

func TestConfiguredCPUHooksSurviveRuntimeRegistration(t *testing.T) {
	const (
		phaOpcode = 0x48
		pushCount = 2
	)

	var pre, stack int
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New()), WithCPUOptions(
		cpu6502.WithPreExecutionHook(func(*cpu6502.CPU, *cpu6502.Instruction, ...any) {
			pre++
		}),
		cpu6502.WithStackEventHook(func(cpu6502.StackEvent) {
			stack++
		}),
	)))
	assert.NoError(t, err)
	remove := sys.AddPreExecutionHook(func(*cpu6502.CPU, *cpu6502.Instruction, ...any) {})

	for range pushCount {
		sys.Bus.Memory.Write(0, phaOpcode)
		sys.PC = 0
		_, err = sys.StepSystem()
		assert.NoError(t, err)
		remove()
	}

	assert.Equal(t, pushCount, pre)
	assert.Equal(t, pushCount, stack)
}
