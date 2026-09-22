package rainbow

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/assert"
)

func TestOAMGeneratedUpdates(t *testing.T) {
	for _, limit := range []byte{0, 16, 63} {
		m, system := newTestMapperWithBus(t, 0x8000, 0x2000)
		m.Write(regOAMLimit, limit)
		m.Write(regOAMSlowPage, 0xFF)
		m.Write(regOAMExtendedPage, 0xFE)
		for i := range 256 {
			m.fpgaRAM[0x1F00+i] = byte(i)
			m.fpgaRAM[0x1E00+i] = byte(i + 1)
		}
		cpu := newOAMCPU(t, m, system)
		cycles := runOAMRoutine(t, m, cpu, oamRoutineStart)
		// Totals include the caller's JSR and the generated RTS.
		assert.Equal(t, uint64(12+(int(limit)+1)*24), cycles)
		for i := range 256 {
			system.PPU.Write(0x2003, byte(i))
			expected := byte(0)
			if i < (int(limit)+1)*4 {
				expected = byte(i)
				if i%4 == 2 { // the unimplemented attribute bits read back as zero
					expected &= 0xE3
				}
			}
			assert.Equal(t, expected, system.PPU.Read(0x2004))
		}
		cycles = runOAMRoutine(t, m, cpu, oamSpriteRoutineStart)
		assert.Equal(t, uint64(12+(int(limit)+1)*6), cycles)
		for i := range 64 {
			expected := byte(0)
			if i <= int(limit) {
				expected = byte(i*4 + 1)
			}
			assert.Equal(t, expected, m.spriteBankLower[i])
		}
	}
}

func TestOAMSlowUpdatePreservesStartAddress(t *testing.T) {
	m, system := newTestMapperWithBus(t, 0x8000, 0x2000)
	cpu := newOAMCPU(t, m, system)
	m.Write(regOAMLimit, 0)
	copy(m.fpgaRAM[0x1F00:], []byte{0x11, 0x22, 0x33, 0x44})
	system.PPU.Write(0x2003, 0xFE)

	assert.Equal(t, uint64(36), runOAMRoutine(t, m, cpu, oamRoutineStart))
	// The first value is an attribute byte, its unimplemented bits read back as zero.
	for i, expected := range []byte{0x11 & 0xE3, 0x22, 0x33, 0x44} {
		system.PPU.Write(0x2003, byte(0xFE+i))
		assert.Equal(t, expected, system.PPU.Read(0x2004))
	}
	system.PPU.Write(0x2003, 2)
	assert.Equal(t, byte(0), system.PPU.Read(0x2004))
}

func TestOAMCodeLockAndRemovedClearEntry(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regOAMLimit, 0)
	m.Write(0x4F00, 0x12)
	assert.Equal(t, byte(0xA9), m.Read(oamRoutineStart))
	m.Write(0x4F00, 0x34)
	assert.Equal(t, byte(0x12), m.Read(oamRoutineStart+1))
	// This STA opcode overlaps the other entry point. It must not generate
	// the extended routine, but it releases the lock for the next call.
	assert.Equal(t, byte(0x8D), m.Read(oamSpriteRoutineStart))
	assert.False(t, m.oamCodeLocked)
	m.Read(oamRoutineStart)
	assert.Equal(t, byte(0x34), m.Read(oamRoutineStart+1))
	m.Read(oamSpriteRoutineStart)
	m.oamCode[6] = 0x5A
	assert.Equal(t, byte(0x5A), m.Read(oamRoutineStart+6))
}

func newOAMCPU(t *testing.T, m *Mapper, system *bus.Bus) *cpu6502.CPU {
	t.Helper()
	system.Mapper = m
	system.Memory = memory.New(system)
	mem, err := cpu6502.NewMemory(system.Memory)
	assert.NoError(t, err)
	cpu := cpu6502.New(mem)
	system.CPU = cpu
	system.PPU = ppu.New(system)
	return cpu
}

func runOAMRoutine(t *testing.T, m *Mapper, cpu *cpu6502.CPU, address uint16) uint64 {
	t.Helper()
	copy(m.prgROM, []byte{0x20, byte(address), byte(address >> 8), 0xEA})
	cpu.PC = 0x8000
	start := cpu.Cycles()
	for range 600 {
		assert.NoError(t, cpu.Step())
		if cpu.PC == 0x8003 {
			return cpu.Cycles() - start
		}
	}
	t.Fatal("OAM routine did not return")
	return 0
}
