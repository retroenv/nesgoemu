package framecounter

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestFourStepSequenceClocks(t *testing.T) {
	f := New()
	r := &recorder{}

	r.clock(f, 29830)

	assert.Equal(t, []uint64{7457, 14913, 22371, 29829}, r.quarter)
	assert.Equal(t, []uint64{14913, 29829}, r.half)
	assert.Equal(t, []uint64{29828}, r.irqAt)
}

func TestSequenceKeepsItsPeriod(t *testing.T) {
	f := New()
	r := &recorder{}
	r.clock(f, 59660)

	assert.Equal(t, []uint64{7457, 14913, 22371, 29829, 37287, 44743, 52201, 59659}, r.quarter)
}

func TestFrameIRQReassertsOnThreeCycles(t *testing.T) {
	f := New()
	clock(f, 29827)
	for range 3 {
		f.Clock()
		assert.True(t, f.IRQ())
		f.ClearIRQ()
	}
	f.Clock()
	assert.False(t, f.IRQ())
}

func TestPendingWriteKeepsOldSequenceRunning(t *testing.T) {
	f := New()
	clock(f, 7456)
	f.Write(0)
	quarter, half := f.Clock()

	assert.True(t, quarter)
	assert.False(t, half)
}

func TestWriteDelayDependsOnClockPhase(t *testing.T) {
	for phase := range 2 {
		f := New()
		clock(f, phase)
		f.Write(0x80)
		for range 2 + phase {
			quarter, half := f.Clock()
			assert.False(t, quarter)
			assert.False(t, half)
		}
		quarter, half := f.Clock()
		assert.True(t, quarter)
		assert.True(t, half)
	}
}

func TestFiveStepSequenceClocks(t *testing.T) {
	f := New()
	f.Write(0x80)

	assert.Equal(t, [2]bool{true, true}, clockSteps(t, f, resetDelayCycles))

	r := &recorder{}
	r.clock(f, 37282)

	assert.Equal(t, []uint64{7457, 14913, 22371, 37281}, r.quarter)
	assert.Equal(t, []uint64{14913, 37281}, r.half)
	assert.Len(t, r.irqAt, 0, "the 5-step sequence never sets the interrupt")
}

func TestWriteWithoutModeDoesNotClock(t *testing.T) {
	f := New()
	f.Write(0x00)

	assert.Equal(t, [2]bool{false, false}, clockSteps(t, f, resetDelayCycles))
}

func TestWriteResetsSequenceAfterDelay(t *testing.T) {
	f := New()
	clock(f, 100)
	assert.Equal(t, uint64(100), f.cycles)

	f.Write(0x00)

	clock(f, resetDelayCycles)
	assert.Equal(t, uint64(0), f.cycles)
	assert.Equal(t, 0, f.step)
}

func TestWriteWithInhibitClearsIRQ(t *testing.T) {
	f := New()
	clock(f, 29828)
	assert.True(t, f.IRQ())

	f.Write(0x40)

	assert.False(t, f.IRQ())
}

func TestWriteKeepsIRQWithoutInhibit(t *testing.T) {
	f := New()
	clock(f, 29828)

	f.Write(0x00)

	assert.True(t, f.IRQ())
}

func TestClearIRQ(t *testing.T) {
	f := New()
	clock(f, 29828)

	f.ClearIRQ()

	assert.False(t, f.IRQ())
}

func TestReset(t *testing.T) {
	f := New()
	f.Write(0xc0)
	clock(f, 29828)

	f.Reset()

	assert.False(t, f.IRQ())
	assert.True(t, f.mode)
	assert.True(t, f.inhibit)
	assert.Equal(t, uint64(0), f.cycles)
}

// recorder collects the CPU cycles of the frame counter events.
type recorder struct {
	quarter []uint64
	half    []uint64
	irqAt   []uint64
}

// clock advances the counter for the given number of cycles.
func (r *recorder) clock(f *FrameCounter, cycles int) {
	for cycle := 1; cycle <= cycles; cycle++ {
		quarter, half := f.Clock()
		if quarter {
			r.quarter = append(r.quarter, uint64(cycle))
		}
		if half {
			r.half = append(r.half, uint64(cycle))
		}
		if f.IRQ() && len(r.irqAt) == 0 {
			r.irqAt = append(r.irqAt, uint64(cycle))
		}
	}
}

// clockSteps advances the counter by one cycle and returns the clock signals.
func clockSteps(t *testing.T, f *FrameCounter, cycles int) [2]bool {
	t.Helper()

	quarter, half := false, false
	for range cycles {
		quarter, half = f.Clock()
	}

	return [2]bool{quarter, half}
}

// clock advances the counter by one cycle for the given number of cycles.
func clock(f *FrameCounter, cycles int) {
	for range cycles {
		f.Clock()
	}
}
