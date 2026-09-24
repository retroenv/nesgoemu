package nes

import (
	"image"
	"io"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestClockComponents(t *testing.T) {
	mapper := &clockMapper{}
	ppu := &clockPPU{}
	sys := &System{Bus: &bus.Bus{
		Mapper: mapper,
		PPU:    ppu,
	}}

	sys.clockComponents(3)

	assert.Equal(t, []uint64{1, 1, 1}, mapper.cycles)
	assert.Equal(t, []int{3, 3, 3}, ppu.cycles)
}

func TestClockComponentsWithoutMapperClock(t *testing.T) {
	ppu := &clockPPU{}
	sys := &System{Bus: &bus.Bus{
		Mapper: plainMapper{},
		PPU:    ppu,
	}}

	sys.clockComponents(2)

	assert.Equal(t, []int{3, 3}, ppu.cycles)
}

func TestNewSystemWiresOpenBus(t *testing.T) {
	t.Parallel()

	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	sys.Bus.Memory.Write(0x0000, 0x5A)

	// The mapper reads the CPU data bus for addresses without memory.
	assert.Equal(t, byte(0x5A), sys.Bus.Mapper.Read(0x4020))
}

func TestObserveCPUWritesForwardsBusWrites(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	var writes []memory.WriteEvent
	sys.ObserveCPUWrites(func(write memory.WriteEvent) {
		writes = append(writes, write)
	})
	sys.Bus.Memory.Write(0x0123, 0x5a)

	assert.Equal(t, []memory.WriteEvent{{
		Frame:     sys.Bus.PPU.Frame(),
		CPUCycles: sys.CPU.Cycles(),
		Address:   0x0123,
		Value:     0x5a,
	}}, writes)
}

func TestCPUPreExecutionHookRunsWithTracing(t *testing.T) {
	recorder := &cpuHookRecorder{}
	sys, err := NewSystem(NewOptions(
		WithCartridge(cartridge.New()),
		WithTracingTarget(io.Discard),
		WithCPUPreExecutionHook(recorder.observe),
	))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0xa9)
	sys.Bus.Memory.Write(1, 0x2a)
	sys.PC = 0

	_, err = sys.StepSystem()

	assert.NoError(t, err)
	assert.Equal(t, 1, recorder.calls)
	assert.Equal(t, "LDA #$2A", recorder.traceData)
	assert.Equal(t, "LDA #$2A", sys.CPU.TraceStep.CustomData)
}

func TestCPUPreExecutionHookRunsWithoutTracing(t *testing.T) {
	recorder := &cpuHookRecorder{}
	sys, err := NewSystem(NewOptions(
		WithCartridge(cartridge.New()),
		WithCPUPreExecutionHook(recorder.observe),
	))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0, 0xea)
	sys.PC = 0

	_, err = sys.StepSystem()

	assert.NoError(t, err)
	assert.Equal(t, 1, recorder.calls)
	assert.Equal(t, "", recorder.traceData)
}

func TestCPUInstructionWriteReportsCurrentPPUPosition(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	program := []byte{0xa9, 0x2a, 0x8d, 0x07, 0x20}
	for address, value := range program {
		sys.Bus.Memory.Write(uint16(address), value)
	}
	sys.PC = 0
	sys.Bus.PPU.Write(register.PPU_ADDR, 0x20)
	sys.Bus.PPU.Write(register.PPU_ADDR, 0x10)

	_, err = sys.StepSystem()
	assert.NoError(t, err)
	cyclesBeforeWrite := sys.CPU.Cycles()

	var cpuWrites []memory.WriteEvent
	var ppuWrites []ppu.WriteEvent
	sys.ObserveCPUWrites(func(write memory.WriteEvent) { cpuWrites = append(cpuWrites, write) })
	sys.Bus.PPU.(*ppu.PPU).ObserveWrites(func(write ppu.WriteEvent) { ppuWrites = append(ppuWrites, write) })
	sys.Bus.PPU.Write(register.PPU_CTRL, 0)

	_, err = sys.StepSystem()
	assert.NoError(t, err)
	sys.Bus.PPU.Write(register.PPU_CTRL, 0)

	assert.Len(t, cpuWrites, 1)
	assert.Equal(t, register.PPU_DATA, cpuWrites[0].Address)
	assert.Equal(t, cyclesBeforeWrite+4, cpuWrites[0].CPUCycles)
	assert.Len(t, ppuWrites, 3)
	assert.Equal(t, register.PPU_DATA, ppuWrites[1].Register)
	assert.Equal(t, uint16(0x2010), ppuWrites[1].PPUAddress)
	assert.Equal(t, cpuWrites[0].CPUCycles, ppuWrites[1].CPUCycles)
	assert.Equal(t, ppuWrites[0].Scanline, ppuWrites[1].Scanline)
	assert.Equal(t, ppuWrites[0].Dot, ppuWrites[1].Dot)
	assert.NotEqual(t, ppuWrites[1].Dot, ppuWrites[2].Dot)
}

type cpuHookRecorder struct {
	calls     int
	traceData string
}

func (r *cpuHookRecorder) observe(cpu *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) {
	r.calls++
	r.traceData = cpu.TraceStep.CustomData
}

type plainMapper struct{}

func (plainMapper) MirrorMode() cartridge.MirrorMode {
	return cartridge.MirrorHorizontal
}

func (plainMapper) Read(_ uint16) byte {
	return 0
}

func (plainMapper) State() bus.MapperState {
	return bus.MapperState{}
}

func (plainMapper) Write(_ uint16, _ byte) {}

type clockMapper struct {
	plainMapper
	cycles []uint64
}

func (mapper *clockMapper) ClockCPU(cycles uint64) {
	mapper.cycles = append(mapper.cycles, cycles)
}

type clockPPU struct {
	cycles []int
}

func (ppu *clockPPU) Image() *image.RGBA {
	return nil
}

func (ppu *clockPPU) Frame() uint64 {
	return 0
}

func (ppu *clockPPU) OAM() [256]byte {
	return [256]byte{}
}

func (ppu *clockPPU) Palette() bus.Palette {
	return nil
}

func (ppu *clockPPU) Read(_ uint16) byte {
	return 0
}

func (ppu *clockPPU) Step(cycles int) {
	ppu.cycles = append(ppu.cycles, cycles)
}

func (ppu *clockPPU) Write(_ uint16, _ byte) {}
