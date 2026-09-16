package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestWindowFetchPositions(t *testing.T) {
	tests := []struct {
		cycle, line, column, fetchLine int
		inside                         bool
	}{
		{321, 261, 0, 0, true}, {329, 261, 1, 0, true}, {1, 0, 2, 0, true},
		{233, 0, 31, 0, true}, {241, 0, 32, 0, false}, {249, 0, 33, 0, false},
		{321, 7, 0, 8, true}, {329, 7, 1, 8, true}, {257, 7, -1, 7, false},
		{0, 7, -1, 7, false}, {1, 240, 2, 240, false}, {1, 261, 2, 261, false},
	}
	m := newTestMapper(t, 0x8000, 0x2000)
	m.windowSplitRegs = [6]byte{0, 31, 0, 239, 0, 0}
	for _, test := range tests {
		m.ppuCycle, m.ppuScanLine = test.cycle, test.line
		column, line, inside := m.windowPosition()
		assert.Equal(t, test.column, column)
		assert.Equal(t, test.fetchLine, line)
		assert.Equal(t, test.inside, inside)
	}
}

func TestWindowScrollAndPalette(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x8000)
	m.Write(regCHRControl, 0x10)
	m.windowSplitRegs = [6]byte{0, 31, 0, 239, 0, 0}
	m.ppuCycle, m.ppuScanLine = 1, 16
	m.fpgaRAM[66] = 3
	m.fpgaRAM[0x3C0] = 0x80
	assert.Equal(t, byte(3), m.NameTableMemory().Read(0x2000))
	assert.Equal(t, byte(0xAA), m.NameTableMemory().Read(0x23C0))
	m.Write(regWindowSplitStart+4, 0xFF)
	m.Write(regWindowSplitEnd, 5)
	m.fpgaRAM[65] = 4
	assert.Equal(t, byte(4), m.NameTableMemory().Read(0x2000))
	m.chrROM[0x45] = 0xAB
	assert.Equal(t, byte(0xAB), m.Read(0x40))
	m.Write(regNTWindowControl, 0x0E)
	m.fpgaRAM[3*1024+65] = 2
	m.chrROM[0x2045] = 0xCD
	m.NameTableMemory().Read(0x2000)
	assert.Equal(t, byte(0xCD), m.Read(0x40))
	m.Write(regNTWindowControl, 0x23)
	m.Write(regFillTile, 7)
	m.Write(regFillAttribute, 1)
	assert.Equal(t, byte(7), m.NameTableMemory().Read(0x2000))
	assert.Equal(t, byte(0x55), m.NameTableMemory().Read(0x23C0))
	m.Write(regCHRControl, 0)
	assert.Equal(t, byte(0), m.NameTableMemory().Read(0x2000))
}

func TestWindowBounds(t *testing.T) {
	for _, test := range []struct {
		value, start, end int
		inside            bool
	}{
		{0, 0, 31, true}, {31, 0, 31, true}, {32, 0, 31, false},
		{28, 28, 3, true}, {29, 28, 3, true}, {3, 28, 3, true}, {4, 28, 3, false},
		{0, 7, 7, false}, {7, 7, 7, true}, {31, 7, 7, false},
		{230, 230, 10, true}, {231, 230, 10, true}, {10, 230, 10, true}, {11, 230, 10, false},
	} {
		assert.Equal(t, test.inside, windowInRange(test.value, test.start, test.end))
	}
	m := newTestMapper(t, 0x8000, 0x2000)
	for _, address := range []uint16{regWindowSplitStart, regWindowSplitStart + 1, regWindowSplitStart + 4} {
		m.Write(address, 0xFF)
		assert.Equal(t, byte(31), m.windowSplitRegs[address-regWindowSplitStart])
	}
}

// Equal bounds select one tile column and one scanline. Wrapped bounds include
// both endpoints. Check the selected memory, as well as the range comparison.
func TestWindowBoundaryRouting(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regCHRControl, 0x10)
	m.Write(regNTWindowControl, 0x20)
	m.Write(regFillTile, 0xA5)
	m.NameTableMemory().WriteCIRAM(0, 0x5A)

	for _, test := range []struct {
		startX, endX, startY, endY byte
		column, line               int
		inside                     bool
	}{
		{7, 7, 10, 10, 7, 10, true},
		{7, 7, 10, 10, 6, 10, false},
		{7, 7, 10, 10, 7, 9, false},
		{28, 3, 230, 10, 28, 230, true},
		{28, 3, 230, 10, 3, 10, true},
		{28, 3, 230, 10, 27, 230, false},
		{28, 3, 230, 10, 28, 229, false},
	} {
		m.Write(regWindowSplitStart, test.startX)
		m.Write(regWindowSplitStart+1, test.endX)
		m.Write(regWindowSplitStart+2, test.startY)
		m.Write(regWindowSplitStart+3, test.endY)
		m.TickPPU((test.column-2)*8+1, test.line, true)
		expected := byte(0x5A)
		if test.inside {
			expected = 0xA5
		}
		assert.Equal(t, expected, m.NameTableMemory().Read(0x2000))
	}
}

func TestWindowPrefetchScrollWrap(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.windowEnabled = true
	m.windowSplitRegs = [6]byte{0, 31, 0, 239, 31, 239}
	m.ppuCycle, m.ppuScanLine = 321, 0
	m.fpgaRAM[31] = 0xAB
	m.fpgaRAM[0] = 0xCD
	assert.Equal(t, byte(0xAB), m.NameTableMemory().Read(0x2000))
	m.ppuCycle = 329
	assert.Equal(t, byte(0xCD), m.NameTableMemory().Read(0x2000))
}

func TestBusWindowFetchPositions(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.EnableBusTiming()
	m.scanIRQ.inFrame = true
	m.scanIRQ.counter = 7
	m.windowSplitRegs = [6]byte{0, 31, 0, 239, 0, 0}

	for _, test := range []struct {
		fetch        int
		column, line int
		inside       bool
	}{
		{0, 1, 7, true},
		{30, 31, 7, true},
		{31, 32, 7, false},
		{49, 0, 8, true},
		{50, 1, 8, true},
	} {
		m.ppuBus.tiles = test.fetch
		column, line, inside := m.windowPosition()
		assert.Equal(t, test.column, column)
		assert.Equal(t, test.line, line)
		assert.Equal(t, test.inside, inside)
	}

	m.scanIRQ.inFrame = false
	column, line, inside := m.windowPosition()
	assert.Equal(t, 0, column)
	assert.Equal(t, 0, line)
	assert.False(t, inside)
}
