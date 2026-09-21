// Package apu provides the APU (Audio Processing Unit).
//
// The APU contains five sound channels, the frame counter that clocks their low
// frequency units, the mixer that combines the channel levels, and the output
// stage that converts the mixed signal to host samples.
//
// Step advances the APU by CPU cycles. The pulse, noise, and DMC timers advance
// once per APU cycle, which is every second CPU cycle, and the triangle timer
// advances every CPU cycle. Each step mixes the channel levels and adds them to
// the sample output.
//
// Sources:
//   - https://www.nesdev.org/wiki/APU
//   - https://www.nesdev.org/wiki/APU_registers
//   - https://www.nesdev.org/wiki/APU_Pulse
//   - https://www.nesdev.org/wiki/APU_Triangle
//   - https://www.nesdev.org/wiki/APU_Noise
//   - https://www.nesdev.org/wiki/APU_DMC
//   - https://www.nesdev.org/wiki/APU_Frame_Counter
//   - https://www.nesdev.org/wiki/APU_Mixer
package apu

import (
	"github.com/retroenv/nesgoemu/pkg/apu/dmc"
	"github.com/retroenv/nesgoemu/pkg/apu/framecounter"
	"github.com/retroenv/nesgoemu/pkg/apu/mixer"
	"github.com/retroenv/nesgoemu/pkg/apu/noise"
	"github.com/retroenv/nesgoemu/pkg/apu/output"
	"github.com/retroenv/nesgoemu/pkg/apu/pulse"
	"github.com/retroenv/nesgoemu/pkg/apu/sampler"
	"github.com/retroenv/nesgoemu/pkg/apu/sweep"
	"github.com/retroenv/nesgoemu/pkg/apu/triangle"
	"github.com/retroenv/nesgoemu/pkg/bus"
)

// SampleRate is the output sample rate of the APU in samples per second.
// The output is mono, 16-bit signed, little-endian.
const SampleRate = 44100

// APU provides the Audio Processing Unit.
type APU struct {
	bus *bus.Bus

	pulse1   *pulse.Pulse
	pulse2   *pulse.Pulse
	triangle *triangle.Triangle
	noise    *noise.Noise
	dmc      *dmc.DMC
	frame    *framecounter.FrameCounter

	sampler *sampler.Sampler
	output  *output.Stage

	cycle       uint64
	irqAsserted bool

	writeObserver func(RegisterWrite)
}

// New returns a new APU.
func New(systemBus *bus.Bus) *APU {
	apu := &APU{bus: systemBus}
	apu.reset()

	return apu
}

// FillSamples fills the destination with mono 16-bit signed little-endian
// samples at SampleRate. The function writes silence when no samples are
// queued. The playback worker calls this method.
func (a *APU) FillSamples(destination []byte) {
	a.DrainSamples(destination)
}

// DrainSamples fills the destination and returns the number of queued sample
// frames that it removed. It writes silence after the queued samples.
func (a *APU) DrainSamples(destination []byte) int {
	return a.output.Fill(destination)
}

// ObserveRegisterWrites replaces the optional APU register-write observer.
// The emulation goroutine calls the observer before it applies each write.
func (a *APU) ObserveRegisterWrites(observer func(RegisterWrite)) {
	a.writeObserver = observer
}

// Reset returns the APU to its power-up state. Samples that wait for playback
// are kept.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (a *APU) Reset() {
	a.reset()
}

// Step advances the APU by the given number of CPU cycles.
func (a *APU) Step(cycles int) {
	for range cycles {
		a.clock()
	}
}

// clock advances the APU by one CPU cycle.
func (a *APU) clock() {
	if a.cycle&1 == 0 {
		a.pulse1.Clock()
		a.pulse2.Clock()
		a.noise.Clock()
		a.dmc.Clock()
	}
	a.triangle.Clock()

	quarter, half := a.frame.Clock()
	if quarter || half {
		a.clockFrames(quarter, half)
	}

	a.cycle++
	a.sampler.Add(a.mix())
	a.updateIRQ()
}

// clockFrames advances the low frequency units of the channels.
func (a *APU) clockFrames(quarter, half bool) {
	if quarter {
		a.pulse1.ClockQuarterFrame()
		a.pulse2.ClockQuarterFrame()
		a.triangle.ClockQuarterFrame()
		a.noise.ClockQuarterFrame()
	}

	if half {
		a.pulse1.ClockHalfFrame()
		a.pulse2.ClockHalfFrame()
		a.triangle.ClockHalfFrame()
		a.noise.ClockHalfFrame()
	}
}

// mix returns the mixed level of all channels.
// https://www.nesdev.org/wiki/APU_Mixer
func (a *APU) mix() float64 {
	return mixer.Mix(a.pulse1.Output(), a.pulse2.Output(), a.triangle.Output(), a.noise.Output(), a.dmc.Output())
}

// reset creates the channel units and clears their state.
func (a *APU) reset() {
	a.pulse1 = pulse.New(sweep.OnesComplement)
	a.pulse2 = pulse.New(sweep.TwosComplement)
	a.triangle = triangle.New()
	a.noise = noise.New()
	a.dmc = dmc.New(a.bus.Mapper, a.bus.CPU)
	a.frame = framecounter.New()

	if a.output == nil {
		a.output = output.New(SampleRate)
		a.sampler = sampler.New(SampleRate, a.output.Write)
	}

	a.cycle = 0
	a.irqAsserted = false
	a.bus.CPU.SetIRQ(false)
}

// updateIRQ drives the CPU interrupt line from the interrupt flags.
// The frame counter and the DMC channel share the line with the mapper
// interrupt, so a mapper that changes the line at the same time can overwrite
// the level of the APU.
func (a *APU) updateIRQ() {
	a.setIRQ(a.frame.IRQ() || a.dmc.IRQ())
}

// setIRQ drives the CPU interrupt line when the level changes.
func (a *APU) setIRQ(active bool) {
	if active == a.irqAsserted {
		return
	}

	a.irqAsserted = active
	a.bus.CPU.SetIRQ(active)
}
