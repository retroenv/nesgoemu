package mapperbase

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestPPUMappingBanksAndSources(t *testing.T) {
	for _, size := range []int{0x400, 0x1000, 0x2000} {
		for _, ram := range []bool{false, true} {
			base := newPPUMappingTestBase(t, size, ram)
			base.SetChrWindow(0, 19)
			base.SetChrWindow(0x2000/size-1, -1)
			source := bus.GraphicsCHRROM
			if ram {
				source = bus.GraphicsCHRRAM
			}
			for _, address := range []uint16{0, 1, uint16(size - 1), 0x1fff} {
				bank := 19 % (0x8000 / size)
				if address >= uint16(0x2000-size) {
					bank = 0x8000/size - 1
				}
				want := bus.GraphicsMapping{
					Memory: source,
					Offset: bank*size + int(address)%size,
				}
				mapping, ok := base.PPUReadMapping(address)
				assert.True(t, ok)
				assert.Equal(t, want, mapping)
				write, writable := base.PPUWriteMapping(address)
				assert.Equal(t, ram, writable)
				if ram {
					assert.Equal(t, want, write)
					base.Write(address, 0xa5)
					assert.Equal(t, byte(0xa5), base.chrRAM[write.Offset])
				}
			}
		}
	}
}

func TestPPUMappingAbsentMemoryAndInvalidAddresses(t *testing.T) {
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	for _, initialized := range []bool{false, true} {
		if initialized {
			base.Initialize()
		}
		_, ok := base.PPUReadMapping(0)
		assert.False(t, ok)
		_, ok = base.PPUWriteMapping(0)
		assert.False(t, ok)
	}
	base = newPPUMappingTestBase(t, 0x2000, true)
	for _, address := range []uint16{0x2000, 0x3000, 0x3f00, 0xffff} {
		_, ok := base.PPUReadMapping(address)
		assert.False(t, ok)
		_, ok = base.PPUWriteMapping(address)
		assert.False(t, ok)
	}
}

func TestPPUMappingDoesNotCallHooks(t *testing.T) {
	base := newPPUMappingTestBase(t, 0x2000, true)
	called := false
	read := base.AddReadHook(0, 0xfff, func(uint16) (byte, error) {
		called = true
		return 0, nil
	})
	write := base.AddWriteHook(0, 0xfff, func(uint16, byte) error {
		called = true
		return nil
	})
	_, ok := base.PPUReadMapping(1)
	assert.False(t, ok)
	_, ok = base.PPUWriteMapping(1)
	assert.False(t, ok)
	_, ok = base.PPUReadMapping(0x1000)
	assert.True(t, ok)
	_, ok = base.PPUWriteMapping(0x1000)
	assert.True(t, ok)
	read.SetProxyOnly(true)
	write.SetProxyOnly(true)
	_, ok = base.PPUReadMapping(1)
	assert.True(t, ok)
	_, ok = base.PPUWriteMapping(1)
	assert.True(t, ok)
	assert.False(t, called)
}

func newPPUMappingTestBase(t *testing.T, size int, ram bool) *Base {
	t.Helper()
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{PRG: make([]byte, 0x8000)},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	if ram {
		base.SetChrRAM(make([]byte, 0x8000))
	} else {
		base.Cartridge().CHR = make([]byte, 0x8000)
	}
	base.SetChrWindowSize(size)
	base.Initialize()
	return base
}
