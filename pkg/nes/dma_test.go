package nes

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/retrogolib/assert"
)

func TestOAMDMAUsesBusCycles(t *testing.T) {
	for _, oddStart := range []bool{false, true} {
		program := []byte{0xa9, 2, 0x8d, 0x14, 0x40, 0xea}
		if oddStart {
			program = append([]byte{0x24, 0}, program...)
		}
		sys := newAudioTestSystem(t, program)
		var expected [256]byte
		for i := range expected {
			expected[i] = byte(i ^ 0xa5)
			sys.Bus.Memory.Write(0x200+uint16(i), expected[i])
		}
		if oddStart {
			_, err := sys.StepSystem()
			assert.NoError(t, err)
		}
		_, err := sys.StepSystem()
		assert.NoError(t, err)
		write, err := sys.StepSystem()
		assert.NoError(t, err)
		assert.Equal(t, uint64(4), write.CPUCycles, "DMA must wait for a read cycle")
		step, err := sys.StepSystem()
		assert.NoError(t, err)
		cycles := uint64(513 + 2)
		if oddStart {
			cycles++
		}
		assert.Equal(t, cycles, step.CPUCycles, "DMA halt, alignment, 256 transfers, then NOP")
		assert.Equal(t, expected, sys.InspectOAM())
	}
}

func TestDMCEnableDefersFetchAndIRQ(t *testing.T) {
	program := []byte{0xa9, 0x10, 0x8d, 0x15, 0x40, 0xea, 0xea, 0xea}
	sys := newAudioTestSystem(t, program)
	sys.Bus.APU.Write(0x4010, 0x80)
	var writes []apu.RegisterWrite
	sys.apu.ObserveRegisterWrites(func(write apu.RegisterWrite) { writes = append(writes, write) })
	for range 2 {
		_, err := sys.StepSystem()
		assert.NoError(t, err)
	}
	assert.Equal(t, []apu.RegisterWrite{{Cycle: 13, Address: 0x4015, Value: 0x10}}, writes)
	assert.Equal(t, byte(0x10), sys.Bus.APU.Read(0x4015), "a register write must not complete a DMA read")
	before := sys.CPU.Cycles()
	for range 3 {
		_, err := sys.StepSystem()
		assert.NoError(t, err)
	}
	assert.Equal(t, uint64(9), sys.CPU.Cycles()-before, "three NOPs and a three-cycle load DMA")
	assert.Equal(t, byte(0x80), sys.Bus.APU.Read(0x4015), "the last fetched byte sets DMC IRQ")
}
