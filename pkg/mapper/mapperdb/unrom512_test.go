package mapperdb

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestMapperUNROM512(t *testing.T) {
	prg := make([]byte, 0x8000*2)

	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: prg,
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	m, err := NewUNROM512(base)
	assert.NoError(t, err)

	chr := make([]byte, 0x8000)
	base.SetChrRAM(chr)
	base.Initialize()

	chr[0x1010] = 0x03 // bank 0
	chr[0x3010] = 0x04 // bank 1
	assert.Equal(t, 0x03, m.Read(0x1010))

	prg[0x0010] = 0x03 // bank 0
	prg[0x8010] = 0x04 // bank 1
	assert.Equal(t, 0x03, m.Read(0x8010))

	m.Write(0x8000, 0b1010_0010) // select mirror mode 1, chr bank 1, prg bank 2
	assert.Equal(t, 0x04, m.Read(0x8010))

	assert.Equal(t, cartridge.MirrorSingle1, m.MirrorMode())
}
