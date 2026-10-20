package nes

import (
	"fmt"
	"strings"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
)

// printTraceStep outputs the current trace step in the Nintendulator and
// nestest.log compatible format. A trace ring receives the line when one is
// attached; otherwise the line is written to the tracing target.
func (sys *System) printTraceStep(state cpuState) {
	step := sys.CPU.TraceStep

	var opcodes [cpu6502.MaxOpcodeSize]string
	for i := range cpu6502.MaxOpcodeSize {
		s := "  "
		if i < len(step.OpcodeOperands) {
			op := step.OpcodeOperands[i]
			s = fmt.Sprintf("%02X", op)
		}

		opcodes[i] = s
	}
	unofficial := " "
	if step.Opcode.Instruction.Unofficial {
		unofficial = "*"
	}

	line := fmt.Sprintf("%04X  %s %s %s %s%-31s A:%02X X:%02X Y:%02X P:%02X SP:%02X CYC:%d\n",
		step.PC, opcodes[0], opcodes[1], opcodes[2], unofficial, step.CustomData,
		state.A, state.X, state.Y, state.Flags, state.SP, state.Cycles)

	if ring := sys.opts.traceRing; ring != nil {
		ring.add(line)
		return
	}
	_, _ = fmt.Fprint(sys.opts.tracingTarget, line)
}

// captureTraceOperands sets the trace text of the current instruction. The
// reader is side-effect free when a trace ring is attached.
func (sys *System) captureTraceOperands(cpu *cpu6502.CPU, ins *cpu6502.Instruction, params ...any) {
	peek := busPeek(cpu)
	if sys.opts.traceRing != nil {
		peek = sys.tracePeek
	}
	setTraceOperands(peek, cpu, ins, params...)
}

// memoryPeek reads one byte without side effects. The ok result is false when
// the address has no side-effect-free read path.
type memoryPeek func(address uint16) (byte, bool)

type cpuState struct {
	A      uint8
	X      uint8
	Y      uint8
	SP     uint8
	Flags  uint8
	Cycles uint64
}

type paramConverterFunc func(peek memoryPeek, cpu *cpu6502.CPU, instruction *cpu6502.Instruction, params ...any) string

var paramConverter = map[cpu6502.AddressingMode]paramConverterFunc{
	cpu6502.ImpliedAddressing:     paramConverterImplied,
	cpu6502.ImmediateAddressing:   paramConverterImmediate,
	cpu6502.AccumulatorAddressing: paramConverterAccumulator,
	cpu6502.AbsoluteAddressing:    paramConverterAbsolute,
	cpu6502.AbsoluteXAddressing:   paramConverterAbsoluteX,
	cpu6502.AbsoluteYAddressing:   paramConverterAbsoluteY,
	cpu6502.ZeroPageAddressing:    paramConverterZeroPage,
	cpu6502.ZeroPageXAddressing:   paramConverterZeroPageX,
	cpu6502.ZeroPageYAddressing:   paramConverterZeroPageY,
	cpu6502.RelativeAddressing:    paramConverterRelative,
	cpu6502.IndirectAddressing:    paramConverterIndirect,
	cpu6502.IndirectXAddressing:   paramConverterIndirectX,
	cpu6502.IndirectYAddressing:   paramConverterIndirectY,
}

// busPeek returns a reader that uses the emulated bus.
func busPeek(cpu *cpu6502.CPU) memoryPeek {
	return func(address uint16) (byte, bool) {
		return cpu.Memory().Read(address), true
	}
}

// setTraceOperands sets the trace text of the current instruction using the
// given read function.
func setTraceOperands(peek memoryPeek, cpu *cpu6502.CPU, ins *cpu6502.Instruction, params ...any) {
	paramsAsString, err := traceCPUParamString(peek, cpu, ins, params...)

	text := strings.ToUpper(ins.Name)
	if err == nil && paramsAsString != "" {
		text += " " + paramsAsString
	}
	cpu.TraceStep.CustomData = text
}

// traceCPUParamString returns the instruction parameters formatted as string.
func traceCPUParamString(peek memoryPeek, cpu *cpu6502.CPU, ins *cpu6502.Instruction, params ...any) (string, error) {
	addressing := cpu.TraceStep.Opcode.Addressing
	fun, ok := paramConverter[addressing]
	if !ok {
		return "", fmt.Errorf("unsupported addressing mode %00x", addressing)
	}

	s := fun(peek, cpu, ins, params...)
	return s, nil
}

func paramConverterImplied(_ memoryPeek, _ *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) string {
	return ""
}

func paramConverterImmediate(_ memoryPeek, _ *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	imm := params[0]
	return fmt.Sprintf("#$%02X", imm)
}

func paramConverterAccumulator(_ memoryPeek, _ *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) string {
	return "A"
}

