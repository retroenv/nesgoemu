// Package dmc provides the APU delta modulation channel.
// The channel plays 1-bit delta encoded samples that it reads from the CPU
// address space, and it can output a level that the CPU loads directly.
// https://www.nesdev.org/wiki/APU_DMC
package dmc

// ntscRateTable contains the sample rates in APU cycles. The source table gives
// CPU cycles, which are twice the APU cycles.
// https://www.nesdev.org/wiki/APU_DMC
var ntscRateTable = [16]uint16{
	214, 190, 170, 160, 143, 127, 113, 107, 95, 80, 71, 64, 53, 42, 36, 27,
}

const (
	// sampleBaseAddress is the first address that a sample can use.
	// The sample address register selects 64 byte steps above it.
	// https://www.nesdev.org/wiki/APU_DMC
	sampleBaseAddress = 0xc000

	// dmaStallCycles is the number of CPU cycles that a sample fetch stops the
	// CPU. The hardware stalls one to four cycles depending on the position of
	// the fetch in the instruction, as described in the DMA article.
	// https://www.nesdev.org/wiki/DMA
	dmaStallCycles = 4

	// outputLevelStep is the level change of one sample bit.
	outputLevelStep = 2

	// outputLevelMaximum is the highest output level.
	outputLevelMaximum = 127

	// sampleBits is the number of sample bits that one sample byte holds.
	sampleBits = 8
)

// Fetch records one DMC sample DMA read and its modeled CPU stall cost.
type Fetch struct {
	Address     uint16
	StallCycles uint16
}

// SampleReader reads a byte from the CPU address space.
type SampleReader interface {
	Read(address uint16) byte
}

// CycleStaller stops the CPU for a number of cycles.
type CycleStaller interface {
	StallCycles(cycles uint16)
}

// DMC provides the delta modulation channel.
type DMC struct {
	reader SampleReader
	cpu    CycleStaller

	irqEnabled bool
	loop       bool
	sampleAddr byte
	sampleLen  byte

	timer   uint16
	counter uint16
	irq     bool

	address    uint16
	remaining  uint16
	sample     byte
	sampleFull bool
	shift      byte
	bits       byte
	silence    bool

	output byte

	fetchObserver func(Fetch)
}

// New returns a new delta modulation channel. The reader supplies sample bytes
// from the CPU address space, and the staller receives the DMA stall cycles.
func New(reader SampleReader, cpu CycleStaller) *DMC {
	channel := &DMC{
		reader: reader,
		cpu:    cpu,
	}
	channel.Reset()

	return channel
}

// Active reports whether the channel has sample bytes remaining.
func (d *DMC) Active() bool {
	return d.remaining > 0
}

// ClearIRQ clears the interrupt flag.
func (d *DMC) ClearIRQ() {
	d.irq = false
}

// Clock advances the channel by one APU cycle. The output unit advances when
// the timer reaches zero.
// https://www.nesdev.org/wiki/APU_DMC
func (d *DMC) Clock() {
	if d.counter == 0 {
		d.clockOutput()
		d.counter = d.timer - 1
		return
	}
	d.counter--
}

// IRQ reports whether the channel asserts an interrupt.
func (d *DMC) IRQ() bool {
	return d.irq
}

// ObserveFetches replaces the optional sample DMA observer.
func (d *DMC) ObserveFetches(observer func(Fetch)) {
	d.fetchObserver = observer
}

// Output returns the channel level between 0 and 127. The channel sends its
// level to the mixer whether it is enabled or not.
func (d *DMC) Output() byte {
	return d.output
}

// Reset returns the channel to its power-up state.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (d *DMC) Reset() {
	d.irqEnabled = false
	d.loop = false
	d.sampleAddr = 0
	d.sampleLen = 0

	// The rate register powers up as zero and selects the first table entry.
	d.timer = ntscRateTable[0]
	d.counter = 0
	d.irq = false

	d.address = sampleBaseAddress
	d.remaining = 0
	d.sample = 0
	d.sampleFull = false
	d.shift = 0
	d.bits = 0
	d.silence = false
	d.output = 0
}

// SetEnabled enables or disables automatic sample playback. Disabling the
// channel clears the bytes remaining counter and the interrupt flag. Enabling
// the channel restarts the sample when no bytes remain.
// https://www.nesdev.org/wiki/APU#Status_($4015)
func (d *DMC) SetEnabled(enabled bool) {
	if !enabled {
		d.remaining = 0
		d.irq = false
		return
	}

	if d.remaining > 0 {
		return
	}
	d.restart()
	d.fetch()
}

// Write sets a channel register. The low two address bits select the register.
// https://www.nesdev.org/wiki/APU_DMC
func (d *DMC) Write(address uint16, value byte) {
	switch address & 0x03 {
	case 0: // interrupt enable, loop and rate
		d.irqEnabled = value&0x80 != 0
		if !d.irqEnabled {
			d.irq = false
		}
		d.loop = value&0x40 != 0
		d.timer = ntscRateTable[value&0x0f]

	case 1: // direct load
		d.output = value & 0x7f

	case 2: // sample address
		d.sampleAddr = value

	default: // sample length
		d.sampleLen = value
	}
}

// clockOutput advances the output unit by one step of the current output cycle.
// Each step consumes one sample bit, and a new cycle loads eight bits from the
// sample buffer.
// https://www.nesdev.org/wiki/APU_DMC
func (d *DMC) clockOutput() {
	d.fetch()

	if d.bits == 0 {
		d.bits = sampleBits
		if d.sampleFull {
			d.shift = d.sample
			d.sampleFull = false
			d.silence = false
		} else {
			d.silence = true
		}
	}

	if !d.silence {
		d.changeLevel(d.shift & 1)
	}
	d.shift >>= 1
	d.bits--
}

// fetch reads one sample byte into the sample buffer. The address and the bytes
// remaining counter advance, and the sample ends when the counter reaches zero.
// https://www.nesdev.org/wiki/APU_DMC
func (d *DMC) fetch() {
	if d.sampleFull || d.remaining == 0 {
		return
	}

	address := d.address
	d.sample = d.reader.Read(address)
	d.sampleFull = true
	d.cpu.StallCycles(dmaStallCycles)
	if d.fetchObserver != nil {
		d.fetchObserver(Fetch{
			Address:     address,
			StallCycles: dmaStallCycles,
		})
	}

	d.address++
	if d.address == 0 {
		d.address = 0x8000
	}

	d.remaining--
	if d.remaining > 0 {
		return
	}

	switch {
	case d.loop:
		d.restart()
	case d.irqEnabled:
		d.irq = true
	}
}

// changeLevel moves the output level by one sample bit. The level stays inside
// the range of the 7-bit counter.
func (d *DMC) changeLevel(bit byte) {
	if bit != 0 {
		d.output = min(d.output+outputLevelStep, outputLevelMaximum)
		return
	}

	if d.output >= outputLevelStep {
		d.output -= outputLevelStep
	}
}

// restart sets the address and the bytes remaining counter from the registers.
func (d *DMC) restart() {
	d.address = sampleBaseAddress + uint16(d.sampleAddr)*64
	d.remaining = uint16(d.sampleLen)*16 + 1
}
