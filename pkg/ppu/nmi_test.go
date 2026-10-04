package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestNMIDoesNotClearVBlank(t *testing.T) {
	p, cpu := newNMITestPPU(t)
	p.Write(register.PPU_CTRL, 0x80)
	p.Step(100)

	assert.Equal(t, 1, cpu.nmis)
	assert.Equal(t, byte(0x80), p.Read(register.PPU_STATUS)&0x80)
	assert.Equal(t, byte(0), p.Read(register.PPU_STATUS)&0x80)
	p.Step(100)
	assert.Equal(t, 1, cpu.nmis)
}

func TestNMIEnableDuringVBlank(t *testing.T) {
	p, cpu := newNMITestPPU(t)
	p.Step(100)
	assert.Equal(t, 0, cpu.nmis)

	p.Write(register.PPU_CTRL, 0x80)
	p.Step(3)
	assert.Equal(t, 1, cpu.nmis)
	p.Write(register.PPU_CTRL, 0x80)
	p.Step(3)
	assert.Equal(t, 1, cpu.nmis)

	p.Write(register.PPU_CTRL, 0)
	p.Write(register.PPU_CTRL, 0x80)
	p.Step(3)
	assert.Equal(t, 2, cpu.nmis)
	assert.Equal(t, byte(0x80), p.Read(register.PPU_STATUS)&0x80)

	p.Write(register.PPU_CTRL, 0)
	p.Write(register.PPU_CTRL, 0x80)
	p.Step(3)
	assert.Equal(t, 2, cpu.nmis)
}

func TestNMIRepeatsOnNextVBlank(t *testing.T) {
	p, cpu := newNMITestPPU(t)
	p.Write(register.PPU_CTRL, 0x80)
	p.Step(100)
	assert.Equal(t, 1, cpu.nmis)

	// The pre-render line clears vblank without a status read.
	p.Step(20 * 341)
	assert.Equal(t, byte(0), p.Read(register.PPU_STATUS)&0x80)
	p.Step(242 * 341)
	assert.Equal(t, 2, cpu.nmis)
	assert.Equal(t, byte(0x80), p.Read(register.PPU_STATUS)&0x80)
}

type nmiTestCPU struct {
	bus.CPU
	nmis int
}

func (c *nmiTestCPU) TriggerNMI() { c.nmis++ }

func newNMITestPPU(t *testing.T) (*PPU, *nmiTestCPU) {
	t.Helper()
	cpu := &nmiTestCPU{}
	system := &bus.Bus{
		Cartridge: cartridge.New(),
		CPU:       cpu,
	}
	system.Mapper = mapper.NewMockMapper(system)
	return New(system), cpu
}
