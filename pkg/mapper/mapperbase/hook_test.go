package mapperbase

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

// A proxy-only read hook observes the access and keeps the memory value.
// The regression: AddReadHook returned a pointer to a copy of the hook, so
// SetProxyOnly did not change the registered hook and the hook replaced the value.
func TestReadHookProxyOnlyKeepsMemoryValue(t *testing.T) {
	t.Parallel()

	base, chr := newHookTestBase(t)
	chr[0] = 0x42

	calls := 0
	base.AddReadHook(0x0000, 0x1FFF, func(uint16) (uint8, error) {
		calls++
		return 0xEE, nil
	}).SetProxyOnly(true)

	assert.Equal(t, byte(0x42), base.Read(0x0000))
	assert.Equal(t, 1, calls)
}

// A read hook without SetProxyOnly replaces the memory value.
func TestReadHookReplacesMemoryValue(t *testing.T) {
	t.Parallel()

	base, chr := newHookTestBase(t)
	chr[0] = 0x42

	base.AddReadHook(0x0000, 0x1FFF, func(uint16) (uint8, error) {
		return 0xEE, nil
	})

	assert.Equal(t, byte(0xEE), base.Read(0x0000))
}

// A proxy-only write hook observes the access and keeps the memory write.
func TestWriteHookProxyOnlyKeepsMemoryWrite(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{}})
	base.SetPrgRAM(make([]byte, 0x2000))

	calls := 0
	base.AddWriteHook(prgRAMStart, prgRAMEnd, func(uint16, uint8) error {
		calls++
		return nil
	}).SetProxyOnly(true)

	base.Write(prgRAMStart, 0x5A)
	assert.Equal(t, byte(0x5A), base.Read(prgRAMStart))
	assert.Equal(t, 1, calls)
}

// A write hook without SetProxyOnly stops the memory write.
func TestWriteHookBlocksMemoryWrite(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{}})
	base.SetPrgRAM(make([]byte, 0x2000))

	base.AddWriteHook(prgRAMStart, prgRAMEnd, func(uint16, uint8) error {
		return nil
	})

	base.Write(prgRAMStart, 0x5A)
	assert.Equal(t, byte(0x00), base.Read(prgRAMStart))
}

func newHookTestBase(t *testing.T) (*Base, []byte) {
	t.Helper()

	chr := make([]byte, 0x2000)
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			CHR: chr,
			PRG: make([]byte, 0x4000),
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	base.Initialize()

	return base, chr
}
