package nes

import (
	"strings"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

var disassemblyOperandCases = []struct {
	bytes []byte
	name  string
	want  string
}{
	{
		bytes: []byte{0xea},
		name:  "implied",
		want:  "NOP",
	},
	{
		bytes: []byte{0x0a},
		name:  "accumulator",
		want:  "ASL A",
	},
	{
		bytes: []byte{0xa9, 0x2a},
		name:  "immediate",
		want:  "LDA #$2A",
	},
	{
		bytes: []byte{0xa5, 0x10},
		name:  "zero page",
		want:  "LDA $10",
	},
	{
		bytes: []byte{0xb5, 0x20},
		name:  "zero page x",
		want:  "LDA $20,X",
	},
	{
		bytes: []byte{0xb6, 0x20},
		name:  "zero page y",
		want:  "LDX $20,Y",
	},
	{
		bytes: []byte{0xad, 0x00, 0x02},
		name:  "absolute",
		want:  "LDA $0200",
	},
	{
		bytes: []byte{0xbd, 0x00, 0x02},
		name:  "absolute x",
		want:  "LDA $0200,X",
	},
	{
		bytes: []byte{0xb9, 0x00, 0x02},
		name:  "absolute y",
		want:  "LDA $0200,Y",
	},
	{
		bytes: []byte{0x6c, 0x34, 0x12},
		name:  "indirect",
		want:  "JMP ($1234)",
	},
	{
		bytes: []byte{0x01, 0x34},
		name:  "indirect x",
		want:  "ORA ($34,X)",
	},
	{
		bytes: []byte{0x11, 0x10},
		name:  "indirect y",
		want:  "ORA ($10),Y",
	},
	{
		bytes: []byte{0xd0, 0xfe},
		name:  "relative backward",
		want:  "BNE $0010",
	},
	{
		bytes: []byte{0xf0, 0x04},
		name:  "relative forward",
		want:  "BEQ $0016",
	},
}

func TestRunUntilExecBreakpoint(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0xa9) // LDA #$2A
	sys.Bus.Memory.Write(1, 0x2a)
	sys.Bus.Memory.Write(2, 0xea) // NOP
	sys.Bus.Memory.Write(3, 0x4c) // JMP $0000
	sys.Bus.Memory.Write(4, 0x00)
	sys.Bus.Memory.Write(5, 0x00)
	sys.PC = 0

	set := NewBreakpointSet()
	id := set.Add(Breakpoint{
		End:   2,
		Kind:  BreakpointExec,
		Once:  true,
		Start: 2,
	})

	result := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 100,

		Breakpoints: set,
	})

	assert.Equal(t, StopBreakpoint, result.Reason)
	assert.Equal(t, id, result.Hit.ID)
	assert.Equal(t, uint16(2), result.PC)
	assert.Equal(t, uint64(1), result.Instructions)
	assert.Equal(t, 0, set.Len())
}

func TestRunUntilWriteBreakpoint(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0x85) // STA $10
	sys.Bus.Memory.Write(1, 0x10)
	sys.PC = 0

	set := NewBreakpointSet()
	set.Add(Breakpoint{
		End:   0x10,
		Kind:  BreakpointWrite,
		Start: 0x10,
	})

	result := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 10,

		Breakpoints: set,
	})

	assert.Equal(t, StopBreakpoint, result.Reason)
	assert.Equal(t, uint16(0x10), result.Hit.Address)
}

func TestRunUntilInstructionLimit(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0xea) // NOP
	sys.PC = 0

	result := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 3,
	})

	assert.Equal(t, StopInstructions, result.Reason)
	assert.Equal(t, uint64(3), result.Instructions)
}

func TestDisassemble(t *testing.T) {
	cart := cartridge.New()
	copy(cart.PRG, []byte{0xa9, 0x2a, 0x8d, 0x00, 0x80, 0x4c, 0x00, 0x80})
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	instructions := sys.Disassemble(0x8000, 3)

	assert.Len(t, instructions, 3)
	assert.Equal(t, "LDA", instructions[0].Mnemonic)
	assert.Equal(t, "#$2A", instructions[0].Operand)
	assert.Equal(t, 2, instructions[0].Size)
	assert.Equal(t, "STA", instructions[1].Mnemonic)
	assert.Equal(t, "$8000", instructions[1].Operand)
	assert.Equal(t, "JMP", instructions[2].Mnemonic)
	assert.Equal(t, 0, instructions[0].Location.PRGOffset)
}

func TestStepTracedFillsRing(t *testing.T) {
	ring := NewTraceRing(10)
	ring.SetEnabled(true)

	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New()), WithTraceRing(ring)))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0xa9) // LDA #$2A
	sys.Bus.Memory.Write(1, 0x2a)
	sys.PC = 0

	_, err = sys.StepTraced()
	assert.NoError(t, err)

	lines := ring.Lines()
	assert.Len(t, lines, 1)
	assert.Contains(t, lines[0], "LDA #$2A")
	assert.Contains(t, lines[0], "A9 2A")
}

