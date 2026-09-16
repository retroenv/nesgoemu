package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestBusScanlineDetection(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.EnableBusTiming()
	m.Write(regScanIRQLatch, 0)
	m.Write(regScanIRQOffset, 1)
	nt := m.NameTableMemory()
	nt.Read(0x2000)
	nt.Read(0x2000)
	assert.False(t, m.scanIRQ.inFrame)
	nt.Read(0x2000)
	assert.True(t, m.scanIRQ.inFrame)
	assert.Equal(t, int16(0), m.scanIRQ.counter)
	assert.False(t, m.scanIRQ.pending)
	nt.Read(0x23C0)
	assert.True(t, m.scanIRQ.pending)
	assert.False(t, m.cycleIRQ.lineActive)
	m.Write(regScanIRQControl, 0)
	assert.True(t, m.cycleIRQ.lineActive)
	m.Read(regScanIRQControl)
	assert.False(t, m.cycleIRQ.lineActive)
	m.clockPPUBus()
	m.clockPPUBus()
	assert.True(t, m.scanIRQ.inFrame)
	m.clockPPUBus()
	assert.False(t, m.scanIRQ.inFrame)
	assert.Equal(t, int16(-1), m.scanIRQ.counter)
}

func TestBusReadsBreakScanlineSequence(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.EnableBusTiming()
	nt := m.NameTableMemory()
	nt.Read(0x2000)
	nt.Read(0x2000)
	m.Read(0)
	nt.Read(0x2000)
	assert.False(t, m.scanIRQ.inFrame)
	nt.Read(0x2000)
	nt.Read(0x2000)
	assert.True(t, m.scanIRQ.inFrame)
	for i := range 32 {
		nt.Read(0x23C0)
		nt.Read(0x2000 + uint16(i))
	}
	assert.True(t, m.scanIRQ.inHBlank)
}

func TestFillDisabledOutsideFrame(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.EnableBusTiming()
	m.Write(regNTControlStart, 0x23)
	m.Write(regFillTile, 0xFF)
	m.NameTableMemory().WriteCIRAM(0, 0x12)
	assert.Equal(t, byte(0x12), m.NameTableMemory().Read(0x2000))
}

// An NMI vector read ends the old frame. Two earlier nametable reads must not
// combine with a later read to start a new frame.
func TestNMIVectorBreaksScanlineSequence(t *testing.T) {
	for _, vector := range []uint16{nmiVectorLower, nmiVectorUpper} {
		for _, redirect := range []byte{0, 1} {
			m := newTestMapper(t, 0x8000, 0x2000)
			m.EnableBusTiming()
			m.Write(regVectorControl, redirect)
			nt := m.NameTableMemory()
			nt.Read(0x2000)
			nt.Read(0x2000)
			m.Read(vector)
			nt.Read(0x2000)
			assert.False(t, m.scanIRQ.inFrame)
			nt.Read(0x2000)
			assert.False(t, m.scanIRQ.inFrame)
			nt.Read(0x2000)
			assert.True(t, m.scanIRQ.inFrame)
			assert.Equal(t, byte(0), m.Read(regScanIRQLatch))
		}
	}
}
