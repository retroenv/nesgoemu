package rainbow

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestExplicitRAMSizes(t *testing.T) {
	for _, size := range []int{0, 128, 8192, 65536, 524288} {
		cart := cartridge.New()
		cart.NES2 = &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{
			PRGVolatile: size,
			CHRVolatile: size,
		}}
		system := &bus.Bus{
			Cartridge: cart,
			NameTable: nametable.New(cartridge.MirrorHorizontal),
		}
		instance, err := New(mapperbase.New(system))
		assert.NoError(t, err)
		m := instance.(*Mapper)
		assert.Len(t, m.prgRAM, size)
		assert.Len(t, m.chrRAM, size)
		m.Write(regHighBankUpperStart, 0x80)
		m.Write(0x8000, 0xAB)
		m.Write(regCHRControl, 0x40)
		m.Write(0, 0xCD)
		if size == 0 {
			assert.Equal(t, byte(0), m.Read(0x8000))
			assert.Equal(t, byte(0), m.Read(0))
		} else {
			assert.Equal(t, byte(0xAB), m.Read(0x8000))
			assert.Equal(t, byte(0xCD), m.Read(0))
			m.Write(regNTControlStart, 0x40)
			m.Write(regNTBankStart, 255)
			m.NameTableMemory().Write(0x2012, 0xEF)
			assert.Equal(t, byte(0xEF), m.NameTableMemory().Read(0x2012))
		}
	}
}
