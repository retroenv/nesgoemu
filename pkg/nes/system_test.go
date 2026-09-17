package nes

import (
	"image"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
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
