package mapperbase

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	memoryTestPRGROMSize      = 0x8000
	memoryTestExplicitRAMSize = 0x4000
	memoryTestPRGRAMAddress   = 0x6000
	memoryTestPRGRAMValue     = 0x5a
	memoryTestCHRRAMValue     = 0xa5
)

func TestInitializeAllocatesDefaultsOnce(t *testing.T) {
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: make([]byte, memoryTestPRGROMSize),
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	prg, chr := base.MemorySizes()
	assert.Equal(t, 0, prg)
	assert.Equal(t, 0, chr)
	assert.NoError(t, base.Initialize())
	prg, chr = base.MemorySizes()
	assert.Equal(t, 0x2000, prg)
	assert.Equal(t, 0x2000, chr)
	base.Write(memoryTestPRGRAMAddress, memoryTestPRGRAMValue)
	base.Write(0, memoryTestCHRRAMValue)
	assert.NoError(t, base.Initialize())
	assert.Equal(t, byte(memoryTestPRGRAMValue), base.Read(memoryTestPRGRAMAddress))
	assert.Equal(t, byte(memoryTestCHRRAMValue), base.Read(0))
}

func TestInitializePreservesExplicitRAM(t *testing.T) {
	for _, size := range []int{0, memoryTestExplicitRAMSize} {
		base := New(&bus.Bus{
			Cartridge: &cartridge.Cartridge{
				PRG: make([]byte, memoryTestPRGROMSize),
			},
			NameTable: nametable.New(cartridge.MirrorHorizontal),
		})
		var prg, chr []byte
		if size > 0 {
			prg = make([]byte, size)
			chr = make([]byte, size)
		}
		base.SetPrgRAM(prg)
		base.SetChrRAM(chr)
		assert.NoError(t, base.Initialize())
		prgSize, chrSize := base.MemorySizes()
		assert.Equal(t, size, prgSize)
		assert.Equal(t, size, chrSize)
		if size > 0 {
			base.Write(memoryTestPRGRAMAddress, memoryTestPRGRAMValue)
			base.Write(0, memoryTestCHRRAMValue)
			assert.Equal(t, byte(memoryTestPRGRAMValue), prg[0])
			assert.Equal(t, byte(memoryTestCHRRAMValue), chr[0])
		}
	}
}

func TestInitializeRejectsInvalidBanksBeforeRAMAllocation(t *testing.T) {
	for _, sizes := range [][2]int{{0, 0x2000}, {0x2000, 0}, {0x8001, 0}, {0x8000, 0x2001}} {
		base := New(&bus.Bus{
			Cartridge: &cartridge.Cartridge{
				PRG: make([]byte, sizes[0]),
				CHR: make([]byte, sizes[1]),
			},
			NameTable: nametable.New(cartridge.MirrorHorizontal),
		})
		assert.Error(t, base.Initialize())
		prg, chr := base.MemorySizes()
		assert.Equal(t, 0, prg)
		assert.Equal(t, 0, chr)
	}
}

func TestInitializeRejectsInvalidWindows(t *testing.T) {
	for _, windows := range [][2]int{{0, 0x2000}, {0x3000, 0x2000}, {0x4000, 0}, {0x4000, 0x3000}} {
		base := New(&bus.Bus{
			Cartridge: &cartridge.Cartridge{
				PRG: make([]byte, memoryTestPRGROMSize),
			},
			NameTable: nametable.New(cartridge.MirrorHorizontal),
		})
		base.SetPrgWindowSize(windows[0])
		base.SetChrWindowSize(windows[1])
		assert.Error(t, base.Initialize())
	}
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: make([]byte, memoryTestPRGROMSize),
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	base.SetNameTableCount(0)
	assert.Error(t, base.Initialize())
}
