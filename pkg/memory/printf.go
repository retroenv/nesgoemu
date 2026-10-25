package memory

import "io"

// PrintfAddress is the CPU address that a program uses to print text.
// A write to this address sends one character to the printf target.
// The llvm-mos SDK uses this convention for stdout, and the Mesen script
// printf-mesen.lua reads it. The address is in the unmapped range $4018 to
// $401F, so the write has no hardware effect.
// https://github.com/llvm-mos/llvm-mos-sdk/blob/main/utils/printf-mesen.lua
const PrintfAddress uint16 = 0x401B

// SetPrintfTarget sets the target for text that the program writes to
// PrintfAddress. A nil target ignores the writes.
func (m *Memory) SetPrintfTarget(target io.Writer) {
	m.printf = nil
	if target != nil {
		m.printf = &printfWriter{target: target}
	}
}

// FlushPrintf writes the text of an unfinished line to the printf target.
func (m *Memory) FlushPrintf() {
	if m.printf != nil {
		m.printf.flush()
	}
}

// printfWriter collects characters and writes one complete line at a time.
type printfWriter struct {
	target io.Writer
	line   []byte
}

func (w *printfWriter) writeByte(value byte) {
	w.line = append(w.line, value)
	if value == '\n' {
		w.flush()
	}
}

func (w *printfWriter) flush() {
	if len(w.line) == 0 {
		return
	}

	// A failed write to a diagnostic target must not stop the emulation.
	_, _ = w.target.Write(w.line)
	w.line = w.line[:0]
}
