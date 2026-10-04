// Package nmi contains the PPU NMI manager.
package nmi

import "github.com/retroenv/nesgoemu/pkg/bus"

// Nmi implements a PPU NMI manager.
type Nmi struct {
	enabled   bool
	occurred  bool
	triggered bool
}

// New returns a new mask manager.
func New() *Nmi {
	return &Nmi{}
}

// Occurred returns whether an NMI occurred.
func (n *Nmi) Occurred() bool {
	return n.occurred
}

// SetOccurred sets the occurred flag.
func (n *Nmi) SetOccurred(occurred bool) {
	n.occurred = occurred
	if !occurred {
		n.triggered = false
	}
}

// Enabled returns whether NMI triggering is enabled.
func (n *Nmi) Enabled() bool {
	return n.enabled
}

// SetEnabled sets whether NMI is enabled.
func (n *Nmi) SetEnabled(enabled bool) {
	n.enabled = enabled
	if !enabled {
		n.triggered = false
	}
}

// Trigger sends one NMI when the enabled vblank signal becomes active.
// NMI delivery does not clear vblank. A status read or the pre-render line
// clears it. Disabling NMI permits a new edge during the same vblank.
// https://www.nesdev.org/wiki/NMI
func (n *Nmi) Trigger(cpu bus.CPU) {
	if n.enabled && n.occurred && !n.triggered {
		cpu.TriggerNMI()
		n.triggered = true
	}
}
