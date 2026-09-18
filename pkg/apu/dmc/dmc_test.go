package dmc

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestWriteRegisters(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})

	d.Write(0x4010, 0xcf) // interrupt enabled, loop, rate index 15
	assert.True(t, d.irqEnabled)
	assert.True(t, d.loop)
	assert.Equal(t, uint16(27), d.timer)

	d.Write(0x4011, 0xff)
	assert.Equal(t, byte(0x7f), d.output, "the direct load uses seven bits")

	d.Write(0x4012, 0x02)
	d.Write(0x4013, 0x01)
	assert.Equal(t, byte(2), d.sampleAddr)
	assert.Equal(t, byte(1), d.sampleLen)
}

func TestWriteClearsIRQWhenDisabled(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.irq = true

	d.Write(0x4010, 0x00)

	assert.False(t, d.IRQ())
}

func TestEnableRestartsSampleAndFetches(t *testing.T) {
	reader := &memoryReader{data: map[uint16]byte{0xc000: 0xa5}}
	staller := &cycleStaller{}
	d := New(reader, staller)
	d.Write(0x4012, 0x00)
	d.Write(0x4013, 0x01) // 17 bytes

	d.SetEnabled(true)

	assert.True(t, d.Active())
	assert.Equal(t, 1, reader.reads)
	assert.Equal(t, uint16(dmaStallCycles), staller.total)
	assert.Equal(t, byte(0xa5), d.sample)
	assert.True(t, d.sampleFull)
	assert.Equal(t, uint16(0xc001), d.address)
	assert.Equal(t, uint16(16), d.remaining)
}

func TestEnableKeepsRunningSample(t *testing.T) {
	reader := &memoryReader{}
	d := New(reader, &cycleStaller{})
	d.SetEnabled(true)
	reads := reader.reads

	d.SetEnabled(true)

	assert.Equal(t, reads, reader.reads, "a running sample is not restarted")
}

func TestDisableClearsLengthAndIRQ(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.SetEnabled(true)
	d.irq = true

	d.SetEnabled(false)

	assert.False(t, d.Active())
	assert.False(t, d.IRQ())
}

func TestSampleEndSetsIRQ(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.Write(0x4010, 0x80) // interrupt enabled, no loop
	d.Write(0x4013, 0x00) // one byte

	d.SetEnabled(true)

	assert.False(t, d.Active())
	assert.True(t, d.IRQ(), "the interrupt is set when the last byte is read")
}

func TestSampleEndRestartsWithLoop(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.Write(0x4010, 0x40) // loop, interrupt disabled
	d.Write(0x4013, 0x00) // one byte

	d.SetEnabled(true)

	assert.True(t, d.Active())
	assert.Equal(t, uint16(0xc000), d.address)
	assert.False(t, d.IRQ())
}

func TestFetchAdvancesAndWrapsAddress(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.address = 0xffff
	d.remaining = 2

	d.fetch()

	assert.Equal(t, uint16(0x8000), d.address)
	assert.Equal(t, uint16(1), d.remaining)
}

func TestFetchKeepsFullBuffer(t *testing.T) {
	reader := &memoryReader{}
	d := New(reader, &cycleStaller{})
	d.remaining = 2
	d.sampleFull = true

	d.fetch()

	assert.Equal(t, 0, reader.reads)
}

func TestClockUsesRatePeriod(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.Write(0x4010, 0x0f) // rate index 15, 27 APU cycles

	d.Clock()
	assert.Equal(t, uint16(26), d.counter)

	for range 26 {
		d.Clock()
	}
	assert.Equal(t, uint16(0), d.counter)

	d.Clock()
	assert.Equal(t, uint16(26), d.counter, "the period repeats")
}

func TestOutputUnitIncreasesLevel(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.output = 10
	d.sampleFull = true
	d.sample = 0xff

	d.clockOutput()

	assert.Equal(t, byte(12), d.output)
	assert.Equal(t, byte(0x7f), d.shift)
	assert.Equal(t, byte(7), d.bits)
	assert.False(t, d.sampleFull)
}

func TestOutputUnitDecreasesLevel(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.output = 10
	d.sampleFull = true
	d.sample = 0x00

	d.clockOutput()

	assert.Equal(t, byte(8), d.output)
}

func TestOutputLevelIsClamped(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.output = outputLevelMaximum
	d.sampleFull = true
	d.sample = 0xff

	d.clockOutput()
	assert.Equal(t, byte(outputLevelMaximum), d.output)

	d.output = 1
	d.bits = 0
	d.sampleFull = true
	d.sample = 0x00

	d.clockOutput()
	assert.Equal(t, byte(1), d.output)
}

func TestOutputUnitHoldsLevelWhileSilent(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.output = 42

	d.clockOutput()

	assert.Equal(t, byte(42), d.output)
	assert.True(t, d.silence)
}

func TestReset(t *testing.T) {
	d := New(&memoryReader{}, &cycleStaller{})
	d.Write(0x4010, 0xcf)
	d.Write(0x4011, 0x40)
	d.SetEnabled(true)

	d.Reset()

	assert.False(t, d.Active())
	assert.False(t, d.IRQ())
	assert.False(t, d.loop)
	assert.False(t, d.irqEnabled)
	assert.Equal(t, byte(0), d.output)
	assert.Equal(t, ntscRateTable[0], d.timer)
}

// memoryReader supplies sample bytes and counts the reads.
type memoryReader struct {
	data  map[uint16]byte
	reads int
}

func (m *memoryReader) Read(address uint16) byte {
	m.reads++
	return m.data[address]
}

// cycleStaller records the requested stall cycles.
type cycleStaller struct {
	total uint16
}

func (s *cycleStaller) StallCycles(cycles uint16) {
	s.total += cycles
}