func paramConverterAbsolute(peek memoryPeek, cpu *cpu6502.CPU, instruction *cpu6502.Instruction, params ...any) string {
	var address cpu6502.Absolute
	if len(params) > 0 {
		address = params[0].(cpu6502.Absolute)
	} else {
		// NoParamFunc instructions (e.g. JSR) read operands themselves; reconstruct from memory.
		b1, ok1 := peek(cpu.TraceStep.PC + 1)
		b2, ok2 := peek(cpu.TraceStep.PC + 2)
		if !ok1 || !ok2 {
			return ""
		}
		address = cpu6502.Absolute(uint16(b2)<<8 | uint16(b1))
	}
	if _, ok := cpu6502.BranchingInstructions[instruction.Name]; ok {
		return fmt.Sprintf("$%04X", address)
	}
	if !traceShouldOutputMemoryContent(uint16(address)) {
		return fmt.Sprintf("$%04X", address)
	}

	b, ok := peek(uint16(address))
	if !ok {
		return fmt.Sprintf("$%04X", address)
	}
	return fmt.Sprintf("$%04X = %02X", address, b)
}

func paramConverterAbsoluteX(peek memoryPeek, cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.Absolute)
	offset := address + cpu6502.Absolute(cpu.X)
	b, ok := peek(uint16(offset))
	if !ok {
		return fmt.Sprintf("$%04X,X @ %04X", address, offset)
	}
	return fmt.Sprintf("$%04X,X @ %04X = %02X", address, offset, b)
}

func paramConverterAbsoluteY(peek memoryPeek, cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.Absolute)
	offset := address + cpu6502.Absolute(cpu.Y)
	b, ok := peek(uint16(offset))
	if !ok {
		return fmt.Sprintf("$%04X,Y @ %04X", address, offset)
	}
	return fmt.Sprintf("$%04X,Y @ %04X = %02X", address, offset, b)
}

func paramConverterZeroPage(peek memoryPeek, _ *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.Absolute)
	b, ok := peek(uint16(address))
	if !ok {
		return fmt.Sprintf("$%02X", address)
	}
	return fmt.Sprintf("$%02X = %02X", address, b)
}

func paramConverterZeroPageX(peek memoryPeek, cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.ZeroPage)
	offset := uint16(byte(address) + cpu.X)
	b, ok := peek(offset)
	if !ok {
		return fmt.Sprintf("$%02X,X @ %02X", address, offset)
	}
	return fmt.Sprintf("$%02X,X @ %02X = %02X", address, offset, b)
}

func paramConverterZeroPageY(peek memoryPeek, cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.ZeroPage)
	offset := uint16(byte(address) + cpu.Y)
	b, ok := peek(offset)
	if !ok {
		return fmt.Sprintf("$%02X,Y @ %02X", address, offset)
	}
	return fmt.Sprintf("$%02X,Y @ %02X = %02X", address, offset, b)
}

func paramConverterRelative(_ memoryPeek, _ *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0]
	return fmt.Sprintf("$%04X", address)
}

func paramConverterIndirect(peek memoryPeek, cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.Indirect)
	value, ok := peekWordBug(peek, uint16(address))
	if !ok {
		return fmt.Sprintf("($%02X%02X)", cpu.TraceStep.OpcodeOperands[2], cpu.TraceStep.OpcodeOperands[1])
	}
	return fmt.Sprintf("($%02X%02X) = %04X", cpu.TraceStep.OpcodeOperands[2], cpu.TraceStep.OpcodeOperands[1], value)
}

func paramConverterIndirectX(peek memoryPeek, cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	var address uint16
	indirectAddress, ok := params[0].(cpu6502.Indirect)
	if ok {
		address = uint16(indirectAddress)
	} else {
		address = uint16(params[0].(cpu6502.IndirectResolved))
	}

	b, ok := peek(address)
	offset := cpu.X + cpu.TraceStep.OpcodeOperands[1]
	if !ok {
		return fmt.Sprintf("($%02X,X) @ %02X = %04X", cpu.TraceStep.OpcodeOperands[1], offset, address)
	}
	return fmt.Sprintf("($%02X,X) @ %02X = %04X = %02X", cpu.TraceStep.OpcodeOperands[1], offset, address, b)
}

func paramConverterIndirectY(peek memoryPeek, cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	var address uint16
	indirectAddress, ok := params[0].(cpu6502.Indirect)
	if ok {
		address = uint16(indirectAddress)
	} else {
		address = uint16(params[0].(cpu6502.IndirectResolved))
	}

	b, ok := peek(address)
	offset := address - uint16(cpu.Y)
	if !ok {
		return fmt.Sprintf("($%02X),Y = %04X @ %04X", cpu.TraceStep.OpcodeOperands[1], offset, address)
	}
	return fmt.Sprintf("($%02X),Y = %04X @ %04X = %02X", cpu.TraceStep.OpcodeOperands[1], offset, address, b)
}

// peekWordBug reads a 16-bit value with the NMOS page-wrap behavior.
func peekWordBug(peek memoryPeek, address uint16) (uint16, bool) {
	low, ok := peek(address)
	if !ok {
		return 0, false
	}
	next := address&0xff00 | uint16(byte(address+1))
	high, ok := peek(next)
	if !ok {
		return 0, false
	}
	return uint16(low) | uint16(high)<<8, true
}

func traceShouldOutputMemoryContent(address uint16) bool {
	switch {
	case address <= nes.RAMEndAddress:
		return true
	case address >= nes.IORegisterStartAddress && address <= nes.IORegisterEndAddress:
		return true
	case address >= nes.CodeBaseAddress:
		return true
	default:
		return false
	}
}
