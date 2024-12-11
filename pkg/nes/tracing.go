package nes

import (
	"fmt"
	"strings"

	. "github.com/retroenv/retrogolib/addressing"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/nes"
)

type cpuState struct {
	A      uint8
	X      uint8
	Y      uint8
	SP     uint8
	Flags  uint8
	Cycles uint64
}

func tracePreExecutionHook(cpu *m6502.CPU, ins *m6502.Instruction, params ...any) {
	paramsAsString, err := traceCPUParamString(cpu, ins, params...)
	if err != nil {
		panic(err)
	}

	cpu.TraceStep.CustomData = strings.ToUpper(ins.Name)
	if paramsAsString != "" {
		cpu.TraceStep.CustomData += " " + paramsAsString
	}
}

// print outputs current trace step in Nintendulator / nestest.log compatible format.
func (sys *System) printTraceStep(state cpuState) {
	step := sys.CPU.TraceStep

	var opcodes [m6502.MaxOpcodeSize]string
	for i := range m6502.MaxOpcodeSize {
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

type paramConverterFunc func(cpu *m6502.CPU, instruction *m6502.Instruction, params ...any) string

var paramConverter = map[Mode]paramConverterFunc{
	ImpliedAddressing:     paramConverterImplied,
	ImmediateAddressing:   paramConverterImmediate,
	AccumulatorAddressing: paramConverterAccumulator,
	AbsoluteAddressing:    paramConverterAbsolute,
	AbsoluteXAddressing:   paramConverterAbsoluteX,
	AbsoluteYAddressing:   paramConverterAbsoluteY,
	ZeroPageAddressing:    paramConverterZeroPage,
	ZeroPageXAddressing:   paramConverterZeroPageX,
	ZeroPageYAddressing:   paramConverterZeroPageY,
	RelativeAddressing:    paramConverterRelative,
	IndirectAddressing:    paramConverterIndirect,
	IndirectXAddressing:   paramConverterIndirectX,
	IndirectYAddressing:   paramConverterIndirectY,
}

// traceCPUParamString returns the instruction parameters formatted as string.
func traceCPUParamString(cpu *m6502.CPU, ins *m6502.Instruction, params ...any) (string, error) {
	addressing := cpu.TraceStep.Opcode.Addressing
	fun, ok := paramConverter[addressing]
	if !ok {
		return "", fmt.Errorf("unsupported addressing mode %00x", addressing)
	}

	s := fun(cpu, ins, params...)
	return s, nil
}

func paramConverterImplied(_ *m6502.CPU, _ *m6502.Instruction, _ ...any) string {
	return ""
}

func paramConverterImmediate(_ *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	imm := params[0]
	return fmt.Sprintf("#$%02X", imm)
}

func paramConverterAccumulator(_ *m6502.CPU, _ *m6502.Instruction, _ ...any) string {
	return "A"
}

func paramConverterAbsolute(cpu *m6502.CPU, instruction *m6502.Instruction, params ...any) string {
	address := params[0].(Absolute)
	if _, ok := m6502.BranchingInstructions[instruction.Name]; ok {
		return fmt.Sprintf("$%04X", address)
	}
	if !traceShouldOutputMemoryContent(uint16(address)) {
		return fmt.Sprintf("$%04X", address)
	}

	b := cpu.Memory().Read(uint16(address))
	return fmt.Sprintf("$%04X = %02X", address, b)
}

func paramConverterAbsoluteX(cpu *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	address := params[0].(Absolute)
	offset := address + Absolute(cpu.X)
	b := cpu.Memory().Read(uint16(offset))
	return fmt.Sprintf("$%04X,X @ %04X = %02X", address, offset, b)
}

func paramConverterAbsoluteY(cpu *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	address := params[0].(Absolute)
	offset := address + Absolute(cpu.Y)
	b := cpu.Memory().Read(uint16(offset))
	return fmt.Sprintf("$%04X,Y @ %04X = %02X", address, offset, b)
}

func paramConverterZeroPage(cpu *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	address := params[0].(Absolute)
	b := cpu.Memory().Read(uint16(address))
	return fmt.Sprintf("$%02X = %02X", address, b)
}

func paramConverterZeroPageX(cpu *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	address := params[0].(ZeroPage)
	offset := uint16(byte(address) + cpu.X)
	b := cpu.Memory().Read(offset)
	return fmt.Sprintf("$%02X,X @ %02X = %02X", address, offset, b)
}

func paramConverterZeroPageY(cpu *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	address := params[0].(ZeroPage)
	offset := uint16(byte(address) + cpu.Y)
	b := cpu.Memory().Read(offset)
	return fmt.Sprintf("$%02X,Y @ %02X = %02X", address, offset, b)
}

func paramConverterRelative(_ *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	address := params[0]
	return fmt.Sprintf("$%04X", address)
}

func paramConverterIndirect(cpu *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	address := params[0].(Indirect)
	value := cpu.Memory().ReadWordBug(uint16(address))
	return fmt.Sprintf("($%02X%02X) = %04X", cpu.TraceStep.OpcodeOperands[2], cpu.TraceStep.OpcodeOperands[1], value)
}

func paramConverterIndirectX(cpu *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	var address uint16
	indirectAddress, ok := params[0].(Indirect)
	if ok {
		address = uint16(indirectAddress)
	} else {
		address = uint16(params[0].(IndirectResolved))
	}

	b := cpu.Memory().Read(address)
	offset := cpu.X + cpu.TraceStep.OpcodeOperands[1]
	return fmt.Sprintf("($%02X,X) @ %02X = %04X = %02X", cpu.TraceStep.OpcodeOperands[1], offset, address, b)
}

func paramConverterIndirectY(cpu *m6502.CPU, _ *m6502.Instruction, params ...any) string {
	var address uint16
	indirectAddress, ok := params[0].(Indirect)
	if ok {
		address = uint16(indirectAddress)
	} else {
		address = uint16(params[0].(IndirectResolved))
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
