package memory

import (
	"bytes"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/assert"
)

func writeText(m *Memory, text string) {
	for _, c := range []byte(text) {
		m.Write(PrintfAddress, c)
	}
}

func TestPrintfLines(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	writeText(m, "hello")
	assert.Equal(t, "", out.String())

	writeText(m, "\nworld\n")
	assert.Equal(t, "hello\nworld\n", out.String())
}

func TestPrintfFlush(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	writeText(m, "partial")
	m.FlushPrintf()
	assert.Equal(t, "partial", out.String())

	m.FlushPrintf()
	assert.Equal(t, "partial", out.String())
}

func TestPrintfLongLine(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	text := string(bytes.Repeat([]byte("a"), 200)) + "\n"
	writeText(m, text)
	assert.Equal(t, text, out.String())
}

func TestPrintfDisabled(t *testing.T) {
	m := New(&bus.Bus{})
	writeText(m, "ignored\n")
	m.FlushPrintf()

	var out bytes.Buffer
	m.SetPrintfTarget(&out)
	m.SetPrintfTarget(nil)
	writeText(m, "ignored\n")
	m.FlushPrintf()
	assert.Equal(t, "", out.String())
}

func writeValue(m *Memory, record string) {
	for _, c := range []byte(record) {
		m.Write(PrintValueAddress, c)
	}
}

func TestPrintfOtherTestRangeAddress(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	m.Write(0x401A, 'x')
	m.Write(0x401D, 'x')
	m.FlushPrintf()
	assert.Equal(t, "", out.String())
}

func TestPrintfTextKeepsPercent(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	writeText(m, "5%done %d %%\n")
	assert.Equal(t, "5%done %d %%\n", out.String())
}

func TestPrintfValues(t *testing.T) {
	tests := []struct {
		name   string
		record string
		want   string
	}{
		{name: "uint8 decimal", record: "ud\xc8", want: "200"},
		{name: "int8 decimal", record: "id\xc8", want: "-56"},
		{name: "int8 plus", record: "+id\x05", want: "+5"},
		{name: "uint8 hex", record: "ux\x1f", want: "1f"},
		{name: "uint8 padded hex", record: "02uX\x0f", want: "0F"},
		{name: "uint8 binary", record: "08ub\x05", want: "00000101"},
		{name: "uint8 character", record: "uc\x41", want: "A"},
		{name: "uint8 quoted character", record: "uq\x41", want: "'A'"},
		{name: "uint16 decimal", record: "Ud\x39\x30", want: "12345"},
		{name: "uint16 hex", record: "04Ux\xc2\x04", want: "04c2"},
		{name: "int16 decimal", record: "Id\xff\xff", want: "-1"},
		{name: "left aligned", record: "-4ud\x07", want: "7   "},
		{name: "bool true", record: "bt\x02", want: "true"},
		{name: "bool false", record: "bv\x00", want: "false"},
		{name: "verb byte zero", record: "u\x00\x01", want: "%!\x00(uint8=1)"},
		{name: "bad verb", record: "us\x01", want: "%!s(uint8=1)"},
		{name: "bad type", record: "z", want: "%!z(BADTYPE)"},
		{name: "long directive", record: "123456789", want: "%!(BADWIDTH)"},
		{name: "percent argument", record: "ud%", want: "37"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			m := New(&bus.Bus{})
			m.SetPrintfTarget(&out)

			writeValue(m, test.record)
			m.FlushPrintf()
			assert.Equal(t, test.want, out.String())
		})
	}
}

func TestPrintfValuesInText(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	writeText(m, "a=")
	writeValue(m, "02uX\x1f")
	writeText(m, " c=")
	writeValue(m, "ud\xc8")
	writeText(m, "\n")

	assert.Equal(t, "a=1F c=200\n", out.String())
}

func TestPrintfRegisters(t *testing.T) {
	cpu := &cpu6502.CPU{
		A:  0x12,
		X:  0x34,
		Y:  0x56,
		SP: 0x78,
	}
	cpu.Flags.C = 1
	cpu.Flags.N = 1

	var out bytes.Buffer
	m := New(&bus.Bus{CPU: cpu})
	m.SetPrintfTarget(&out)

	for _, record := range []string{"02aX", "02xX", "02yX", "08pb", "sd"} {
		writeValue(m, record)
		writeText(m, " ")
	}
	writeText(m, "\n")

	assert.Equal(t, "12 34 56 10000001 120 \n", out.String())
}

func TestPrintfRegistersWithoutCPU(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	writeText(m, "[")
	writeValue(m, "ax")
	writeText(m, "]\n")

	assert.Equal(t, "[%!a(NOCPU)]\n", out.String())
}

func TestPrintfRAMValues(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)
	m.Write(0x0300, 0xa5)
	m.Write(0x0301, 0x12)

	writeValue(m, "02mX\x00\x03")
	writeText(m, " ")
	writeValue(m, "Md\x00\x03")
	writeText(m, "\n")

	assert.Equal(t, "A5 4773\n", out.String())
}

func TestPrintfRAMOutsideInternalRAM(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	writeValue(m, "mx\x00\x20")
	writeText(m, "\n")

	assert.Equal(t, "%!m(BADADDR)\n", out.String())
}

func TestPrintfIncompleteValue(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	writeText(m, "value ")
	writeValue(m, "Ux\x01")
	m.FlushPrintf()
	assert.Equal(t, "value ", out.String())

	writeValue(m, "Ux\x01")
	writeText(m, "\n")
	assert.Equal(t, "value %!(MISSING)\n", out.String())
}
