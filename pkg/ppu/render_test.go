package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/addressing"
	ppumemory "github.com/retroenv/nesgoemu/pkg/ppu/memory"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestUnusedNametableReads(t *testing.T) {
	nt := &recordNameTable{}
	p := &PPU{
		addressing: addressing.New(),
		memory:     ppumemory.New(nil, nt, nil),
	}
	var readCycles []int

	for cycle := range 341 {
		before := len(nt.addresses)
		p.renderLine(cycle, false)
		if len(nt.addresses) != before {
			assert.Len(t, nt.addresses, before+1)
			readCycles = append(readCycles, cycle)
		}
	}
	assert.Equal(t, []int{257, 259, 265, 267, 273, 275, 281, 283,
		289, 291, 297, 299, 305, 307, 313, 315, 337, 339}, readCycles)
}

func TestFirstSpriteFetchMixedAddress(t *testing.T) {
	nt := &recordNameTable{}
	p := &PPU{
		addressing: addressing.New(),
		memory:     ppumemory.New(nil, nt, nil),
	}

	p.addressing.SetAddress(0x21)
	p.addressing.SetAddress(0xA5)
	p.addressing.SetScroll(7 * 8)
	p.addressing.SetTempNameTables(1, 0)

	p.renderLine(257, false)
	p.renderLine(259, false)
	assert.Equal(t, []uint16{0x25A5, 0x25A7}, nt.addresses)
}

// Grayscale changes pixels without changing the palette RAM values.
func TestGrayscalePixelOutput(t *testing.T) {
	system := &bus.Bus{Cartridge: cartridge.New()}
	system.Mapper = mapper.NewMockMapper(system)
	p := New(system)

	for p.renderState.ScanLine() != 0 {
		p.renderState.Tick(p.mask)
	}

	for value := range 64 {
		p.palette.Write(0, byte(value))
		for _, grayscale := range []bool{true, false} {
			p.mask.Grayscale = grayscale
			p.renderState.Tick(p.mask)
			p.renderPixel()
		}
		assert.Equal(t, byte(value), p.palette.Read(0))
	}

	p.screen.FinishRendering()

	for value := range 64 {
		assert.Equal(t, colors[value&0x30], p.Image().RGBAAt(value*2, 0))
		assert.Equal(t, colors[value], p.Image().RGBAAt(value*2+1, 0))
	}
}

type recordNameTable struct {
	bus.NameTable
	addresses []uint16
}

func (nt *recordNameTable) Read(address uint16) byte {
	nt.addresses = append(nt.addresses, address)
	return 0
}
