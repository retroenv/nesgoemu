package nes

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestPeekReadsRAM(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Memory.Write(0x0010, 0x2a)

	value, ok := sys.Peek(0x0010)

	assert.True(t, ok)
	assert.Equal(t, byte(0x2a), value)
}

func TestPeekRefusesRegister(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)

	_, ok := sys.Peek(0x2002)

	assert.False(t, ok)
}

func TestPeekReadsPRGROM(t *testing.T) {
	cart := cartridge.New()
	cart.PRG[0] = 0xab
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	value, ok := sys.Peek(0x8000)

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

	_, offset, ok := sys.PRGLocation(0x8000)

	assert.True(t, ok)
	assert.Equal(t, 0, offset)
}
