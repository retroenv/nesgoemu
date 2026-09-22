package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/nesgoemu/pkg/ppu/openbus"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func newOpenBusTestPPU() *PPU {
	cart := cartridge.New()
	nameTable := nametable.New(cart.Mirror)
	nameTable.SetVRAM(make([]byte, nametable.VramSize))
	systemBus := &bus.Bus{
		Cartridge: cart,
		NameTable: nameTable,
	}
	systemBus.Mapper = mapper.NewMockMapper(systemBus)

	return New(systemBus)
}

func TestWriteOnlyPortsReturnPPUOpenBus(t *testing.T) {
	t.Parallel()

	p := newOpenBusTestPPU()
	p.Write(register.PPU_ADDR, 0x3F)

	assert.Equal(t, byte(0x3F), p.Read(register.PPU_CTRL))
	assert.Equal(t, byte(0x3F), p.Read(register.PPU_MASK))
	assert.Equal(t, byte(0x3F), p.Read(register.OAM_ADDR))
	assert.Equal(t, byte(0x3F), p.Read(register.PPU_SCROLL))
	assert.Equal(t, byte(0x3F), p.Read(register.PPU_ADDR))
}

func TestStatusReadKeepsOpenBusBits(t *testing.T) {
	t.Parallel()

	p := newOpenBusTestPPU()
	p.Write(register.PPU_SCROLL, 0x1F)
	p.nmi.SetOccurred(true)

	// The read loads bits 7-5 of the bus and keeps bits 4-0, the combined value
	// stays on the bus.
	assert.Equal(t, byte(0x9F), p.Read(register.PPU_STATUS))
	assert.Equal(t, byte(0x9F), p.Read(register.PPU_CTRL))
}

func TestOAMDataReadLoadsPPUOpenBus(t *testing.T) {
	t.Parallel()

	p := newOpenBusTestPPU()
	p.Write(register.OAM_ADDR, 0x04)
	p.Write(register.OAM_DATA, 0x5A)
	p.Write(register.OAM_ADDR, 0x04)

	assert.Equal(t, byte(0x5A), p.Read(register.OAM_DATA))
	assert.Equal(t, byte(0x5A), p.Read(register.PPU_CTRL))
}

func TestDataReadLoadsPPUOpenBus(t *testing.T) {
	t.Parallel()

	p := newOpenBusTestPPU()
	p.bus.Mapper.Write(0x0010, 0x5A)
	p.Write(register.PPU_ADDR, 0x00)
	p.Write(register.PPU_ADDR, 0x10)

	assert.Equal(t, byte(0), p.Read(register.PPU_DATA), "the first read returns the empty buffer")
	// The read refreshes the decay register with the value that it returns.
	assert.Equal(t, byte(0), p.Read(register.PPU_CTRL))
	assert.Equal(t, byte(0x5A), p.Read(register.PPU_DATA), "the second read returns the buffered byte")
	assert.Equal(t, byte(0x5A), p.Read(register.PPU_CTRL))
}

func TestPaletteReadKeepsPPUOpenBusBits(t *testing.T) {
	t.Parallel()

	p := newOpenBusTestPPU()
	p.palette.Write(0, 0x2A)
	p.Write(register.PPU_ADDR, 0x3F)
	p.Write(register.PPU_ADDR, 0x00)
	p.Write(register.OAM_ADDR, 0x04)
	p.Write(register.OAM_DATA, 0xC0) // loads 1100 0000 onto the PPU I/O bus

	// Palette RAM drives bits 5-0 of the bus and keeps bits 7-6.
	assert.Equal(t, byte(0xEA), p.Read(register.PPU_DATA))
	assert.Equal(t, byte(0xEA), p.Read(register.PPU_CTRL))
}

func TestPaletteReadLoadsShadowNametableIntoDataBuffer(t *testing.T) {
	t.Parallel()

	p := newOpenBusTestPPU()
	p.memory.Write(0x2F00, 0x5A)
	p.palette.Write(0x3F00, 0x2A)
	p.Write(register.PPU_ADDR, 0x3F)
	p.Write(register.PPU_ADDR, 0x00)

	assert.Equal(t, byte(0x2A), p.Read(register.PPU_DATA))

	p.Write(register.PPU_ADDR, 0x20)
	p.Write(register.PPU_ADDR, 0x00)

	assert.Equal(t, byte(0x5A), p.Read(register.PPU_DATA))
}

func TestOAMAttributeBitsReadAsZero(t *testing.T) {
	t.Parallel()

	p := newOpenBusTestPPU()
	p.Write(register.OAM_ADDR, 0x02)
	p.Write(register.OAM_DATA, 0xFF)
	p.Write(register.OAM_ADDR, 0x02)

	// The three unimplemented bits of the attribute byte read back as zero.
	assert.Equal(t, byte(0xE3), p.Read(register.OAM_DATA))
}

func TestDecayValueDecaysWhileRunning(t *testing.T) {
	t.Parallel()

	p := newOpenBusTestPPU()
	p.Write(register.PPU_ADDR, 0xFF)

	p.Step(openbus.DecayCycles - 1)
	assert.Equal(t, byte(0xFF), p.Read(register.PPU_CTRL), "the value is kept within the decay time")

	p.Step(1)
	assert.Equal(t, byte(0), p.Read(register.PPU_CTRL), "the value decays to zero")
}

func TestOAMDMAputsLastByteOnPPUOpenBus(t *testing.T) {
	t.Parallel()

	systemBus := &bus.Bus{
		Cartridge: cartridge.New(),
		CPU:       &openBusTestCPU{},
	}
	systemBus.Mapper = mapper.NewMockMapper(systemBus)
	systemMemory := memory.New(systemBus)
	systemBus.Memory = systemMemory
	p := New(systemBus)

	systemMemory.Write(0x03FF, 0x77)
	p.Write(register.OAM_DMA, 0x03)

	assert.Equal(t, byte(0x77), p.Read(register.PPU_CTRL))
}

type openBusTestCPU struct {
	bus.CPU
}

func (c *openBusTestCPU) Cycles() uint64 {
	return 0
}

func (c *openBusTestCPU) StallCycles(uint16) {}
