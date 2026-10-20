package nes

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
	"github.com/retroenv/retrogolib/assert"
)

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
		Kind:  BreakpointExec,
		Start: 2,
		End:   2,
		Once:  true,
	})

	result := sys.RunUntil(t.Context(), RunUntilOptions{
		Breakpoints:     set,
		MaxInstructions: 100,
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
		Kind:  BreakpointWrite,
		Start: 0x10,
		End:   0x10,
	})

	result := sys.RunUntil(t.Context(), RunUntilOptions{
		Breakpoints:     set,
		MaxInstructions: 10,
	})

	assert.Equal(t, StopBreakpoint, result.Reason)
	assert.Equal(t, uint16(0x10), result.Hit.Address)
}

func TestRunUntilInstructionLimit(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0xea) // NOP
	sys.PC = 0

	result := sys.RunUntil(t.Context(), RunUntilOptions{MaxInstructions: 3})

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

func TestCDLRecorderMarksCode(t *testing.T) {
	cart := cartridge.New()
	copy(cart.PRG, []byte{0xa9, 0x2a, 0xea, 0x4c, 0x00, 0x80})
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	sys.PC = 0x8000

	recorder := sys.NewCDLRecorder()
	recorder.Start()
	_, err = sys.StepSystem()
	recorder.Stop()
	assert.NoError(t, err)

	prg := recorder.PRG()
	assert.True(t, prg[0]&0x01 != 0)
	assert.True(t, prg[1]&0x01 != 0)

	coverage := recorder.Coverage()
	assert.NotEmpty(t, coverage)
	assert.Equal(t, 2, coverage[0].Code)
}

func TestCDLRecorderIgnoresWritesAndMarksMirroredPPUDataReads(t *testing.T) {
	cart := cartridge.New()
	program := []byte{
		0xa9, 0x00, // LDA #$00
		0x8d, 0x06, 0x20, // STA $2006
		0xa9, 0x10, // LDA #$10
		0x8d, 0x06, 0x20, // STA $2006
		0xad, 0x0f, 0x20, // LDA $200F, a mirror of $2007
		0x8d, 0x00, 0x90, // STA $9000, a write into the PRG range
		0xea, // NOP
	}
	copy(cart.PRG, program)
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	sys.PC = 0x8000

	recorder := sys.NewCDLRecorder()
	recorder.Start()
	for range 7 {
		_, err = sys.StepSystem()
		assert.NoError(t, err)
	}
	recorder.Stop()

	assert.Equal(t, codedatalog.PrgFlag(0), recorder.PRG()[0x1000])
	assert.True(t, len(recorder.CHR()) > 0x10)
	assert.Equal(t, codedatalog.ReadProgrammatically, recorder.CHR()[0x10])
}

func TestCDLRecorderClearsIndirectDataOfFetchedCode(t *testing.T) {
	cart := cartridge.New()
	// LDA $9000,X is an indexed read; the NOP after it is fetched as code.
	copy(cart.PRG, []byte{0xbd, 0x00, 0x90, 0xea})
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	sys.PC = 0x8000

	recorder := sys.NewCDLRecorder()
	recorder.Start()
	for range 2 {
		_, err = sys.StepSystem()
		assert.NoError(t, err)
	}
	recorder.Stop()

	prg := recorder.PRG()
	assert.Equal(t, codedatalog.Code, prg[3])
	assert.Equal(t, codedatalog.IndirectData, prg[0x1000])
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
			{Frame: 0, Controller: 1, Buttons: controller.A},
			{Frame: 0, Controller: 2, Buttons: controller.B},
			{Frame: 1, Controller: 1},
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
		Kind:  BreakpointWrite,
		Start: 0x10,
		End:   0x10,
	})
	result := sys.RunUntil(t.Context(), RunUntilOptions{
		Breakpoints:     set,
		MaxInstructions: 10,
	})

	assert.Equal(t, StopBreakpoint, result.Reason)
	assert.Equal(t, uint16(0x10), result.Hit.Address)
	assert.Equal(t, uint16(5), result.Hit.PC)
}
