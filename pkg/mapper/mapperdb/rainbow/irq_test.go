package rainbow

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/assert"
)

func TestIRQOffsetUnits(t *testing.T) {
	for _, offset := range []byte{1, 50, 129, 135, 170} {
		m := newTestMapper(t, 0x8000, 0x2000)
		m.Write(regScanIRQLatch, 12)
		m.Write(regScanIRQControl, 0)
		m.Write(regScanIRQOffset, offset)
		for dot := range 341 {
			m.TickPPU(dot, 12, true)
			assert.Equal(t, dot >= int(offset)*2-1, m.scanIRQ.pending)
			assert.Equal(t, dot >= 256, m.scanIRQ.inHBlank)
		}
		assert.Equal(t, byte(12), m.Read(regScanIRQLatch))
		assert.Equal(t, byte(0xC0), m.Read(regScanIRQControl))
		assert.False(t, m.scanIRQ.pending)
	}
}

func TestCycleIRQReloadAndAcknowledge(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regCycleIRQReloadLower, 5)
	m.Write(regCycleIRQControl, 1)
	m.ClockCPU(7)
	assert.True(t, m.cycleIRQ.enabled)
	assert.True(t, m.cycleIRQ.pending)
	assert.Equal(t, uint16(3), m.cycleIRQ.counter)
	m.Write(regCycleIRQAcknowledge, 0)
	assert.False(t, m.cycleIRQ.enabled)
	assert.Equal(t, uint16(3), m.cycleIRQ.counter)
	m.Write(regCycleIRQControl, 2)
	m.Write(regCycleIRQAcknowledge, 0)
	assert.True(t, m.cycleIRQ.enabled)
	assert.Equal(t, uint16(3), m.cycleIRQ.counter)
	m.Write(regCycleIRQReloadLower, 8)
	assert.Equal(t, uint16(3), m.cycleIRQ.counter)
	m.ClockCPU(3)
	assert.Equal(t, uint16(8), m.cycleIRQ.counter)
	m.Write(regCycleIRQControl, 0)
	assert.False(t, m.cycleIRQ.pending)
}

func TestCPUParityAndZeroCounter(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.ClockCPU(3)
	assert.Equal(t, byte(0x80), m.Read(regCycleIRQParity))
	m.Write(regCycleIRQParity, 0xFF)
	assert.Equal(t, byte(0x80), m.Read(regCycleIRQParity))
	m.Write(regCycleIRQControl, 1)
	m.ClockCPU(1)
	assert.Equal(t, uint16(0), m.cycleIRQ.counter)
	assert.False(t, m.cycleIRQ.pending)
	m.ClockCPU(testUint16Maximum)
	assert.False(t, m.cycleIRQ.pending)
	assert.Equal(t, byte(0x80), m.Read(regCycleIRQParity))
}

func TestTickPPURenderingStopClearsFallbackState(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.scanIRQ.inFrame = true
	m.scanIRQ.inHBlank = true
	m.scanIRQ.readyToFire = true
	m.bgExtActive = true

	m.TickPPU(123, 45, false)

	assert.Equal(t, ppuLastVisibleLine+1, m.ppuScanLine)
	assert.False(t, m.scanIRQ.inFrame)
	assert.False(t, m.scanIRQ.inHBlank)
	assert.False(t, m.scanIRQ.readyToFire)
	assert.False(t, m.bgExtActive)
}

func TestScanlineIRQRequiresRendering(t *testing.T) {
	m, system := newTestMapperWithBus(t, 0x8000, 0x2000)
	newOAMCPU(t, m, system)
	m.Write(regScanIRQLatch, 5)
	m.Write(regScanIRQControl, 0)
	system.PPU.Step(341 * 20)
	assert.False(t, m.scanIRQ.pending)
	assert.False(t, m.scanIRQ.inFrame)
	system.PPU.Write(0x2001, 0x08)
	system.PPU.Step(341 * 262)
	assert.True(t, m.scanIRQ.pending)
	_ = m.Read(regScanIRQControl)
	system.PPU.Write(0x2001, 0)
	system.PPU.Step(341 * 262)
	m.ClockCPU(3)
	assert.False(t, m.scanIRQ.pending)
	assert.False(t, m.scanIRQ.inFrame)
}

func TestPCMReadAcknowledgesCycleIRQ(t *testing.T) {
	m, system := newTestMapperWithBus(t, 0x8000, 0x2000)
	newOAMCPU(t, m, system)
	m.Write(regCycleIRQReloadLower, 4)
	m.Write(regCycleIRQControl, 7)
	m.ClockCPU(5)
	assert.True(t, m.cycleIRQ.pending)
	assert.Equal(t, byte(0), system.Memory.Read(0x4011))
	assert.False(t, m.cycleIRQ.pending)
	assert.Equal(t, uint16(3), m.cycleIRQ.counter)
}

func TestNMIVectorReadWithoutRedirection(t *testing.T) {
	for _, address := range []uint16{nmiVectorLower, nmiVectorUpper} {
		m := newTestMapper(t, 0x8000, 0x2000)
		m.scanIRQ.pending = true
		m.scanIRQ.inFrame = true
		m.scanIRQ.counter = 12
		m.prgROM[address-0x8000] = 0xAB
		assert.Equal(t, byte(0xAB), m.Read(address))
		assert.False(t, m.scanIRQ.pending)
		assert.False(t, m.scanIRQ.inFrame)
		assert.Equal(t, int16(0), m.scanIRQ.counter)
	}
}

func TestIRQLineAcknowledgement(t *testing.T) {
	m, system := newTestMapperWithBus(t, 0x8000, 0x2000)
	input := &irqInput{}
	system.CPU = input
	m.Write(regCycleIRQReloadLower, 2)
	m.Write(regCycleIRQControl, 1)
	m.ClockCPU(7)
	assert.True(t, input.active)
	assert.Equal(t, byte(5), m.scanIRQ.jitter)
	m.scanIRQ.pending, m.scanIRQ.enabled = true, true
	m.Write(regCycleIRQAcknowledge, 0)
	assert.True(t, input.active)
	m.Read(regScanIRQControl)
	assert.False(t, input.active)
	m.scanIRQ.latch = 5
	m.scanIRQ.enabled = false
	m.TickPPU(0, 5, true)
	m.TickPPU(269, 5, true)
	assert.True(t, m.scanIRQ.pending)
	assert.False(t, input.active)
	m.Write(regScanIRQControl, 0)
	assert.True(t, input.active)
}

func TestCombinedIRQStatusDoesNotAcknowledge(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regScanIRQLatch, 1)
	m.Write(regScanIRQControl, 0)
	m.TickPPU(0, 1, true)
	m.TickPPU(269, 1, true)
	m.Write(regCycleIRQReloadLower, 1)
	m.Write(regCycleIRQControl, 1)
	m.ClockCPU(1)

	for range 3 {
		assert.Equal(t, byte(0xC0), m.Read(regIRQStatus))
		assert.True(t, m.cycleIRQ.lineActive)
	}
	m.Read(regScanIRQControl)
	assert.Equal(t, byte(0x40), m.Read(regIRQStatus))
	assert.True(t, m.cycleIRQ.lineActive)
	m.Write(regCycleIRQAcknowledge, 0)
	assert.Equal(t, byte(0), m.Read(regIRQStatus))
	assert.False(t, m.cycleIRQ.lineActive)
}

type irqInput struct {
	bus.CPU
	active bool
}

func (input *irqInput) SetIRQ(active bool) { input.active = active }
