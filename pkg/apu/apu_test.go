package apu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/assert"
)

func TestWriteEnablesChannelsAndReadsStatus(t *testing.T) {
	a, _ := newTestAPU()

	a.Write(0x4015, 0x0f)
	a.Write(0x4003, 0x00) // pulse 1
	a.Write(0x4007, 0x00) // pulse 2
	a.Write(0x400b, 0x00) // triangle
	a.Write(0x400f, 0x00) // noise

	assert.Equal(t, byte(0x0f), a.Read(0x4015))
}

func TestWriteStatusDiscardsLengthsWhenDisabled(t *testing.T) {
	a, _ := newTestAPU()
	a.Write(0x4015, 0x01)
	a.Write(0x4003, 0x00)
	assert.Equal(t, byte(0x01), a.Read(0x4015))

	a.Write(0x4015, 0x00)

	assert.Equal(t, byte(0x00), a.Read(0x4015))
}

func TestReadOfWriteOnlyRegister(t *testing.T) {
	a, _ := newTestAPU()

	assert.Equal(t, byte(0xff), a.Read(0x4000))
	assert.Equal(t, byte(0xff), a.Read(0x4017))
}

func TestWriteOfUnmappedRegisterIsIgnored(t *testing.T) {
	a, _ := newTestAPU()

	a.Write(0x4016, 0xff)

	assert.Equal(t, byte(0x00), a.Read(0x4015))
}

func TestObserveRegisterWritesReportsCycleAddressAndValue(t *testing.T) {
	a, _ := newTestAPU()
	var writes []RegisterWrite
	a.ObserveRegisterWrites(func(write RegisterWrite) {
		writes = append(writes, write)
	})
	a.Step(17)

	a.Write(0x4000, 0xbf)
	a.Write(0x4016, 0xff)

	assert.Equal(t, []RegisterWrite{{Cycle: 17, Address: 0x4000, Value: 0xbf}}, writes)
}

func TestFrameIRQDrivesCPULine(t *testing.T) {
	a, cpu := newTestAPU()

	a.Step(29828)
	assert.True(t, cpu.irqLine)

	assert.Equal(t, byte(0x80), a.Read(0x4015))
	assert.False(t, cpu.irqLine, "the read clears the flag and the line")
}

func TestWriteFrameInhibitClearsIRQ(t *testing.T) {
	a, cpu := newTestAPU()
	a.Step(29828)

	a.Write(0x4017, 0x40)

	assert.False(t, a.frame.IRQ())
	assert.False(t, cpu.irqLine)
}

func TestDMCSampleSetsIRQ(t *testing.T) {
	a, cpu := newTestAPU()
	a.Write(0x4010, 0x80) // interrupt enabled
	a.Write(0x4013, 0x00) // one byte

	a.Write(0x4015, 0x10) // enable the DMC, which fetches the byte

	assert.True(t, cpu.irqLine)
	assert.Equal(t, uint16(4), cpu.stalls)
	assert.Equal(t, byte(0x40), a.Read(0x4015))
	assert.True(t, cpu.irqLine, "a read does not clear the DMC interrupt")
}

func TestStepAdvancesCycles(t *testing.T) {
	a, _ := newTestAPU()

	a.Step(10)

	assert.Equal(t, uint64(10), a.cycle)
}

func TestStepProducesSamples(t *testing.T) {
	a, _ := newTestAPU()

	a.Step(44100)

	assert.Equal(t, 1086, a.output.Queued(), "44100 CPU cycles at 44100 Hz")
}

func TestFillSamplesWritesSilence(t *testing.T) {
	a, _ := newTestAPU()
	buffer := make([]byte, 8)

	a.FillSamples(buffer)

	assert.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0}, buffer)
}

func TestDrainSamplesReportsQueuedFrames(t *testing.T) {
	a, _ := newTestAPU()
	a.Step(100)
	buffer := make([]byte, 16)

	frames := a.DrainSamples(buffer)

	assert.Equal(t, 2, frames)
}

func TestResetClearsChannels(t *testing.T) {
	a, cpu := newTestAPU()
	a.Write(0x4015, 0x0f)
	a.Write(0x4003, 0x00)
	a.Step(29828)
	assert.True(t, cpu.irqLine)

	a.Reset()

	assert.Equal(t, byte(0x00), a.Read(0x4015))
	assert.False(t, cpu.irqLine)
}

// testCPU records the interrupt line and the stall cycles of the APU.
type testCPU struct {
	irqLine bool
	stalls  uint16
}

func (c *testCPU) Cycles() uint64 {
	return 0
}

func (c *testCPU) SetIRQ(active bool) {
	c.irqLine = active
}

func (c *testCPU) StallCycles(cycles uint16) {
	c.stalls += cycles
}

func (c *testCPU) State() cpu6502.State {
	return cpu6502.State{}
}

func (c *testCPU) TriggerIrq() {}

func (c *testCPU) TriggerNMI() {}

// testMapper supplies sample bytes to the DMC channel.
type testMapper struct {
	bus.Mapper
}

func (m *testMapper) Read(_ uint16) byte {
	return 0
}

// newTestAPU returns an APU that is connected to a test CPU and mapper.
func newTestAPU() (*APU, *testCPU) {
	cpu := &testCPU{}
	systemBus := &bus.Bus{
		CPU:    cpu,
		Mapper: &testMapper{},
	}

	return New(systemBus), cpu
}
