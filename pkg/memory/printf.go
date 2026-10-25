package memory

import (
	"fmt"
	"io"
	"strings"
)

// PrintfAddress is the CPU address that a program uses to print text.
// A write to this address sends one character to the printf target.
// The llvm-mos SDK uses this convention for stdout, and the Mesen script
// printf-mesen.lua reads it. The address is in the unmapped range $4018 to
// $401F, so the write has no hardware effect.
// https://github.com/llvm-mos/llvm-mos-sdk/blob/main/utils/printf-mesen.lua
const PrintfAddress uint16 = 0x401B

// PrintValueAddress is the CPU address that a program uses to print a value.
// The program writes a value record to this address. The emulator formats the
// value with the Go fmt package and adds the text to the current line.
//
// A record has these parts, in this sequence:
//
//  1. Optional flags, width, and precision of a Go verb, for example "04".
//     The permitted characters are "+-# 0123456789.".
//  2. One type letter. It sets the source and the Go type of the value.
//  3. The Go verb, for example 'd' or 'x'.
//  4. The argument bytes of the type, low byte first.
//
// The type letters are:
//
//	u  one byte, uint8
//	i  one byte, int8
//	U  two bytes, uint16
//	I  two bytes, int16
//	b  one byte, bool (zero is false)
//	m  two address bytes, internal RAM byte as uint8
//	M  two address bytes, internal RAM word as uint16
//	a  no bytes, accumulator as uint8
//	x  no bytes, X register as uint8
//	y  no bytes, Y register as uint8
//	p  no bytes, processor status flags as uint8
//	s  no bytes, stack pointer as uint8
//
// For example, the bytes "04", 'U', 'x', $C2, $04 print "04c2". A RAM read has
// no bus side effect. A record that cannot give a value prints a Go style
// error text, for example "%!m(BADADDR)". A text write before the end of a
// record prints "%!(MISSING)" and stops the record.
//
// The address is in the unmapped range $4018 to $401F, so the write has no
// hardware effect.
const PrintValueAddress uint16 = 0x401C

// printDirectiveCharacters are the characters of the flags, width, and
// precision of a value record.
const printDirectiveCharacters = "+-# 0123456789."

// Problem texts of a value record that cannot give a value.
const (
	printProblemBadAddress = "BADADDR"
	printProblemNoCPU      = "NOCPU"
)

// printDirectiveMaxLength limits the flags, width, and precision of a value
// record. A longer directive stops the record.
const printDirectiveMaxLength = 8

// SetPrintfTarget sets the target for text that the program writes to
// PrintfAddress and PrintValueAddress. A nil target ignores the writes.
func (m *Memory) SetPrintfTarget(target io.Writer) {
	m.printf = nil
	if target != nil {
		m.printf = &printfWriter{
			target: target,
			memory: m,
		}
	}
}

// FlushPrintf writes the text of an unfinished line to the printf target.
// It discards an incomplete value record.
func (m *Memory) FlushPrintf() {
	if m.printf != nil {
		m.printf.reset()
		m.printf.flush()
	}
}

// statusFlagGetter reports the processor status flags as one byte.
type statusFlagGetter interface {
	GetFlags() uint8
}

// printfWriter collects characters and writes one complete line at a time.
// It formats the value records that the program writes to PrintValueAddress.
type printfWriter struct {
	target io.Writer
	memory *Memory

	line []byte

	directive []byte
	valueType byte
	verb      byte
	hasVerb   bool
	arguments []byte
	missing   int
}

// write handles a write to an address in the unmapped range $4018 to $401F.
func (w *printfWriter) write(address uint16, value byte) {
	switch address {
	case PrintfAddress:
		w.writeText(value)

	case PrintValueAddress:
		w.writeValue(value)
	}
}

// writeText adds one character of program text.
func (w *printfWriter) writeText(value byte) {
	if w.pending() {
		w.appendFormatted("%!(MISSING)")
		w.reset()
	}

	w.line = append(w.line, value)
	if value == '\n' {
		w.flush()
	}
}