func TestTraceRingLinesSinceSurvivesWrap(t *testing.T) {
	ring := NewTraceRing(3)
	ring.SetEnabled(true)

	for _, line := range []string{"a", "b", "c", "d", "e"} {
		ring.add(line)
	}

	assert.Equal(t, uint64(5), ring.Total())
	assert.Equal(t, 3, ring.Len())
	assert.Equal(t, []string{"c", "d", "e"}, ring.LinesSince(0))
	assert.Equal(t, []string{"d", "e"}, ring.LinesSince(3))
	assert.Equal(t, []string{"e"}, ring.LinesSince(4))
	assert.Empty(t, ring.LinesSince(5))
}

func TestStepTracedDisabledRingFormatsNoOperands(t *testing.T) {
	ring := NewTraceRing(10)
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New()), WithTraceRing(ring)))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0xea)
	sys.PC = 0

	_, err = sys.StepTraced()
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), ring.Total())
	assert.Empty(t, sys.CPU.TraceStep.CustomData)
}

func TestRunUntilAppliesEveryInputOfAFrame(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0x4c) // JMP $0000
	sys.Bus.Memory.Write(1, 0x00)
	sys.Bus.Memory.Write(2, 0x00)
	sys.PC = 0

	var applied []InputEvent
	result := sys.RunUntil(t.Context(), RunUntilOptions{
		Frames: 2,

		Inputs: []InputEvent{
			{
				Frame:      0,
				Controller: 1,
				Buttons:    controller.A,
			},
			{
				Frame:      0,
				Controller: 2,
				Buttons:    controller.B,
			},
			{
				Frame:      1,
				Controller: 1,
			},
		},
		OnInput: func(event InputEvent) { applied = append(applied, event) },
	})

	assert.Equal(t, StopFrames, result.Reason)
	assert.Len(t, applied, 3)
	assert.Equal(t, 2, applied[1].Controller)
}

func TestRunUntilMemoryBreakpointReportsInstructionAddress(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(5, 0x85) // STA $10
	sys.Bus.Memory.Write(6, 0x10)
	sys.PC = 5

	set := NewBreakpointSet()
	set.Add(Breakpoint{
		End:   0x10,
		Kind:  BreakpointWrite,
		Start: 0x10,
	})
	result := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 10,

		Breakpoints: set,
	})

	assert.Equal(t, StopBreakpoint, result.Reason)
	assert.Equal(t, uint16(0x10), result.Hit.Address)
	assert.Equal(t, uint16(5), result.Hit.PC)
}

func TestRunUntilSkipExecAtStartContinuesFromBreakpoint(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0xea) // NOP
	sys.Bus.Memory.Write(1, 0xea) // NOP
	sys.Bus.Memory.Write(2, 0x4c) // JMP $0000
	sys.Bus.Memory.Write(3, 0x00)
	sys.Bus.Memory.Write(4, 0x00)
	sys.PC = 1

	set := NewBreakpointSet()
	set.Add(Breakpoint{
		End:   1,
		Kind:  BreakpointExec,
		Start: 1,
	})

	stuck := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 100,

		Breakpoints: set,
	})
	assert.Equal(t, StopBreakpoint, stuck.Reason)
	assert.Equal(t, BreakpointExec, stuck.Hit.Kind)
	assert.Equal(t, uint64(0), stuck.Instructions)

	// A second run without the option stops at the same place.
	again := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 100,

		Breakpoints: set,
	})
	assert.Equal(t, uint64(0), again.Instructions)

	// The option executes the first instruction and the loop reaches the
	// breakpoint again.
	resumed := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 100,

		Breakpoints:     set,
		SkipExecAtStart: true,
	})
	assert.Equal(t, StopBreakpoint, resumed.Reason)
	assert.Equal(t, uint64(3), resumed.Instructions)
	assert.Equal(t, uint16(1), resumed.PC)
}

func TestRunUntilReadBreakpointReportsRunningInstructionAddress(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(5, 0xea) // NOP
	sys.Bus.Memory.Write(6, 0xea) // NOP
	sys.PC = 5

	set := NewBreakpointSet()
	set.Add(Breakpoint{
		End:   6,
		Kind:  BreakpointRead,
		Start: 6,
	})
	result := sys.RunUntil(t.Context(), RunUntilOptions{
		MaxInstructions: 10,

		Breakpoints: set,
	})

	assert.Equal(t, StopBreakpoint, result.Reason)
	assert.Equal(t, uint16(6), result.Hit.Address)
	assert.Equal(t, uint16(5), result.Hit.PC)
}

func TestDisassembleOperandNotation(t *testing.T) {
	for _, tt := range disassemblyOperandCases {
		t.Run(tt.name, func(t *testing.T) {
			sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
			assert.NoError(t, err)
			for index, value := range tt.bytes {
				sys.Bus.Memory.Write(0x10+uint16(index), value)
			}

			instructions := sys.Disassemble(0x10, 1)

			assert.Len(t, instructions, 1)
			assert.Len(t, tt.bytes, instructions[0].Size)
			text := strings.TrimSpace(instructions[0].Mnemonic + " " + instructions[0].Operand)
			assert.Equal(t, tt.want, text)
		})
	}
}

func TestDisassembleClampsCount(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	assert.Empty(t, sys.Disassemble(0x10, -1))
	assert.Empty(t, sys.Disassemble(0x10, 0))
	assert.True(t, len(sys.Disassemble(0x8000, 1<<30)) <= maxDisassembleCount)
}
