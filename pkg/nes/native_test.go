package nes

import (
	"errors"
	"strconv"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/ppu/screen"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNativeDevices(t *testing.T) {
	n, err := NewNative(NativeOptions{
		CHR:    make([]byte, 8192),
		Mirror: cartridge.MirrorHorizontal,
	})
	assert.NoError(t, err)
	n.Write(0x4017, 0x40)
	n.Write(0x2000, 0x80)
	n.Advance(30000)
	assert.True(t, n.TakeNMI())
	assert.False(t, n.TakeNMI())
	assert.True(t, n.Frame() > 0)
	assert.Equal(t, byte(0x80), n.Read(0x2002)&0x80)
	assert.Equal(t, byte(0), n.Read(0x2002)&0x80)
	before := n.Cycles()
	for range 10 {
		n.Image()
		n.InspectOAM()
		n.InspectRAM(0)
		n.Frame()
	}
	assert.Equal(t, before, n.Cycles())
}

func TestNativeDMAAndControllers(t *testing.T) {
	n, err := NewNative(NativeOptions{CHR: make([]byte, 8192)})
	assert.NoError(t, err)
	for index := range 256 {
		n.Write(0x200+uint16(index), byte(index))
	}
	n.Write(0x4014, 2)
	n.Advance(1)
	for index, value := range n.InspectOAM() {
		assert.Equal(t, byte(index), value)
	}
	assert.True(t, n.Cycles() >= 513)
	n.SetButtons(0xa5)
	n.Write(0x4016, 1)
	n.Write(0x4016, 0)
	var buttons byte
	for bit := range 8 {
		n.Advance(8)
		buttons |= (n.Read(0x4016) & 1) << bit
	}
	assert.Equal(t, byte(0xa5), buttons)
}

func TestNativeAudio(t *testing.T) {
	n, err := NewNative(NativeOptions{CHR: make([]byte, 8192)})
	assert.NoError(t, err)
	n.Write(0x4017, 0x40)
	n.Write(0x4015, 1)
	n.Write(0x4000, 0xbf)
	n.Write(0x4002, 0xfd)
	n.Write(0x4003, 0x08)
	n.Advance(30000)
	buffer := make([]byte, 4096)
	assert.True(t, n.DrainAudio(buffer) > 0)
	var audible bool
	for _, value := range buffer {
		audible = audible || value != 0
	}
	assert.True(t, audible)
	cycles := n.Cycles()
	n.AudioCallback(buffer)
	assert.Equal(t, cycles, n.Cycles())
}

func TestNativeRejectsUnsupportedBoard(t *testing.T) {
	for _, opts := range []NativeOptions{{Mapper: 682, CHR: make([]byte, 8192)}, {CHR: make([]byte, 4096)},
		{CHR: make([]byte, 8192), PRGData: make([]byte, 32769)}} {
		_, err := NewNative(opts)
		assert.Error(t, err)
	}
}

func TestNativeMirroring(t *testing.T) {
	for _, mode := range []cartridge.MirrorMode{
		cartridge.MirrorHorizontal, cartridge.MirrorVertical,
		cartridge.MirrorSingle0, cartridge.MirrorSingle1, cartridge.Mirror4,
		cartridge.MirrorDiagonal, cartridge.MirrorLShaped,
		cartridge.Mirror3Vertical, cartridge.Mirror3Horizontal, cartridge.Mirror3Diagonal,
	} {
		t.Run(strconv.Itoa(int(mode)), func(t *testing.T) {
			device, err := NewNative(NativeOptions{
				CHR:    make([]byte, 8192),
				Mirror: mode,
			})
			assert.NoError(t, err)
			assert.Equal(t, mode, device.bus.NameTable.MirrorMode())
		})
	}
	for _, mode := range []cartridge.MirrorMode{-1, 255, cartridge.Mirror3Diagonal + 1} {
		t.Run("invalid "+strconv.Itoa(int(mode)), func(t *testing.T) {
			device, err := NewNative(NativeOptions{
				CHR:    make([]byte, 8192),
				Mirror: mode,
			})
			assert.ErrorContains(t, err, "mirroring")
			assert.Nil(t, device)
		})
	}
}

func TestNativeIRQObservation(t *testing.T) {
	device, err := NewNative(NativeOptions{
		CHR: make([]byte, 8192),
	})
	assert.NoError(t, err)
	assert.False(t, device.IRQActive())
	device.Write(0x4017, 0)
	device.Advance(40000)
	cycles := device.Cycles()
	assert.True(t, device.IRQActive())
	assert.True(t, device.IRQActive())
	assert.Equal(t, cycles, device.Cycles())
	assert.Equal(t, byte(0x40), device.Read(0x4015)&0x40)
	assert.False(t, device.IRQActive())
	device.Advance(30000)
	assert.True(t, device.IRQActive())
	device.Write(0x4017, 0x40)
	assert.False(t, device.IRQActive())
}

func TestNativeDisplayAndAudioPolicy(t *testing.T) {
	device, err := NewNative(NativeOptions{
		CHR: make([]byte, 8192),
	})
	assert.NoError(t, err)
	system := newAudioTestSystem(t, loopProgram)
	dimensions := device.Dimensions()
	assert.Equal(t, screen.Width, dimensions.Width)
	assert.Equal(t, screen.Height, dimensions.Height)
	assert.Equal(t, system.Dimensions(), dimensions)
	assert.Equal(t, system.AudioFormat(), device.AudioFormat())
	assert.NoError(t, device.AudioFormat().Validate())
	assert.False(t, device.AudioPaused())
	assert.True(t, system.AudioPaused())
}

func TestMustNewNative(t *testing.T) {
	device := MustNewNative(NativeOptions{
		CHR: make([]byte, 8192),
	})
	device.Write(0x700, 42)
	assert.Equal(t, byte(42), device.Read(0x700))
	for _, options := range []NativeOptions{
		{},
		{CHR: make([]byte, 8192), Mirror: cartridge.MirrorMode(255)},
	} {
		func() {
			defer func() {
				failure, ok := recover().(error)
				assert.True(t, ok)
				assert.ErrorContains(t, failure, "creating native NES devices")
				assert.NotNil(t, errors.Unwrap(failure))
			}()
			MustNewNative(options)
		}()
	}
}
