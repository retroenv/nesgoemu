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

func TestPrgRAMSize(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0x2000, PrgRAMSize(&cartridge.Cartridge{}),
		"a legacy file without a size gets 8 KiB")
	assert.Equal(t, 0x2000, PrgRAMSize(&cartridge.Cartridge{RAM: 2}),
		"the legacy size field is ignored")
	assert.Equal(t, 0, PrgRAMSize(&cartridge.Cartridge{NES2: &cartridge.NES2Metadata{}}),
		"a NES 2.0 size of zero means that the memory is absent")
	assert.Equal(t, 0x800, PrgRAMSize(&cartridge.Cartridge{
		NES2: &cartridge.NES2Metadata{
			RAMSizes: cartridge.RAMSizes{
				PRGVolatile:    0x400,
				PRGNonvolatile: 0x400,
			},
		},
	}))
}

func TestPrgRAMSupports64KiB(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{}})
	base.SetPrgRAM(make([]byte, 64*1024))

	base.Write(PRGRAMStart, 0x5A)
	base.Write(PRGRAMEnd, 0xA5)

	assert.Equal(t, byte(0x5A), base.Read(PRGRAMStart))
	assert.Equal(t, byte(0xA5), base.Read(PRGRAMEnd))
}

func TestReadReturnsOpenBusValue(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{OpenBus: openBusTestValue(0x5A)})

	// $4020 to $5FFF and $6000 to $7FFF without PRG RAM have no memory.
	assert.Equal(t, byte(0x5A), base.Read(0x4020))
	assert.Equal(t, byte(0x5A), base.Read(0x6000))
}

func TestReadWithoutChrMemoryReturnsAddressLowByte(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{})

	// The video memory bus is multiplexed with the low byte of the address.
	assert.Equal(t, byte(0x35), base.Read(0x0035))
}

func TestReadDuringBankSwitches(t *testing.T) {
	cart := &cartridge.Cartridge{
		CHR: make([]byte, 0x4000),
		PRG: make([]byte, 0x8000),
	}
	cart.CHR[0] = 1
	cart.CHR[0x2000] = 2
	cart.PRG[0] = 3
	cart.PRG[0x4000] = 4
	base := New(&bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
	})
	assert.NoError(t, base.Initialize())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			base.SetChrWindow(0, 1)
			base.SetPrgWindow(0, 1)
			base.SetChrWindow(0, 0)
			base.SetPrgWindow(0, 0)
		}
	}()
	for {
		select {
		case <-done:
			assert.Equal(t, byte(1), base.Read(0))
			assert.Equal(t, byte(3), base.Read(0x8000))
			return

		default:
			chr := base.Read(0)
			prg := base.Read(0x8000)
			assert.True(t, chr == 1 || chr == 2)
			assert.True(t, prg == 3 || prg == 4)
			physical, ok := base.PRGROMOffset(0x8000)
			assert.True(t, ok)
			assert.True(t, physical == 0 || physical == 0x4000)
		}
	}
}

type openBusTestValue byte

func (v openBusTestValue) OpenBus() byte {
	return byte(v)
}
