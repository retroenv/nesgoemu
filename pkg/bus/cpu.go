package bus

import "github.com/retroenv/retrogolib/arch/cpu/m6502"

// CPU represents the Central Processing Unit.
type CPU interface {
	Cycles() uint64
	StallCycles(cycles uint16)
	State() m6502.State
	TriggerIrq()
	TriggerNMI()
}
