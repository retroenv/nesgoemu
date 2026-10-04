package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

// TestMMC3FourScreenMirroring checks that power on and register writes keep four separate nametables.
func TestMMC3FourScreenMirroring(t *testing.T) {
	cart := &cartridge.Cartridge{
		Mapper: 4,
		PRG:    make([]byte, 0x8000),
		CHR:    make([]byte, 0x2000),
		Mirror: cartridge.Mirror4,
	}
	nt := nametable.New(cart.Mirror)
	m, err := New(&bus.Bus{
		Cartridge: cart,
		NameTable: nt,
	})
	assert.NoError(t, err)
	assert.Equal(t, cartridge.Mirror4, nt.MirrorMode())
	for _, value := range []byte{0, 1} {
		m.Write(0xA000, value)
		assert.Equal(t, cartridge.Mirror4, nt.MirrorMode())
	}
	for table := range 4 {
		nt.Write(uint16(0x2000+table*0x400), byte(table+1))
	}
	for table := range 4 {
		assert.Equal(t, byte(table+1), nt.Read(uint16(0x2000+table*0x400)))
	}
}

// TestMMC3NES2CHRRAM checks allocation before bank setup and access after bank changes.
func TestMMC3NES2CHRRAM(t *testing.T) {
	for _, nonvolatile := range []bool{false, true} {
		sizes := cartridge.RAMSizes{CHRVolatile: 0x2000}
		if nonvolatile {
			sizes = cartridge.RAMSizes{CHRNonvolatile: 0x2000}
		}
		m := newTestMapper(t, &cartridge.Cartridge{
			Mapper: 4,
			PRG:    make([]byte, 0x8000),
			NES2:   &cartridge.NES2Metadata{RAMSizes: sizes},
		})
		m.Write(0x8000, 2)
		m.Write(0x8001, 7)
		m.Write(0x1000, 0x5A)
		m.Write(0x8001, 6)
		assert.Equal(t, byte(0), m.Read(0x1000))
		m.Write(0x8001, 7)
		assert.Equal(t, byte(0x5A), m.Read(0x1000))
	}
}

func TestMMC3RejectsUnsupportedLayouts(t *testing.T) {
	tests := []struct {
		name             string
		prgSize, chrSize int
		sizes            cartridge.RAMSizes
	}{
		{name: "absent PRG", chrSize: 0x2000},
		{name: "short PRG", prgSize: 0x2000, chrSize: 0x2000},
		{name: "partial PRG bank", prgSize: 0x8001, chrSize: 0x2000},
		{name: "oversized PRG", prgSize: 0x82000, chrSize: 0x2000},
		{name: "absent CHR", prgSize: 0x8000},
		{name: "short CHR", prgSize: 0x8000, chrSize: 0x400},
		{name: "partial CHR bank", prgSize: 0x8000, chrSize: 0x2001},
		{name: "oversized CHR", prgSize: 0x8000, chrSize: 0x40400},
		{name: "mixed CHR", prgSize: 0x8000, chrSize: 0x2000, sizes: cartridge.RAMSizes{CHRVolatile: 0x2000}},
		{name: "oversized PRG RAM", prgSize: 0x8000, chrSize: 0x2000, sizes: cartridge.RAMSizes{PRGVolatile: 0x4000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cart := &cartridge.Cartridge{
				Mapper: 4,
				PRG:    make([]byte, tt.prgSize),
				CHR:    make([]byte, tt.chrSize),
				NES2:   &cartridge.NES2Metadata{RAMSizes: tt.sizes},
			}
			m, err := New(&bus.Bus{
				Cartridge: cart,
				NameTable: nametable.New(cart.Mirror),
			})
			assert.Nil(t, m)
			assert.Error(t, err)
		})
	}
}

func TestMMC3RejectsUnsupportedSubmappers(t *testing.T) {
	for submapper := byte(1); submapper < 16; submapper++ {
		cart := &cartridge.Cartridge{
			Mapper: 4,
			PRG:    make([]byte, 0x8000),
			CHR:    make([]byte, 0x2000),
			NES2:   &cartridge.NES2Metadata{Submapper: submapper},
		}
		m, err := New(&bus.Bus{
			Cartridge: cart,
			NameTable: nametable.New(cart.Mirror),
		})
		assert.Nil(t, m)
		assert.ErrorContains(t, err, "submapper")
	}
}
