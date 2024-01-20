package cpu

import (
	"fmt"
	"strings"

	. "github.com/retroenv/retrogolib/addressing"
	"github.com/retroenv/retrogolib/arch/nes"
	"github.com/retroenv/retrogolib/cpu"
)

// TracingMode defines a tracing mode.
type TracingMode int

// tracing modes, either disabled or in emulator mode.
const (
	NoTracing TracingMode = iota
	EmulatorTracing
)

// TraceStep contains all info needed to print a trace step.
type TraceStep struct {
	PC             uint16
	Opcode         []byte
	Addressing     Mode
	Timing         byte
	PageCrossCycle bool
	PageCrossed    bool
	Unofficial     bool
	Instruction    string
}

// print outputs current trace step in Nintendulator / nestest.log compatible format.
func (t TraceStep) print(cpu *CPU) {
	var opcodes [3]string
	for i := 0; i < 3; i++ {
		s := "  "
		if i < len(t.Opcode) {
			op := t.Opcode[i]
			s = fmt.Sprintf("%02X", op)
		}

		opcodes[i] = s
	}
	unofficial := " "
	if t.Unofficial {
		unofficial = "*"
	}

	s := fmt.Sprintf("%04X  %s %s %s %s%-31s A:%02X X:%02X Y:%02X P:%02X SP:%02X CYC:%d\n",
		t.PC, opcodes[0], opcodes[1], opcodes[2], unofficial, t.Instruction,
		cpu.A, cpu.X, cpu.Y, cpu.GetFlags(), cpu.SP, cpu.cycles)
	_, _ = fmt.Fprint(cpu.tracingTarget, s)
}

// Trace logs the trace information of the passed instruction and its parameters.
// Params can be of length 0 to 2.
func (c *CPU) trace(instruction *cpu.Instruction, params ...any) {
	paramsAsString := c.ParamString(instruction, params...)

	c.TraceStep.Unofficial = instruction.Unofficial
	c.TraceStep.Instruction = strings.ToUpper(instruction.Name)
	if paramsAsString != "" {
		c.TraceStep.Instruction += " " + paramsAsString
	}
	c.TraceStep.print(c)
}

func shouldOutputMemoryContent(address uint16) bool {
	switch {
	case address < 0x0800:
		return true
	case address >= 0x4000 && address <= 0x4020:
		return true
	case address >= nes.CodeBaseAddress:
		return true
	default:
		return false
	}
}

func addressModeFromCallNoParam(instruction *cpu.Instruction) Mode {
	if instruction.HasAddressing(AccumulatorAddressing) {
		return AccumulatorAddressing
	}
	// branches have no target in go mode
	if instruction.HasAddressing(RelativeAddressing) {
		return RelativeAddressing
	}
	return ImpliedAddressing
}