// writeValue adds one byte of a value record.
func (w *printfWriter) writeValue(value byte) {
	switch {
	case w.hasVerb:
		w.arguments = append(w.arguments, value)
		w.missing--
		if w.missing == 0 {
			w.emit()
		}

	case w.valueType != 0:
		w.verb = value
		w.hasVerb = true
		w.missing = valueArgumentCount(w.valueType)
		if w.missing == 0 {
			w.emit()
		}

	case strings.IndexByte(printDirectiveCharacters, value) >= 0:
		if len(w.directive) == printDirectiveMaxLength {
			w.appendFormatted("%!(BADWIDTH)")
			w.reset()
			return
		}
		w.directive = append(w.directive, value)

	case valueArgumentCount(value) >= 0:
		w.valueType = value

	default:
		w.appendFormatted(fmt.Sprintf("%%!%c(BADTYPE)", value))
		w.reset()
	}
}

func (w *printfWriter) pending() bool {
	return len(w.directive) > 0 || w.valueType != 0
}

func (w *printfWriter) reset() {
	w.directive = w.directive[:0]
	w.valueType = 0
	w.verb = 0
	w.hasVerb = false
	w.arguments = w.arguments[:0]
	w.missing = 0
}

// emit formats the value of the complete record and starts a new record.
func (w *printfWriter) emit() {
	defer w.reset()

	value, problem := w.value()
	if problem != "" {
		w.appendFormatted(fmt.Sprintf("%%!%c(%s)", w.valueType, problem))
		return
	}

	format := "%" + string(w.directive) + string(rune(w.verb))
	w.appendFormatted(fmt.Sprintf(format, value))
}

// value returns the value of the complete record, or a problem text when the
// record cannot give a value.
func (w *printfWriter) value() (any, string) {
	switch w.valueType {
	case 'u':
		return w.arguments[0], ""

	case 'i':
		return int8(w.arguments[0]), ""

	case 'U':
		return w.word(), ""

	case 'I':
		return int16(w.word()), ""

	case 'b':
		return w.arguments[0] != 0, ""

	case 'm':
		value, ok := w.memory.InspectRAM(w.word())
		if !ok {
			return nil, printProblemBadAddress
		}
		return value, ""

	case 'M':
		return w.ramWord()

	default:
		return w.register()
	}
}

func (w *printfWriter) word() uint16 {
	return uint16(w.arguments[0]) | uint16(w.arguments[1])<<8
}

func (w *printfWriter) ramWord() (any, string) {
	address := w.word()

	low, ok := w.memory.InspectRAM(address)
	if !ok {
		return nil, printProblemBadAddress
	}

	high, ok := w.memory.InspectRAM(address + 1)
	if !ok {
		return nil, printProblemBadAddress
	}

	return uint16(low) | uint16(high)<<8, ""
}

func (w *printfWriter) register() (any, string) {
	cpu := w.memory.bus.CPU
	if cpu == nil {
		return nil, printProblemNoCPU
	}

	state := cpu.State()

	switch w.valueType {
	case 'a':
		return state.A, ""

	case 'x':
		return state.X, ""

	case 'y':
		return state.Y, ""

	case 's':
		return state.SP, ""

	default:
		flags, ok := cpu.(statusFlagGetter)
		if !ok {
			return nil, printProblemNoCPU
		}

		return flags.GetFlags(), ""
	}
}

func (w *printfWriter) appendFormatted(text string) {
	w.line = append(w.line, text...)
}

func (w *printfWriter) flush() {
	if len(w.line) == 0 {
		return
	}

	// A failed write to a diagnostic target must not stop the emulation.
	_, _ = w.target.Write(w.line)
	w.line = w.line[:0]
}

// valueArgumentCount returns the number of argument bytes of a value record
// type letter. It returns -1 for an unknown type letter.
func valueArgumentCount(valueType byte) int {
	switch valueType {
	case 'u', 'i', 'b':
		return 1

	case 'U', 'I', 'm', 'M':
		return 2

	case 'a', 'x', 'y', 'p', 's':
		return 0
	}

	return -1
}
