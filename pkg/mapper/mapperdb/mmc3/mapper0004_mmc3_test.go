package mmc3

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNewAppliesPowerOnState(t *testing.T) {
	m, _ := newTestMapper(t)

	assert.Equal(t, []int{0, 0, 2, 3}, m.State().PrgWindows)
	assert.Equal(t, []int{0, 1, 0, 1, 0, 0, 0, 0}, m.State().ChrWindows)
	assert.Equal(t, cartridge.MirrorVertical, m.MirrorMode())
}

func TestFeatures(t *testing.T) {
	m, _ := newTestMapper(t)

	usage := m.Features()
	assert.Equal(t, []feature.Usage{
		{
			ID:   feature.PRGRAM,
			Name: "PRG RAM",
		},
		{
			ID:   feature.CHRBanking,
			Name: "CHR banking",
		},
		{
			ID:   feature.Mirroring,
			Name: "Mirroring",
		},
		{
			ID:   feature.PRGBanking,
			Name: "PRG banking",
		},
		{
			ID:   feature.ScanlineIRQ,
			Name: "Scanline IRQ",
		},
	}, usage)

	m.Write(0x8000, 2) // select CHR bank register 2
	m.Write(0x8001, 0)
	m.Write(0xA000, 0)
	m.Write(0x8000, 6) // select PRG bank register 6
	m.Write(0x8001, 0)
	m.Write(0xC000, 0)

	usage = m.Features()
	assert.Equal(t, []feature.Usage{
		{
			ID:   feature.PRGRAM,
			Name: "PRG RAM",
		},
		{
			ID:   feature.CHRBanking,
			Name: "CHR banking",
			Used: true,
		},
		{
			ID:   feature.Mirroring,
			Name: "Mirroring",
			Used: true,
		},
		{
			ID:   feature.PRGBanking,
			Name: "PRG banking",
			Used: true,
		},
		{
			ID:   feature.ScanlineIRQ,
			Name: "Scanline IRQ",
			Used: true,
		},
	}, usage)
}

func TestPRGBankingMode0(t *testing.T) {
	m, _ := newTestMapper(t)

	// $8000: bank register 6, $A000: bank register 7,
	// $C000: fixed second-last bank, $E000: fixed last bank.
	m.Write(0x8000, 6)
	m.Write(0x8001, 1)
	m.Write(0x8000, 7)
	m.Write(0x8001, 2)

	assert.Equal(t, byte(1), m.Read(0x8000))
	assert.Equal(t, byte(2), m.Read(0xA000))
	assert.Equal(t, byte(2), m.Read(0xC000))
	assert.Equal(t, byte(3), m.Read(0xE000))
}

func TestPRGBankingMode1(t *testing.T) {
	m, _ := newTestMapper(t)

	// $8000: fixed second-last bank, $A000: bank register 7,
	// $C000: bank register 6, $E000: fixed last bank.
	m.Write(0x8000, 0x40|6)
	m.Write(0x8001, 1)
	m.Write(0x8000, 0x40|7)
	m.Write(0x8001, 2)

	assert.Equal(t, byte(2), m.Read(0x8000))
	assert.Equal(t, byte(2), m.Read(0xA000))
	assert.Equal(t, byte(1), m.Read(0xC000))
	assert.Equal(t, byte(3), m.Read(0xE000))
}

func TestCHRBankingMode0(t *testing.T) {
	m, _ := newTestMapper(t)

	// Registers 0 and 1 form two 2K windows, registers 2-5 form four 1K
	// windows. Writes to registers 0 and 1 ignore bit 0.
	m.Write(0x8000, 0)
	m.Write(0x8001, 0x03) // masked to 0x02
	m.Write(0x8000, 2)
	m.Write(0x8001, 5)

	assert.Equal(t, byte(2), m.Read(0x0000))
	assert.Equal(t, byte(3), m.Read(0x0400))
	assert.Equal(t, byte(5), m.Read(0x1000))
}

func TestCHRBankingMode1(t *testing.T) {
	m, _ := newTestMapper(t)

	// Registers 2-5 form four 1K windows at $0000-$0FFF, registers 0 and 1
	// form two 2K windows at $1000-$1FFF.
	m.Write(0x8000, 0x80|2)
	m.Write(0x8001, 6)
	m.Write(0x8000, 0x80)
	m.Write(0x8001, 0x04)

	assert.Equal(t, byte(6), m.Read(0x0000))
	assert.Equal(t, byte(4), m.Read(0x1000))
	assert.Equal(t, byte(5), m.Read(0x1400))
}

func TestMirroring(t *testing.T) {
	m, _ := newTestMapper(t)

	m.Write(0xA000, 1)
	assert.Equal(t, cartridge.MirrorHorizontal, m.MirrorMode())

	m.Write(0xA000, 0)
	assert.Equal(t, cartridge.MirrorVertical, m.MirrorMode())
}

