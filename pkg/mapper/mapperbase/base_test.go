package mapperbase

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNameTableMemoryReturnsSystemNameTable(t *testing.T) {
	nameTable := nametable.New(cartridge.MirrorHorizontal)
	system := &bus.Bus{NameTable: nameTable}
	assert.Equal(t, nameTable, New(system).NameTableMemory())
}
