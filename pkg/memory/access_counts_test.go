package memory

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/assert"
)

type countPPU struct {
	bus.PPU
	writes []uint16
}

func (p *countPPU) Write(address uint16, _ byte) { p.writes = append(p.writes, address) }
func (p *countPPU) Read(_ uint16) byte           { return 0 }

func TestCountAccesses(t *testing.T) {
	ppu := &countPPU{}
	memory := New(&bus.Bus{PPU: ppu})
	memory.Write(0x10, 1)
	memory.Read(0x10)

	var counts AccessCounts
	memory.CountAccesses(&counts)
	memory.Write(0x10, 2)
	memory.Write(0x2000, 0)
	memory.Write(0x2001, 0)
	memory.Write(0x2007, 0)
	memory.Write(0x2007, 0)
	memory.Write(0x3FF1, 0) // mirror of $2001
	memory.Write(0x4014, 2)
	memory.Read(0x10)
	memory.Read(0x2002)
	memory.SetDMAAccess(true)
	memory.Read(0x0200)
	memory.SetDMAAccess(false)
	memory.Read(0x11)

	assert.Equal(t, uint64(3), counts.Reads)
	assert.Equal(t, uint64(1), counts.DMAReads)
	assert.Equal(t, uint64(7), counts.Writes)
	assert.Equal(t, [8]uint64{1, 2, 0, 0, 0, 0, 0, 2}, counts.PPUWrites)
	assert.Equal(t, uint64(1), counts.OAMDMAWrites)
	assert.Equal(t, uint64(6), counts.PPURegisterWrites())
	assert.Len(t, ppu.writes, 6)

	memory.CountAccesses(nil)
	memory.Write(0x10, 3)
	memory.Read(0x10)
	assert.Equal(t, uint64(7), counts.Writes)
	assert.Equal(t, uint64(3), counts.Reads)
}

func TestCountAccessesDoNotChangeBehavior(t *testing.T) {
	plain := New(&bus.Bus{})
	counted := New(&bus.Bus{})
	var counts AccessCounts
	counted.CountAccesses(&counts)
	for address := range uint16(0x40) {
		plain.Write(address, byte(address)*3)
		counted.Write(address, byte(address)*3)
		assert.Equal(t, plain.Read(address), counted.Read(address))
		assert.Equal(t, plain.OpenBus(), counted.OpenBus())
	}
	assert.Equal(t, uint64(0x40), counts.Writes)
	assert.Equal(t, uint64(0x40), counts.Reads)
}
