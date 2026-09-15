package bus

import "github.com/retroenv/retrogolib/arch/cpu/cpu6502"

// CPU represents the Central Processing Unit.
type CPU interface {
	Cycles() uint64
	StallCycles(cycles uint16)
	State() cpu6502.State

	SetIRQ(active bool)
	TriggerIrq()
	TriggerNMI()
}
