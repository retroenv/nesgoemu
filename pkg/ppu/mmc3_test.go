package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

// TestMMC3RenderedIRQ uses the PPU fetch path, including empty sprite slots.
// The expected dots check the current PPU fetch schedule and nine-dot filter.
// This approximation can add clocks with the background table at $1000.
// https://www.nesdev.org/wiki/PPU_rendering
func TestMMC3RenderedIRQ(t *testing.T) {
	tests := []struct {
		name          string
		control, mask byte
		want          []mmc3IRQPosition
	}{
		{"sprites high", 0x08, 0x18, []mmc3IRQPosition{{261, 261}, {0, 261}, {1, 261}}},
		{"background high", 0x10, 0x18, []mmc3IRQPosition{{261, 325}, {0, 5}, {0, 325}, {1, 5}, {1, 325}}},
		{"both high", 0x18, 0x18, []mmc3IRQPosition{{0, 5}, {1, 5}}},
		{"both low", 0, 0x18, nil},
		{"background only", 0x08, 0x08, []mmc3IRQPosition{{261, 261}, {0, 261}, {1, 261}}},
		{"sprites only", 0x08, 0x10, []mmc3IRQPosition{{261, 261}, {0, 261}, {1, 261}}},
		{"empty tall sprites", 0x20, 0x18, []mmc3IRQPosition{{261, 261}, {0, 261}, {1, 261}}},
		{"rendering disabled", 0x08, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, cpu := newMMC3TestPPU(t)
			p.Step(20*341 + 1)
			assert.Equal(t, 261, p.renderState.ScanLine())
			assert.Equal(t, 0, p.renderState.Cycle())
			p.Write(0x2000, tt.control)
			p.Write(0x2001, tt.mask)
			p.Step(3 * 341)
			assert.Equal(t, tt.want, cpu.positions)
		})
	}
}

func TestMMC3CPUDataAccessClocksIRQ(t *testing.T) {
	for _, write := range []bool{false, true} {
		p, cpu := newMMC3TestPPU(t)
		mmc3DataAccess(p, 0x2000, write)
		p.Step(8)
		mmc3DataAccess(p, 0x1000, write)
		assert.Empty(t, cpu.positions)
		mmc3DataAccess(p, 0x2000, write)
		p.Step(9)
		mmc3DataAccess(p, 0x3000, write)
		assert.Len(t, cpu.positions, 1)
		assert.True(t, cpu.irq)
		p.bus.Mapper.Write(0xE000, 0)
		assert.False(t, cpu.irq)
	}
}

type mmc3IRQPosition struct {
	line, dot int
}

type mmc3TestCPU struct {
	bus.CPU
	ppu       *PPU
	irq       bool
	positions []mmc3IRQPosition
}

func (c *mmc3TestCPU) SetIRQ(active bool) {
	c.irq = active
	if active {
		c.positions = append(c.positions, mmc3IRQPosition{c.ppu.renderState.ScanLine(), c.ppu.renderState.Cycle()})
	}
}

func (c *mmc3TestCPU) TriggerNMI() {}

func newMMC3TestPPU(t *testing.T) (*PPU, *mmc3TestCPU) {
	t.Helper()
	cpu := &mmc3TestCPU{}
	cart := &cartridge.Cartridge{
		Mapper: 4,
		PRG:    make([]byte, 0x8000),
		CHR:    make([]byte, 0x2000),
	}
	system := &bus.Bus{
		Cartridge: cart,
		CPU:       cpu,
		NameTable: nametable.New(cart.Mirror),
	}
	m, err := mapper.New(system)
	assert.NoError(t, err)
	system.Mapper = m
	p := New(system)
	cpu.ppu = p
	// No sprite is visible. The PPU must still fetch patterns for all slots.
	p.Write(0x2003, 0)
	for range 64 {
		p.Write(0x2004, 0xFF)
		p.Write(0x2004, 0xFF)
		p.Write(0x2004, 0)
		p.Write(0x2004, 0)
	}
	m.Write(0xC000, 0)
	m.Write(0xE001, 0)
	return p, cpu
}

func mmc3DataAccess(p *PPU, address uint16, write bool) {
	p.Write(0x2006, byte(address>>8))
	p.Write(0x2006, byte(address))
	if write {
		p.Write(0x2007, 0)
	} else {
		p.Read(0x2007)
	}
}
