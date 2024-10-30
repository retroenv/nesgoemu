// Package apu provides APU (Audio Processing Unit) functionality.
package apu

import "github.com/retroenv/nesgoemu/pkg/bus"

type APU struct {
	bus *bus.Bus
}

// New returns a new APU.
func New(bus *bus.Bus) *APU {
	return &APU{
		bus: bus,
	}
}
