// Package envelope provides the APU volume envelope unit for the pulse and
// noise channels.
// https://www.nesdev.org/wiki/APU_Envelope
package envelope

import (
	"fmt"

	"github.com/retroenv/nesgoemu/internal/stateio"
)

// Envelope provides a decreasing volume level with an optional loop.
type Envelope struct {
	start    bool
	divider  byte
	decay    byte
	loop     bool
	constant bool
	volume   byte
}

// Snapshot reports the decoded channel state without bus effects.
// Read it while emulation is stopped.
type Snapshot struct {
	Constant bool `json:"constant"`
	Loop     bool `json:"loop"`
	Volume   byte `json:"volume"`

	Decay   byte `json:"decay"`
	Divider byte `json:"divider"`
	Start   bool `json:"start"`
}

// New returns a new envelope.
func New() *Envelope {
	return &Envelope{}
}

// Clock advances the envelope by one quarter frame.
func (e *Envelope) Clock() {
	if e.start {
		e.start = false
		e.decay = 15
		e.divider = e.volume

		return
	}

	if e.divider > 0 {
		e.divider--
		return
	}

	e.divider = e.volume
	switch {
	case e.decay > 0:
		e.decay--
	case e.loop:
		e.decay = 15
	}
}

// Loop reports the loop flag. The length counter uses it as its halt input.
func (e *Envelope) Loop() bool {
	return e.loop
}

// Output returns the current volume level between 0 and 15.
func (e *Envelope) Output() byte {
	if e.constant {
		return e.volume
	}

	return e.decay
}

// Reset clears the envelope counters and keeps the register settings.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (e *Envelope) Reset() {
	e.start = false
	e.divider = 0
	e.decay = 0
}

// Start sets the start flag. A channel sets it when it writes a length load
// register, which restarts the envelope.
func (e *Envelope) Start() {
	e.start = true
}

// Write sets the envelope parameters from a volume register value.
// Bit 5 is the length counter halt and envelope loop flag, bit 4 selects the
// constant volume, and bits 0 to 3 hold the volume or divider period.
func (e *Envelope) Write(value byte) {
	e.loop = value&0x20 != 0
	e.constant = value&0x10 != 0
	e.volume = value & 0x0f
}

// TransferState transfers stopped Envelope state. The system owns the record version.
func (e *Envelope) TransferState(archive *stateio.Archive) error {
	if err := archive.Values(&e.start, &e.divider, &e.decay, &e.loop, &e.constant, &e.volume); err != nil {
		return fmt.Errorf("transferring envelope state: %w", err)
	}
	return nil
}

// Snapshot returns an independent copy of the current state.
func (e *Envelope) Snapshot() Snapshot {
	return Snapshot{
		Constant: e.constant,
		Loop:     e.loop,
		Volume:   e.volume,

		Decay:   e.decay,
		Divider: e.divider,
		Start:   e.start,
	}
}
