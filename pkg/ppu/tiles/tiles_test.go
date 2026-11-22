package tiles

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/memory"
	ppuaddress "github.com/retroenv/nesgoemu/pkg/ppu/addressing"
	"github.com/retroenv/retrogolib/assert"
)

func TestNametableFetchMasksFineY(t *testing.T) {
	addr := ppuaddress.New()
	nt := &fetchNameTable{}
	tiles := New(addr, nil, nt)

	addr.SetAddress(0x3B)
	addr.SetAddress(0x45)

	tiles.FetchCycle(1)
	assert.Equal(t, uint16(0x2B45), nt.address)
}

func TestPatternReadObserverReplacement(t *testing.T) {
	const (
		lowPlaneFetchDot  = 5
		highPlaneFetchDot = 7
	)
	tiles := New(ppuaddress.New(), memory.NewRAM(0, 2*patternPlaneOffset), &fetchNameTable{})
	assert.Nil(t, tiles.patternReadCallback())
	var first, second []uint16
	tiles.ObservePatternReads(func(address uint16) {
		first = append(first, address)
	})
	tiles.FetchCycle(lowPlaneFetchDot)
	tiles.ObservePatternReads(func(address uint16) {
		second = append(second, address)
	})
	tiles.FetchCycle(highPlaneFetchDot)
	tiles.ObservePatternReads(nil)
	tiles.FetchCycle(lowPlaneFetchDot)
	assert.Equal(t, []uint16{0}, first)
	assert.Equal(t, []uint16{8}, second)
	assert.Nil(t, tiles.patternReadCallback())
}

type fetchNameTable struct {
	bus.NameTable
	address uint16
}

func (nt *fetchNameTable) Fetch(address uint16) { nt.address = address }

func (*fetchNameTable) Value() byte { return 0 }
