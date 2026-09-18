package mapperbase

import (
	"bytes"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestBatteryRoundTripPreservesVolatilePrgRAM(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{
		NES2: &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{
			PRGVolatile:    0x400,
			PRGNonvolatile: 0x400,
		}},
	}})
	base.SetPrgRAM(make([]byte, 0x800))
	base.prgRAM[0] = 0x11
	base.prgRAM[0x400] = 0x22

	var saved bytes.Buffer
	assert.True(t, base.BatteryBacked())
	assert.NoError(t, base.SaveBattery(&saved))
	assert.Len(t, saved.Bytes(), 0x400)
	assert.Equal(t, byte(0x22), saved.Bytes()[0])
	assert.Equal(t, bytes.Repeat([]byte{0}, 0x3FF), saved.Bytes()[1:])

	base.prgRAM[0] = 0x33
	base.prgRAM[0x400] = 0x44
	assert.NoError(t, base.LoadBattery(bytes.NewReader(saved.Bytes())))

	assert.Equal(t, byte(0x33), base.prgRAM[0])
	assert.Equal(t, byte(0x22), base.prgRAM[0x400])
}

func TestBatteryLoadRejectsWrongSize(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{Battery: 1}})
	base.SetPrgRAM(make([]byte, 0x2000))

	assert.Error(t, base.LoadBattery(bytes.NewReader(make([]byte, 0x1FFF))))
	assert.Error(t, base.LoadBattery(bytes.NewReader(make([]byte, 0x2001))))
}

func TestLegacyBatteryRoundTrip(t *testing.T) {
	t.Parallel()

	base := New(&bus.Bus{Cartridge: &cartridge.Cartridge{Battery: 1}})
	base.SetPrgRAM(make([]byte, 0x2000))
	base.prgRAM[0x123] = 0x5A

	var saved bytes.Buffer
	assert.True(t, base.BatteryBacked())
	assert.NoError(t, base.SaveBattery(&saved))

	base.prgRAM[0x123] = 0
	assert.NoError(t, base.LoadBattery(bytes.NewReader(saved.Bytes())))
	assert.Equal(t, byte(0x5A), base.prgRAM[0x123])
}
