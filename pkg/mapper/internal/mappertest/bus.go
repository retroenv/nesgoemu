// Package mappertest provides bus fixtures for mapper tests.
package mappertest

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// OpenBusValue is the CPU data bus value in the fixture.
const OpenBusValue byte = 0x5a

// NewBus connects a cartridge to nametable memory and a fixed CPU data bus.
func NewBus(cart *cartridge.Cartridge) *bus.Bus {
	return &bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
		OpenBus:   openBus{},
	}
}

type openBus struct{}

func (openBus) OpenBus() byte { return OpenBusValue }
