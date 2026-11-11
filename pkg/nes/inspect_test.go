package nes

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestPeekReadsRAM(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0x0010, 0x2a)

	value, ok := sys.Peek(0x0010)

	assert.True(t, ok)
	assert.Equal(t, byte(0x2a), value)

	sys.Bus.Memory.Write(0x07ff, 0x43)
	value, ok = sys.Peek(0x1fff)
	assert.True(t, ok)
	assert.Equal(t, byte(0x43), value)
}

func TestPeekReadsPRGRAMBoundaries(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = rainbowMapperID
	cart.PRG[0] = 0xab
	cart.NES2 = &cartridge.NES2Metadata{
		RAMSizes: cartridge.RAMSizes{
			PRGVolatile: mapperbase.PRGRAMEnd - mapperbase.PRGRAMStart + 1,
		},
	}
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	for _, address := range []uint16{0x6000, 0x7fff} {
		value, ok := sys.Peek(address)
		assert.True(t, ok)
		assert.Equal(t, byte(0), value)
	}

	_, ok := sys.Peek(0x5fff)
	assert.False(t, ok)
	value, ok := sys.Peek(nes.CodeBaseAddress)
	assert.True(t, ok)
	assert.Equal(t, cart.PRG[0], value)
}

func TestPeekRefusesRegister(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	_, ok := sys.Peek(register.PPU_STATUS)

	assert.False(t, ok)
}

func TestPeekReadsPRGROM(t *testing.T) {
	cart := cartridge.New()
	cart.PRG[0] = 0xab
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	value, ok := sys.Peek(nes.CodeBaseAddress)

	assert.True(t, ok)
	assert.Equal(t, byte(0xab), value)
}

func TestPeekRangeStopsAtUnreadable(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0x10, 0x01)
	sys.Bus.Memory.Write(0x11, 0x02)

	data := sys.PeekRange(0x10, 2)
	assert.Equal(t, []byte{0x01, 0x02}, data)

	// The range crosses into the register area at $2000.
	truncated := sys.PeekRange(0x1ffe, 4)
	assert.Len(t, truncated, 2)
}

func TestPRGLocation(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	_, offset, ok := sys.PRGLocation(nes.CodeBaseAddress)

	assert.True(t, ok)
	assert.Equal(t, 0, offset)
}
