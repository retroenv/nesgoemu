package mapperdb

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestMapperAxROM(t *testing.T) {
	prg := make([]byte, 0x8000*2)

	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			CHR: make([]byte, 0x2000),
			PRG: prg,
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	m, err := NewAxROM(base)
	assert.NoError(t, err)

	prg[0x0010] = 0x03 // bank 0
	prg[0x8010] = 0x04 // bank 1
	assert.Equal(t, 0x03, m.Read(0x8010))

	m.Write(0x8000, 1) // select bank 1
	assert.Equal(t, 0x04, m.Read(0x8010))
}
