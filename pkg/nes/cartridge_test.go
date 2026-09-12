package nes

import (
	"bytes"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestCartridgeSystemMetadata(t *testing.T) {
	rom := make([]byte, 16+0x8000+0x2000)
	copy(rom, []byte{'N', 'E', 'S', 0x1A, 2, 1, 0, 8, 0, 0, 0x97, 0x68, 0, 0, 0, 0})
	cart, err := cartridge.LoadFile(bytes.NewReader(rom))
	assert.NoError(t, err)

	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	assert.Equal(t, cart, sys.Bus.Cartridge)
	assert.Equal(t, cartridge.RAMSizes{
		PRGVolatile:    8192,
		PRGNonvolatile: 32768,

		CHRVolatile:    16384,
		CHRNonvolatile: 4096,
	}, sys.Bus.Cartridge.NES2.RAMSizes)
	assert.Equal(t, uint16(0), sys.Bus.Mapper.State().ID)
}

func TestCartridgeOptionReplacesMetadata(t *testing.T) {
	nes2 := &cartridge.Cartridge{NES2: &cartridge.NES2Metadata{
		RAMSizes: cartridge.RAMSizes{PRGVolatile: 8192},
	}}
	legacy := cartridge.New()
	opts := NewOptions(WithCartridge(nes2), WithCartridge(legacy))
	assert.Equal(t, legacy, opts.cartridge)
	assert.Nil(t, opts.cartridge.NES2)

	opts = NewOptions(WithCartridge(legacy), WithCartridge(nes2))
	assert.Equal(t, 8192, opts.cartridge.NES2.RAMSizes.PRGVolatile)
}

func TestLegacyCartridgeSystem(t *testing.T) {
	rom := make([]byte, 16+0x8000+0x2000)
	copy(rom, []byte{'N', 'E', 'S', 0x1A, 2, 1, 1})
	rom[16], rom[16+0x8000] = 0xAA, 0xBB
	cart, err := cartridge.LoadFile(bytes.NewReader(rom))
	assert.NoError(t, err)
	assert.Equal(t, byte(0xAA), cart.PRG[0])
	assert.Equal(t, byte(0xBB), cart.CHR[0])
	assert.Equal(t, cartridge.MirrorVertical, cart.Mirror)

	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	assert.Nil(t, sys.Bus.Cartridge.NES2)
}
