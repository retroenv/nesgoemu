package dmc

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestWriteRegisters(t *testing.T) {
	d := New()
	d.Write(0x4010, 0xcf)
	assert.True(t, d.irqEnabled)
	assert.True(t, d.loop)
	assert.Equal(t, uint16(27), d.timer)
	d.Write(0x4011, 0xff)
	assert.Equal(t, byte(0x7f), d.Output())
	d.Write(0x4012, 2)
	d.Write(0x4013, 1)
	assert.Equal(t, byte(2), d.sampleAddr)
	assert.Equal(t, byte(1), d.sampleLen)
}

func TestEnableWaitsForDMA(t *testing.T) {
	for phase := range 2 {
		d := New()
		for range phase {
			d.Clock()
		}
		d.Write(0x4012, 2)
		d.Write(0x4013, 1)
		d.SetEnabled(true)
		for range 2 + phase {
			_, pending := d.DMARequest()
			assert.False(t, pending)
			d.Clock()
		}
		request, pending := d.DMARequest()
		assert.True(t, pending)
		assert.Equal(t, Request{
			Address: 0xc080,
			Load:    true,
		}, request)
		assert.Equal(t, uint16(17), d.remaining)
		d.CompleteDMA(0xa5)
		assert.Equal(t, uint16(16), d.remaining)
		assert.Equal(t, byte(0xa5), d.sample)
		_, pending = d.DMARequest()
		assert.False(t, pending, "a full buffer cannot request another byte")
	}
}

func TestEnableKeepsRunningSample(t *testing.T) {
	d := New()
	d.Write(0x4013, 1)
	d.SetEnabled(true)
	d.CompleteDMA(0xff)
	d.SetEnabled(true)
	assert.Equal(t, uint16(16), d.remaining)
	assert.Equal(t, uint16(0xc001), d.address)
}

func TestDisableClearsLengthAndIRQ(t *testing.T) {
	d := New()
	d.SetEnabled(true)
	d.irq = true
	d.SetEnabled(false)
	assert.False(t, d.Active())
	assert.False(t, d.IRQ())
	_, pending := d.DMARequest()
	assert.False(t, pending)
}

func TestSampleEndSetsIRQ(t *testing.T) {
	d := New()
	d.Write(0x4010, 0x80)
	d.SetEnabled(true)
	assert.False(t, d.IRQ())
	d.CompleteDMA(0)
	assert.False(t, d.Active())
	assert.True(t, d.IRQ())
	d.Write(0x4010, 0)
	assert.False(t, d.IRQ())
}

func TestSampleEndRestartsWithLoop(t *testing.T) {
	d := New()
	d.Write(0x4010, 0xc0)
	d.SetEnabled(true)
	d.CompleteDMA(0)
	assert.True(t, d.Active())
	assert.Equal(t, uint16(0xc000), d.address)
	assert.False(t, d.IRQ())
}

func TestDMAAdvancesAndWrapsAddress(t *testing.T) {
	d := New()
	d.address = 0xffff
	d.remaining = 2
	d.CompleteDMA(0)
	assert.Equal(t, uint16(0x8000), d.address)
	assert.Equal(t, uint16(1), d.remaining)
}

func TestClockUsesRatePeriod(t *testing.T) {
	d := New()
	d.Write(0x4010, 0x0f)
	d.silence = false
	d.shift = 0xff
	d.Write(0x4011, 10)
	d.Clock()
	assert.Equal(t, byte(12), d.Output())
	for range 53 {
		d.Clock()
		assert.Equal(t, byte(12), d.Output())
	}
	d.Clock()
	assert.Equal(t, byte(14), d.Output(), "rate 15 advances every 54 CPU cycles")
}

func TestBufferWaitsForOutputCycle(t *testing.T) {
	d := New()
	d.Write(0x4011, 10)
	d.SetEnabled(true)
	d.CompleteDMA(0xff)
	for range 8 {
		d.clockOutput()
		assert.Equal(t, byte(10), d.Output())
	}
	d.clockOutput()
	assert.Equal(t, byte(12), d.Output(), "the new byte starts on the next timer clock")
}

func TestRefillStartsWhenBufferEmpties(t *testing.T) {
	d := New()
	d.Write(0x4013, 1)
	d.SetEnabled(true)
	for range 4 {
		d.Clock()
	}
	d.CompleteDMA(0xff)
	for d.sampleFull {
		d.clockOutput()
	}
	request, pending := d.DMARequest()
	assert.True(t, pending)
	assert.Equal(t, Request{Address: 0xc001}, request)
}

func TestOutputStepPreservesParityAtLimits(t *testing.T) {
	// A step outside the DAC range leaves the output unchanged.
	// https://www.nesdev.org/wiki/APU_DMC#Output_unit
	for level := range 128 {
		for bit := range 2 {
			d := New()
			d.output = byte(level)
			want := level
			if bit == 0 && level >= 2 {
				want -= 2
			}
			if bit == 1 && level <= 125 {
				want += 2
			}
			d.changeLevel(byte(bit))
			assert.Equal(t, byte(want), d.Output())
		}
	}
}

func TestReset(t *testing.T) {
	d := New()
	d.Write(0x4010, 0xcf)
	d.Write(0x4011, 0x41)
	d.Write(0x4012, 0x23)
	d.Write(0x4013, 0x45)
	d.SetEnabled(true)
	d.Reset()
	assert.False(t, d.Active())
	assert.False(t, d.IRQ())
	assert.True(t, d.loop)
	assert.True(t, d.irqEnabled)
	assert.Equal(t, byte(1), d.Output())
	assert.Equal(t, ntscRateTable[15], d.timer)
	assert.Equal(t, byte(0x23), d.sampleAddr)
	assert.Equal(t, byte(0x45), d.sampleLen)
}
