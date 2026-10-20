package nes

import (
	"strings"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/parameter"
)

// AddressLocation identifies a CPU address and its PRG bank and offset.
type AddressLocation struct {
	Address   uint16 `json:"address"`
	Bank      int    `json:"bank"`
	PRGOffset int    `json:"prg_offset"`
}

// Instruction is one disassembled instruction.
type Instruction struct {
	Location AddressLocation `json:"location"`
	Bytes    []byte          `json:"bytes"`
	Mnemonic string          `json:"mnemonic"`
	Operand  string          `json:"operand"`
	Size     int             `json:"size"`
}

// disasmConverter formats operands in the shared assembler notation.
var disasmConverter = parameter.New(parameter.Config{})

// Disassemble decodes count instructions starting at a CPU address with
// side-effect-free reads. It stops at an address without a readable byte.
func (sys *System) Disassemble(address uint16, count int) []Instruction {
	instructions := make([]Instruction, 0, count)
	pc := address
	for range count {
		instruction, ok := sys.decodeInstruction(pc)
		if !ok {
			break
		}
		instructions = append(instructions, instruction)
		pc += uint16(instruction.Size)
	}
	return instructions
}

// decodeInstruction decodes one instruction at a CPU address.
func (sys *System) decodeInstruction(address uint16) (Instruction, bool) {
	opcodeByte, ok := sys.Peek(address)
	if !ok {
		return Instruction{}, false
	}
	opcode := cpu6502.Opcodes[opcodeByte]
	if opcode.Instruction == nil {
		return Instruction{}, false
	}

	size := addressingSize(opcode.Addressing)
	data := make([]byte, 0, size)
	for offset := range size {
		value, ok := sys.Peek(address + uint16(offset))
		if !ok {
			break
		}
		data = append(data, value)
	}
	if len(data) == 0 {
		return Instruction{}, false
	}

	operand := ""
	if param, ok := instructionOperand(opcode.Addressing, address, data[1:]); ok && param != nil {
		if text, err := parameter.String(disasmConverter, opcode.Addressing, param); err == nil {
			operand = text
		}
	}

	bank, prgOffset, _ := sys.PRGLocation(address)
	return Instruction{
		Location: AddressLocation{
			Address:   address,
			Bank:      bank,
			PRGOffset: prgOffset,
		},
		Bytes:    data,
		Mnemonic: strings.ToUpper(opcode.Instruction.Name),
		Operand:  operand,
		Size:     len(data),
	}, true
}

// instructionOperand builds the operand value for an addressing mode from the
// instruction bytes.
func instructionOperand(mode cpu6502.AddressingMode, address uint16, operands []byte) (any, bool) {
	switch mode {
	case cpu6502.ImpliedAddressing, cpu6502.AccumulatorAddressing:
		return nil, true

	case cpu6502.ImmediateAddressing:
		return immediateOperand(operands)

	case cpu6502.ZeroPageAddressing:
		if len(operands) < 1 {
			return nil, false
		}
		return cpu6502.Absolute(operands[0]), true

	case cpu6502.ZeroPageXAddressing, cpu6502.ZeroPageYAddressing:
		if len(operands) < 1 {
			return nil, false
		}
		return cpu6502.ZeroPage(operands[0]), true

	case cpu6502.RelativeAddressing:
		return relativeOperand(address, operands)

	default:
		return pointerOperand(mode, operands)
	}
}

// pointerOperand builds the operand for the absolute, indirect, and indexed
// indirect addressing modes.
func pointerOperand(mode cpu6502.AddressingMode, operands []byte) (any, bool) {
	switch mode {
	case cpu6502.AbsoluteAddressing, cpu6502.AbsoluteXAddressing, cpu6502.AbsoluteYAddressing:
		value, ok := wordOperand(operands)
		if !ok {
			return nil, false
		}
		return cpu6502.Absolute(value), true

	case cpu6502.IndirectAddressing:
		value, ok := wordOperand(operands)
		if !ok {
			return nil, false
		}
		return cpu6502.Indirect(value), true

	case cpu6502.IndirectXAddressing, cpu6502.IndirectYAddressing:
		if len(operands) < 1 {
			return nil, false
		}
		return cpu6502.Indirect(uint16(operands[0])), true

	default:
		return nil, false
	}
}

// relativeOperand builds the branch target from a relative offset.
func relativeOperand(address uint16, operands []byte) (any, bool) {
	if len(operands) < 1 {
		return nil, false
	}
	offset := int32(int8(operands[0]))
	target := uint16(int32(address) + 2 + offset)
	return cpu6502.Absolute(target), true
}

// immediateOperand returns the immediate operand value.
func immediateOperand(operands []byte) (any, bool) {
	if len(operands) < 1 {
		return nil, false
	}
	return int(operands[0]), true
}

// wordOperand returns a little-endian 16-bit operand.
func wordOperand(operands []byte) (uint16, bool) {
	if len(operands) < 2 {
		return 0, false
	}
	return uint16(operands[0]) | uint16(operands[1])<<8, true
}

// addressingSize returns the instruction size in bytes for an addressing mode.
func addressingSize(mode cpu6502.AddressingMode) int {
	switch mode {
	case cpu6502.ImpliedAddressing, cpu6502.AccumulatorAddressing:
		return 1
	case cpu6502.AbsoluteAddressing, cpu6502.AbsoluteXAddressing, cpu6502.AbsoluteYAddressing,
		cpu6502.IndirectAddressing:
		return 3
	default:
		return 2
	}
}
