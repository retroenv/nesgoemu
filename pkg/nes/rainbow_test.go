package nes

import (
	"bytes"
	"context"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	rainbowMapperID             = 682
	rainbowProgramStart         = 0x8000
	rainbowResetVectorOffset    = 0x7FFC
	rainbowRegScanIRQLatch      = 0x4150
	rainbowRegScanIRQControl    = 0x4151
	rainbowRegCycleIRQReloadLow = 0x4159
	rainbowRegCycleIRQControl   = 0x415A
	rainbowRegCycleIRQAck       = 0x415B
	rainbowRegIRQStatus         = 0x4161
	rainbowRegVectorControl     = 0x416B
	rainbowRegIRQVectorUpper    = 0x416E
	rainbowRegIRQVectorLower    = 0x416F
	rainbowRegCHRControl        = 0x4120
	rainbowRegNametableControl0 = 0x412a
	ppuMaskRegister             = 0x2001
	ppuScrollRegister           = 0x2005
)

func TestRainbowNES2System(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = rainbowMapperID
	cart.PRG = make([]byte, 0x8000)
	copy(cart.PRG, []byte{0xEA, 0xEA, 0xEA})
	cart.PRG[rainbowResetVectorOffset] = 0
	cart.PRG[rainbowResetVectorOffset+1] = byte(rainbowProgramStart >> 8)
	var rom bytes.Buffer
	assert.NoError(t, cart.Save(&rom))
	loaded, err := cartridge.LoadFile(&rom)
	assert.NoError(t, err)
	sys, err := NewSystem(&Options{cartridge: loaded})
	assert.NoError(t, err)
	assert.Equal(t, uint16(rainbowMapperID), sys.Bus.Mapper.State().ID)
	assert.Equal(t, "Rainbow", sys.Bus.Mapper.State().Name)
	assert.Equal(t, uint16(rainbowProgramStart), sys.PC)
	sys.Bus.Memory.Write(rainbowRegCycleIRQReloadLow, 5)
	sys.Bus.Memory.Write(rainbowRegCycleIRQControl, 1)
	assert.NoError(t, sys.runEmulatorSteps(context.Background(), rainbowProgramStart+3))
	assert.Equal(t, byte(0x40), sys.Bus.Memory.Read(rainbowRegIRQStatus))
	sys.Bus.Memory.Write(rainbowRegCycleIRQAck, 0)
	assert.Equal(t, byte(0), sys.Bus.Memory.Read(rainbowRegIRQStatus))
}

func TestRainbowBusTimingAndIRQDelivery(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = rainbowMapperID
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	sys.Bus.APU.Write(0x4017, 0x40)
	mapper := sys.Bus.Mapper
	mapper.Write(rainbowRegScanIRQLatch, 5)
	mapper.Write(rainbowRegScanIRQControl, 0)
	sys.Bus.PPU.Write(ppuMaskRegister, 8)
	sys.clockComponents(30000)
	assert.Equal(t, byte(0x80), mapper.Read(rainbowRegIRQStatus)&0x80)
	mapper.Read(rainbowRegScanIRQControl)
	assert.False(t, sys.CPU.State().Interrupts.IrqTriggered)
	sys.Bus.PPU.Write(ppuMaskRegister, 0)
	sys.clockComponents(3)
	assert.Equal(t, byte(0), mapper.Read(rainbowRegScanIRQControl)&0x40)
	mapper.Write(rainbowRegVectorControl, 2)
	mapper.Write(rainbowRegIRQVectorUpper, 0x92)
	mapper.Write(rainbowRegIRQVectorLower, 0x34)
	mapper.Write(rainbowRegCycleIRQReloadLow, 2)
	mapper.Write(rainbowRegCycleIRQControl, 1)
	sys.clockComponents(2)
	sys.Flags.I = 0
	sys.PC = 0
	sys.Bus.Memory.Write(0, 0xea)
	assert.NoError(t, sys.CPU.Step())
	assert.True(t, sys.CheckInterrupts())
	assert.Equal(t, uint16(0x9234), sys.PC)
	mapper.Write(rainbowRegCycleIRQAck, 0)
	sys.Flags.I = 0
	assert.False(t, sys.CheckInterrupts())
}

func TestRainbowExtendedBackgroundPrefetchAtLeftEdge(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = rainbowMapperID
	for row := range 8 {
		cart.CHR[row] = 0xff
	}
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	for offset := range uint16(960) {
		sys.Bus.NameTable.Write(0x2000+offset, 1)
	}
	sys.Bus.NameTable.Write(0x20c0, 0)
	sys.Bus.Mapper.Write(rainbowRegCHRControl, 0)
	sys.Bus.Mapper.Write(rainbowRegNametableControl0, 3)
	sys.Bus.Mapper.Write(0x5000, 0)
	sys.Bus.PPU.Palette().Write(0, 0x0f)
	sys.Bus.PPU.Palette().Write(1, 0x30)
	sys.Bus.PPU.Write(ppuScrollRegister, 0)
	sys.Bus.PPU.Write(ppuScrollRegister, 0)
	sys.Bus.PPU.Write(ppuMaskRegister, 0x0a)
	sys.clockComponents(60_000)

	assert.Equal(t, byte(0xff), sys.Image().RGBAAt(0, 48).R)
	assert.Equal(t, byte(0), sys.Image().RGBAAt(8, 48).R)
}
