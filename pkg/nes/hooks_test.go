package nes

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
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
