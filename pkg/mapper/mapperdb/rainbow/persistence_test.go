package rainbow

import (
	"bytes"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestBatteryRoundTrip(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Cartridge().NES2 = &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{
		PRGVolatile: 16384, PRGNonvolatile: 16384,
		CHRVolatile: 16384, CHRNonvolatile: 16384,
	}}
	m.prgROM[0], m.chrROM[0] = 0xAB, 0xCD
	m.prgRAM[16384], m.chrRAM[16384] = 0x12, 0x34
	var saved bytes.Buffer
	assert.NoError(t, m.SaveBattery(&saved))
	m.prgROM[0], m.chrROM[0], m.prgRAM[16384], m.chrRAM[16384] = 0, 0, 0, 0
	m.prgRAM[0] = 0xEF
	assert.NoError(t, m.LoadBattery(bytes.NewReader(saved.Bytes())))
	assert.Equal(t, byte(0xAB), m.prgROM[0])
	assert.Equal(t, byte(0xCD), m.chrROM[0])
	assert.Equal(t, byte(0x12), m.prgRAM[16384])
	assert.Equal(t, byte(0x34), m.chrRAM[16384])
	assert.Equal(t, byte(0xEF), m.prgRAM[0])
	other := newTestMapper(t, 0x8000, 0x2000)
	other.Cartridge().PRG[10] = 1
	assert.Error(t, other.LoadBattery(bytes.NewReader(saved.Bytes())))
	assert.Equal(t, byte(0), other.prgROM[0])
	assert.Error(t, m.LoadBattery(bytes.NewReader(saved.Bytes()[:50])))
	assert.Equal(t, byte(0xAB), m.prgROM[0])
}
