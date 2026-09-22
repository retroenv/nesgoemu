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

	// outputLevelStep is the level change of one sample bit.
	outputLevelStep = 2

	// outputLevelMaximum is the highest output level.
	outputLevelMaximum = 127

	// sampleBits is the number of sample bits that one sample byte holds.
	sampleBits = 8
)

// Request describes a pending DMC memory transfer.
type Request struct {
	// Address is the address of the next sample byte.
	Address uint16
	// Load selects the initial transfer after a write to $4015.
	Load bool
}

// DMC provides the delta modulation channel.
type DMC struct {
	cycle uint64 // Elapsed CPU cycles. Reset keeps the clock phase.

	irqEnabled bool
	loop       bool
	sampleAddr byte
	sampleLen  byte

	timer   uint16
	counter uint16

	address   uint16
	remaining uint16

	load       bool // The next transfer is an initial load rather than a refill.
	loadDelay  byte // CPU cycles before the initial DMA halt attempt.
	sample     byte
	sampleFull bool

	bits    byte
	shift   byte
	silence bool

	output byte

	irq bool
}

// New returns a new delta modulation channel. The system supplies sample bytes
// through CompleteDMA after it grants a transfer from DMARequest.
func New() *DMC {
	channel := &DMC{
		timer: ntscRateTable[0],
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

// Clock advances the channel by one CPU cycle. The timer advances on every
// second cycle. DMA requests use the full CPU clock.
// https://www.nesdev.org/wiki/APU_DMC
func (d *DMC) Clock() {
	if d.loadDelay > 0 {
		d.loadDelay--
	}
	d.cycle++
	if d.cycle&1 == 0 {
		return
	}
	if d.counter == 0 {
		d.clockOutput()
		d.counter = d.timer - 1
		return
	}
	d.counter--
}

// CompleteDMA stores a byte after the system completes the memory read.
func (d *DMC) CompleteDMA(value byte) {
	d.sample = value
	d.sampleFull = true
	d.load = false
	d.address++
	if d.address == 0 {
		d.address = 0x8000
	}
	if d.remaining == 0 {
		return
	}
	d.remaining--
	if d.remaining == 0 {
		if d.loop {
			d.restart()
		} else if d.irqEnabled {
			d.irq = true
		}
	}
}

// DMARequest reports a transfer that the system can attempt on a read cycle.
// The CPU can delay the transfer with writes or other DMA bus activity.
func (d *DMC) DMARequest() (Request, bool) {
	request := Request{
		Address: d.address,
		Load:    d.load,
	}
	return request, d.loadDelay == 0 && !d.sampleFull && d.remaining > 0
}

// IRQ reports whether the channel asserts an interrupt.
func (d *DMC) IRQ() bool {
	return d.irq
}

// Output returns the channel level between 0 and 127. The channel sends its
// level to the mixer whether it is enabled or not.
func (d *DMC) Output() byte {
	return d.output
}

// Reset stops playback and keeps the register settings and the DAC low bit.
// https://www.nesdev.org/wiki/CPU_power_up_state#APU
func (d *DMC) Reset() {
	d.counter = 0
	d.irq = false

	d.address = sampleBaseAddress
	d.remaining = 0
	d.sample = 0
	d.sampleFull = false
	d.shift = 0
	d.bits = sampleBits
	d.silence = true
	d.loadDelay = 0
	d.load = false
	d.output &= 1
}

// SetEnabled enables or disables automatic sample playback. Disabling the
// channel clears the bytes remaining counter and the interrupt flag. Enabling
// the channel restarts the sample when no bytes remain.
// https://www.nesdev.org/wiki/APU#Status_($4015)
func (d *DMC) SetEnabled(enabled bool) {
	if !enabled {
		d.remaining = 0
		d.irq = false
		d.loadDelay = 0
		return
	}

	if d.remaining > 0 {
		return
	}
	d.restart()
	if !d.sampleFull {
		d.load = true
		// The first halt attempt is on a get cycle, three or four CPU cycles
		// after the register write. DMARequest runs before the next Clock.
		// https://www.nesdev.org/wiki/DMA#DMC_DMA
		d.loadDelay = 2 + byte(d.cycle&1)
	}
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
	if !d.silence {
		d.changeLevel(d.shift & 1)
	}
	d.shift >>= 1
	d.bits--
	if d.bits != 0 {
		return
	}
	d.bits = sampleBits
	d.silence = !d.sampleFull
	if d.sampleFull {
		d.shift = d.sample
		d.sampleFull = false
	}
}

// changeLevel moves the output level by one sample bit. The level stays inside
// the range of the 7-bit counter.
func (d *DMC) changeLevel(bit byte) {
	if bit != 0 {
		if d.output <= outputLevelMaximum-outputLevelStep {
			d.output += outputLevelStep
		}
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
