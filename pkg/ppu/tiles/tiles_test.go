package tiles

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
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

type fetchNameTable struct {
	bus.NameTable
	address uint16
}

func (nt *fetchNameTable) Fetch(address uint16) { nt.address = address }