func TestRegisterAddressMirroring(t *testing.T) {
	m, _ := newTestMapper(t)

	// Register addresses are decoded from address bits 14, 13, and 0.
	m.Write(0x8002, 6) // mirrors $8000
	m.Write(0x8001, 1)
	m.Write(0xA002, 1) // mirrors $A000
	m.Write(0xC000, 7)
	m.Write(0xE000, 0)

	assert.Equal(t, byte(1), m.Read(0x8000))
	assert.Equal(t, cartridge.MirrorHorizontal, m.MirrorMode())
}

func TestWramAccess(t *testing.T) {
	m, _ := newTestMapper(t)

	// The WRAM is disabled at power on: reads return open bus and writes
	// are ignored.
	m.Write(0x6000, 0x5A)
	assert.Equal(t, byte(0), m.Read(0x6000))

	// Bit 7 of $A001 enables the WRAM.
	m.Write(0xA001, 0x80)
	m.Write(0x6000, 0x5A)
	assert.Equal(t, byte(0x5A), m.Read(0x6000))

	// Bit 6 of $A001 protects the WRAM from writes.
	m.Write(0xA001, 0xC0)
	m.Write(0x6000, 0x3C)
	assert.Equal(t, byte(0x5A), m.Read(0x6000))

	// A disabled WRAM returns open bus again.
	m.Write(0xA001, 0x40)
	assert.Equal(t, byte(0), m.Read(0x6000))
}

func TestIRQCounterReloadAndFire(t *testing.T) {
	m, cpu := newTestMapper(t)

	m.Write(0xC000, 1)
	m.Write(0xE001, 0)

	// The first clock reloads the zero counter from the latch.
	rise(m)
	assert.False(t, cpu.irq)

	// The second clock decrements the counter to zero and asserts the IRQ.
	rise(m)
	assert.True(t, cpu.irq)

	// $E000 disables the IRQ and acknowledges the IRQ line.
	m.Write(0xE000, 0)
	assert.False(t, cpu.irq)
}

func TestIRQReloadRegister(t *testing.T) {
	m, _ := newTestMapper(t)

	m.Write(0xC000, 3)
	m.Write(0xE001, 0)
	rise(m)
	assert.Equal(t, byte(3), m.irqCounter)

	// $C001 sets the reload flag, so the next clock reloads the latch
	// instead of decrementing the counter.
	m.Write(0xC001, 0)
	rise(m)
	assert.Equal(t, byte(3), m.irqCounter)
}

func TestIRQZeroLatchFiresOnReload(t *testing.T) {
	m, cpu := newTestMapper(t)

	// The Sharp revision asserts the IRQ when a reload loads a zero latch.
	// https://www.nesdev.org/wiki/MMC3#IRQ_Specifics
	m.Write(0xC000, 0)
	m.Write(0xE001, 0)
	rise(m)
	assert.True(t, cpu.irq)
}

func TestIRQCounterClocksWhileDisabled(t *testing.T) {
	m, cpu := newTestMapper(t)

	m.Write(0xC000, 1)
	rise(m) // reload → 1
	rise(m) // 1 → 0, but disabled: no IRQ
	assert.False(t, cpu.irq, "the IRQ must not fire while disabled")

	// A zero counter reloads on the next clock, so the IRQ fires one
	// clock after enabling.
	m.Write(0xE001, 0)
	rise(m) // reload → 1
	rise(m) // 1 → 0 → IRQ
	assert.True(t, cpu.irq)
}

func TestA12FilterIgnoresShortPulses(t *testing.T) {
	m, _ := newTestMapper(t)

	m.Write(0xC000, 1)
	m.Write(0xE001, 0)

	// A high read without a previous fall is not a rising edge.
	m.Read(0x1000)
	assert.Equal(t, byte(0), m.irqCounter)

	// A rise 4 dots after the fall is filtered.
	m.NameTableMemory().Read(0x2000)
	tick(m, 4)
	m.Read(0x1000)
	assert.Equal(t, byte(0), m.irqCounter)

	// A rise 8 dots after the fall is filtered.
	m.NameTableMemory().Read(0x2000)
	tick(m, 8)
	m.Read(0x1000)
	assert.Equal(t, byte(0), m.irqCounter)

	// A rise 9 dots after the fall clocks the counter.
	m.NameTableMemory().Read(0x2000)
	tick(m, 9)
	m.Read(0x1000)
	assert.Equal(t, byte(1), m.irqCounter)
}

func TestScanlineIRQClocksOncePerLine(t *testing.T) {
	m, cpu := newTestMapper(t)

	m.Write(0xC000, 2)
	m.Write(0xE001, 0)

	driver := &lineDriver{m: m}

	// Line 0 has no counting rise, line 1 reloads the counter, line 2
	// decrements it, line 3 decrements it to zero and asserts the IRQ.
	stepScanline(driver, 0)
	stepScanline(driver, 1)
	assert.False(t, cpu.irq)
	assert.Equal(t, byte(2), m.irqCounter)

	stepScanline(driver, 2)
	assert.False(t, cpu.irq)
	assert.Equal(t, byte(1), m.irqCounter)

	stepScanline(driver, 3)
	assert.True(t, cpu.irq)
}

