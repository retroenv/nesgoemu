package memory

import (
	"bytes"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
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

func TestPrintfOtherTestRangeAddress(t *testing.T) {
	var out bytes.Buffer
	m := New(&bus.Bus{})
	m.SetPrintfTarget(&out)

	m.Write(0x401A, 'x')
	m.Write(0x401C, 'x')
	m.FlushPrintf()
	assert.Equal(t, "", out.String())
}
