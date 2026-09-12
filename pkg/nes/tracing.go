package nes

import (
	"fmt"
	"strings"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
)

// print outputs current trace step in Nintendulator / nestest.log compatible format.
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

	s := fmt.Sprintf("%04X  %s %s %s %s%-31s A:%02X X:%02X Y:%02X P:%02X SP:%02X CYC:%d\n",
		step.PC, opcodes[0], opcodes[1], opcodes[2], unofficial, step.CustomData,
		state.A, state.X, state.Y, state.Flags, state.SP, state.Cycles)
	_, _ = fmt.Fprint(sys.opts.tracingTarget, s)
}

type cpuState struct {
	A      uint8
	X      uint8
	Y      uint8
	SP     uint8
	Flags  uint8
	Cycles uint64
}

type paramConverterFunc func(cpu *cpu6502.CPU, instruction *cpu6502.Instruction, params ...any) string

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

func tracePreExecutionHook(cpu *cpu6502.CPU, ins *cpu6502.Instruction, params ...any) {
	paramsAsString, err := traceCPUParamString(cpu, ins, params...)
	if err != nil {
		panic(err)
	}

	cpu.TraceStep.CustomData = strings.ToUpper(ins.Name)
	if paramsAsString != "" {
		cpu.TraceStep.CustomData += " " + paramsAsString
	}
}

// traceCPUParamString returns the instruction parameters formatted as string.
func traceCPUParamString(cpu *cpu6502.CPU, ins *cpu6502.Instruction, params ...any) (string, error) {
	addressing := cpu.TraceStep.Opcode.Addressing
	fun, ok := paramConverter[addressing]
	if !ok {
		return "", fmt.Errorf("unsupported addressing mode %00x", addressing)
	}

	s := fun(cpu, ins, params...)
	return s, nil
}

func paramConverterImplied(_ *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) string {
	return ""
}

func paramConverterImmediate(_ *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	imm := params[0]
	return fmt.Sprintf("#$%02X", imm)
}

func paramConverterAccumulator(_ *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) string {
	return "A"
}

func paramConverterAbsolute(cpu *cpu6502.CPU, instruction *cpu6502.Instruction, params ...any) string {
	var address cpu6502.Absolute
	if len(params) > 0 {
		address = params[0].(cpu6502.Absolute)
	} else {
		// NoParamFunc instructions (e.g. JSR) read operands themselves; reconstruct from memory.
		b1 := uint16(cpu.Memory().Read(cpu.TraceStep.PC + 1))
		b2 := uint16(cpu.Memory().Read(cpu.TraceStep.PC + 2))
		address = cpu6502.Absolute(b2<<8 | b1)
	}
	if _, ok := cpu6502.BranchingInstructions[instruction.Name]; ok {
		return fmt.Sprintf("$%04X", address)
	}
	if !traceShouldOutputMemoryContent(uint16(address)) {
		return fmt.Sprintf("$%04X", address)
	}

	b := cpu.Memory().Read(uint16(address))
	return fmt.Sprintf("$%04X = %02X", address, b)
}

func paramConverterAbsoluteX(cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.Absolute)
	offset := address + cpu6502.Absolute(cpu.X)
	b := cpu.Memory().Read(uint16(offset))
	return fmt.Sprintf("$%04X,X @ %04X = %02X", address, offset, b)
}

func paramConverterAbsoluteY(cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.Absolute)
	offset := address + cpu6502.Absolute(cpu.Y)
	b := cpu.Memory().Read(uint16(offset))
	return fmt.Sprintf("$%04X,Y @ %04X = %02X", address, offset, b)
}

func paramConverterZeroPage(cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.Absolute)
	b := cpu.Memory().Read(uint16(address))
	return fmt.Sprintf("$%02X = %02X", address, b)
}

func paramConverterZeroPageX(cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.ZeroPage)
	offset := uint16(byte(address) + cpu.X)
	b := cpu.Memory().Read(offset)
	return fmt.Sprintf("$%02X,X @ %02X = %02X", address, offset, b)
}

func paramConverterZeroPageY(cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.ZeroPage)
	offset := uint16(byte(address) + cpu.Y)
	b := cpu.Memory().Read(offset)
	return fmt.Sprintf("$%02X,Y @ %02X = %02X", address, offset, b)
}

func paramConverterRelative(_ *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0]
	return fmt.Sprintf("$%04X", address)
}

func paramConverterIndirect(cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	address := params[0].(cpu6502.Indirect)
	value := cpu.Memory().ReadWordBug(uint16(address))
	return fmt.Sprintf("($%02X%02X) = %04X", cpu.TraceStep.OpcodeOperands[2], cpu.TraceStep.OpcodeOperands[1], value)
}

func paramConverterIndirectX(cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	var address uint16
	indirectAddress, ok := params[0].(cpu6502.Indirect)
	if ok {
		address = uint16(indirectAddress)
	} else {
		address = uint16(params[0].(cpu6502.IndirectResolved))
	}

	b := cpu.Memory().Read(address)
	offset := cpu.X + cpu.TraceStep.OpcodeOperands[1]
	return fmt.Sprintf("($%02X,X) @ %02X = %04X = %02X", cpu.TraceStep.OpcodeOperands[1], offset, address, b)
}

func paramConverterIndirectY(cpu *cpu6502.CPU, _ *cpu6502.Instruction, params ...any) string {
	var address uint16
	indirectAddress, ok := params[0].(cpu6502.Indirect)
	if ok {
		address = uint16(indirectAddress)
	} else {
		address = uint16(params[0].(cpu6502.IndirectResolved))
	}

	b := cpu.Memory().Read(address)
	offset := address - uint16(cpu.Y)
	return fmt.Sprintf("($%02X),Y = %04X @ %04X = %02X", cpu.TraceStep.OpcodeOperands[1], offset, address, b)
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