func TestA12ObservesWrites(t *testing.T) {
	m, _ := newTestMapper(t)

	m.Write(0xC000, 1)
	m.Write(0xE001, 0)

	// $2007 writes toggle A12 like reads do: a nametable write keeps the
	// line low, a CHR write raises it.
	m.NameTableMemory().Write(0x2000, 0)
	tick(m, 9)
	m.Write(0x1000, 0)
	assert.Equal(t, byte(1), m.irqCounter)
}

func newTestMapper(t *testing.T) (*mapperMMC3, *irqCaptureCPU) {
	t.Helper()

	prg := make([]byte, 0x8000)
	for i := range prg {
		prg[i] = byte(i / 0x2000)
	}

	chr := make([]byte, 0x2000)
	for i := range chr {
		chr[i] = byte(i / 0x400)
	}

	cpu := &irqCaptureCPU{}
	systemBus := &bus.Bus{
		CPU: cpu,
		Cartridge: &cartridge.Cartridge{
			CHR: chr,
			PRG: prg,
		},
		NameTable: nametable.New(cartridge.MirrorVertical),
	}

	base := mapperbase.New(systemBus)
	base.SetPrgRAM(make([]byte, 0x2000))

	mapper, err := New(base)
	assert.NoError(t, err)

	m, ok := mapper.(*mapperMMC3)
	assert.True(t, ok)
	return m, cpu
}

// rise simulates an accepted A12 rising edge: a nametable read keeps A12 low
// for 9 dots before a pattern read raises it.
func rise(m *mapperMMC3) {
	m.NameTableMemory().Read(0x2000)
	tick(m, 9)
	m.Read(0x1000)
}

func tick(m *mapperMMC3, dots int) {
	for range dots {
		m.TickPPU(0, 0, true)
	}
}

// lineDriver drives PPU bus transactions at absolute PPU dot positions.
type lineDriver struct {
	ticks int
	m     *mapperMMC3
}

// step ticks the PPU up to and including the given dot, like the PPU does
// before each fetch.
func (d *lineDriver) step(absoluteDot int) {
	for d.ticks <= absoluteDot {
		d.m.TickPPU(0, 0, true)
		d.ticks++
	}
}

func (d *lineDriver) ntRead(absoluteDot int) {
	d.step(absoluteDot)
	d.m.NameTableMemory().Read(0x2000)
}

func (d *lineDriver) ptRead(absoluteDot int) {
	d.step(absoluteDot)
	d.m.Read(0x1000)
}

// stepScanline simulates one rendered scanline with the background and
// sprite pattern tables at $1xxx. Nametable reads drive A12 low and pattern
// reads drive it high at the hardware fetch cycles.
func stepScanline(d *lineDriver, line int) {
	base := line * 341

	d.ntRead(base + 1)
	d.ptRead(base + 5)
	for tile := 1; tile < 32; tile++ {
		d.ntRead(base + tile*8 + 1)
		d.ptRead(base + tile*8 + 5)
	}

	d.ntRead(base + 257)
	d.ntRead(base + 259)
	d.ptRead(base + 261)
	for slot := 1; slot < 8; slot++ {
		d.ntRead(base + 257 + slot*8)
		d.ntRead(base + 259 + slot*8)
		d.ptRead(base + 261 + slot*8)
	}

	d.ntRead(base + 321)
	d.ptRead(base + 325)
	d.ntRead(base + 329)
	d.ptRead(base + 333)
	d.ntRead(base + 337)
	d.ntRead(base + 339)
}

type irqCaptureCPU struct {
	irq bool
}

func (c *irqCaptureCPU) Cycles() uint64 { return 0 }
func (c *irqCaptureCPU) SetIRQ(active bool) {
	c.irq = active
}
func (c *irqCaptureCPU) StallCycles(uint16)   {}
func (c *irqCaptureCPU) State() cpu6502.State { return cpu6502.State{} }
func (c *irqCaptureCPU) TriggerIrq()          {}
func (c *irqCaptureCPU) TriggerNMI()          {}

func TestPRGRegistersIgnoreHighBits(t *testing.T) {
	m, _ := newTestMapper(t)
	for _, index := range []byte{6, 7} {
		m.Write(0x8000, index)
		m.Write(0x8001, 0xC1)
		assert.Equal(t, byte(1), m.registers[index])
	}
}

func TestNameTableMirrorAddressClocksA12(t *testing.T) {
	m, cpu := newTestMapper(t)
	m.Write(0xE001, 0)
	m.NameTableMemory().Read(0x2000)
	tick(m, 9)
	m.NameTableMemory().Read(0x3000)
	assert.True(t, cpu.irq)
}
