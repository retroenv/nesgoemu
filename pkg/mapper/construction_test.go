package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNewRejectsGTROMWithCHRROM(t *testing.T) {
	// GTROM used to accept CHR ROM and modify it through the CHR RAM write path.
	cart := &cartridge.Cartridge{
		Mapper: 111,
		PRG:    make([]byte, 0x8000),
		CHR:    make([]byte, 0x2000),
	}
	m, err := New(&bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
	})
	assert.Error(t, err)
	assert.Nil(t, m)
}
