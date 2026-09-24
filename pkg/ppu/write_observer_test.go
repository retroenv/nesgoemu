package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestObserveWritesReportsPPUAddressBeforeIncrement(t *testing.T) {
	cart := cartridge.New()
	names := nametable.New(cart.Mirror)
	names.SetVRAM(make([]byte, nametable.VramSize))
	sys := &bus.Bus{
		Cartridge: cart,
		NameTable: names,
	}
	sys.Mapper = mapper.NewMockMapper(sys)
	p := New(sys)
	p.Write(register.PPU_CTRL, 0)
	p.Step(3)

	var events []WriteEvent
	p.ObserveWrites(func(event WriteEvent) { events = append(events, event) })
	p.Write(register.PPU_ADDR, 0x20)
	p.Write(register.PPU_ADDR, 0x10)
	p.Write(register.PPU_DATA, 0x2a)

	assert.Len(t, events, 3)
	assert.Equal(t, register.PPU_DATA, events[2].Register)
	assert.Equal(t, uint16(0x2010), events[2].PPUAddress)
	assert.Equal(t, byte(0x2a), events[2].Value)
	assert.Equal(t, p.renderState.Frame(), events[2].Frame)
	assert.Equal(t, p.renderState.ScanLine(), events[2].Scanline)
	assert.Equal(t, p.renderState.Cycle(), events[2].Dot)
	assert.Equal(t, uint16(0x2011), p.addressing.Address())

	replaced := 0
	p.ObserveWrites(func(WriteEvent) { replaced++ })
	p.Write(register.PPU_DATA, 0x2b)
	assert.Len(t, events, 3)
	assert.Equal(t, 1, replaced)
	p.ObserveWrites(nil)
	p.Write(register.PPU_DATA, 0x2c)
	assert.Equal(t, 1, replaced)
}
