package apu

import (
	"github.com/retroenv/nesgoemu/pkg/apu/dmc"
	"github.com/retroenv/nesgoemu/pkg/apu/framecounter"
	"github.com/retroenv/nesgoemu/pkg/apu/noise"
	"github.com/retroenv/nesgoemu/pkg/apu/pulse"
	"github.com/retroenv/nesgoemu/pkg/apu/triangle"
)

// Snapshot reports channel state without clearing interrupts or reading the bus.
// Read it while emulation is stopped.
type Snapshot struct {
	DMC      dmc.Snapshot      `json:"dmc"`
	Noise    noise.Snapshot    `json:"noise"`
	Pulses   [2]pulse.Snapshot `json:"pulses"`
	Triangle triangle.Snapshot `json:"triangle"`

	Frame     framecounter.Snapshot `json:"frame_counter"`
	Registers []byte                `json:"registers"`
	Status    byte                  `json:"status"`

	Lengths [4]byte `json:"length_counters"`
	Outputs [5]byte `json:"outputs"`
}

// Snapshot returns an independent copy of the APU state.
func (a *APU) Snapshot() Snapshot {
	return Snapshot{
		DMC:   a.dmc.Snapshot(),
		Noise: a.noise.Snapshot(),
		Pulses: [2]pulse.Snapshot{
			a.pulse1.Snapshot(),
			a.pulse2.Snapshot(),
		},
		Triangle: a.triangle.Snapshot(),

		Frame:     a.frame.Snapshot(),
		Registers: append([]byte(nil), a.registers[:]...),
		Status:    a.PeekStatus(),

		Lengths: [4]byte{a.pulse1.Length(), a.pulse2.Length(), a.triangle.Length(), a.noise.Length()},
		Outputs: [5]byte{a.pulse1.Output(), a.pulse2.Output(), a.triangle.Output(), a.noise.Output(), a.dmc.Output()},
	}
}
