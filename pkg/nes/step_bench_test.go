package nes

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

// benchmarkSystem creates a system that loops over a short program.
func benchmarkSystem(b *testing.B, options ...Option) *System {
	b.Helper()

	cart := cartridge.New()
	// LDA #$2A; STA $10; INX; JMP $8000
	copy(cart.PRG, []byte{0xa9, 0x2a, 0x85, 0x10, 0xe8, 0x4c, 0x00, 0x80})
	options = append([]Option{WithCartridge(cart)}, options...)

	sys, err := NewSystem(NewOptions(options...))
	assert.NoError(b, err)
	sys.PC = 0x8000
	return sys
}

// BenchmarkStepSystem measures the hot path with no runtime hook installed.
func BenchmarkStepSystem(b *testing.B) {
	sys := benchmarkSystem(b)
	for b.Loop() {
		_, err := sys.StepSystem()
		assert.NoError(b, err)
	}
}

// BenchmarkStepSystemWithBusObserver measures the cost of one bus cycle observer.
func BenchmarkStepSystemWithBusObserver(b *testing.B) {
	sys := benchmarkSystem(b)
	var cycles int
	remove := sys.AddBusCycleObserver(func(cycle cpu6502.BusCycle) { cycles++ })
	defer remove()

	for b.Loop() {
		_, err := sys.StepSystem()
		assert.NoError(b, err)
	}
}

// BenchmarkStepTracedRingOff measures a step with a trace ring that is off.
func BenchmarkStepTracedRingOff(b *testing.B) {
	sys := benchmarkSystem(b, WithTraceRing(NewTraceRing(0)))
	for b.Loop() {
		_, err := sys.StepTraced()
		assert.NoError(b, err)
	}
}

// BenchmarkStepTracedRingOn measures a step that captures a trace line.
func BenchmarkStepTracedRingOn(b *testing.B) {
	ring := NewTraceRing(0)
	ring.SetEnabled(true)
	sys := benchmarkSystem(b, WithTraceRing(ring))
	for b.Loop() {
		_, err := sys.StepTraced()
		assert.NoError(b, err)
	}
}
