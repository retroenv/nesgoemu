package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestMapperReceivesPPUDots(t *testing.T) {
	system := &bus.Bus{Cartridge: cartridge.New()}
	timing := &timingTestMapper{Mapper: mapper.NewMockMapper(system)}
	system.Mapper = timing
	p := New(system)

	assert.Equal(t, 1, timing.enabled)
	p.Step(1)
	assert.Equal(t, []bool{false}, timing.rendering)
	assert.Equal(t, p.renderState.Cycle(), timing.cycles[0])

	p.mask.Set(0x08)
	p.Step(1)
	assert.Equal(t, []bool{false, true}, timing.rendering)
	assert.Equal(t, p.renderState.Cycle(), timing.cycles[1])
}

func TestPPUDotsWithoutMapperTiming(t *testing.T) {
	system := &bus.Bus{Cartridge: cartridge.New()}
	system.Mapper = mapper.NewMockMapper(system)
	New(system).Step(1)
}

type timingTestMapper struct {
	bus.Mapper
	enabled   int
	cycles    []int
	rendering []bool
}

func (m *timingTestMapper) EnableBusTiming() { m.enabled++ }

func (m *timingTestMapper) TickPPU(cycle, _ int, rendering bool) {
	m.cycles = append(m.cycles, cycle)
	m.rendering = append(m.rendering, rendering)
}
