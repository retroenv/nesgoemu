package nes

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

func TestResetUsesCartridgeVector(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	mapper := &resetVectorMapper{}
	sys.Bus.Mapper = mapper
	sys.Bus.PPU.Write(register.PPU_MASK, 0x18)
	sys.PC = 0x1234

	sys.Reset()

	assert.True(t, mapper.reset)
	assert.Equal(t, uint16(0x8000), sys.PC)
	assert.Equal(t, byte(0), sys.Bus.PPU.Read(register.PPU_MASK))
}

func TestResetWithoutMapperHook(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	sys.Bus.Mapper = plainMapper{}
	sys.PC = 0x1234

	sys.Reset()

	assert.Equal(t, uint16(0), sys.PC)
}

type resetVectorMapper struct {
	plainMapper
	reset bool
}

func (mapper *resetVectorMapper) Reset() {
	mapper.reset = true
}

func (mapper *resetVectorMapper) Read(address uint16) byte {
	if mapper.reset && address == 0xFFFD {
		return 0x80
	}
	return 0
}
