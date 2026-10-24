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
	for _, opts := range []NativeOptions{{Mapper: 4095, CHR: make([]byte, 8192)},
		{CHR: make([]byte, 8192), Mirror: cartridge.MirrorMode(255)}} {
		_, err := NewNative(opts)
		assert.Error(t, err)
	}
}

func TestNativeCHRRAMAndPRGPadding(t *testing.T) {
	assert.Equal(t, 32*1024, nativePRGSize(0))
	assert.Equal(t, 32*1024, nativePRGSize(32*1024))
	assert.Equal(t, 128*1024, nativePRGSize(80*1024))

	n, err := NewNative(NativeOptions{PRGData: []byte{0x42}})
	assert.NoError(t, err)
	assert.Equal(t, byte(0x42), n.Read(0x8000))
	n.Write(0x2006, 0)
	n.Write(0x2006, 0x10)
	n.Write(0x2007, 0x99)
	n.Write(0x2006, 0)
	n.Write(0x2006, 0x10)
	_ = n.Read(0x2007)
	assert.Equal(t, byte(0x99), n.Read(0x2007))
}

func TestNativeBankedPRGData(t *testing.T) {
	// MMC3 maps 8 KiB PRG banks at $8000 through bank select and bank data.
	data := make([]byte, 128*1024)
	for bank := range 16 {
		data[bank*8192] = byte(bank)
	}

	n, err := NewNative(NativeOptions{
		Mapper:  4,
		PRGData: data,
	})
	assert.NoError(t, err)
	for _, bank := range []byte{3, 9} {
		n.Write(0x8000, 6)
		n.Write(0x8001, bank)
		assert.Equal(t, bank, n.Read(0x8000))
	}
}

func TestNativeRainbowBoard(t *testing.T) {
	chr := make([]byte, 256*1024)
	chr[0] = 0x5a
	n, err := NewNative(NativeOptions{
		Mapper:  682,
		Mirror:  cartridge.MirrorHorizontal,
		CHR:     chr,
		Battery: 1,
		NES2: &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{
			PRGVolatile:    16 * 1024,
			PRGNonvolatile: 16 * 1024,
			CHRVolatile:    32 * 1024,
		}},
	})
	assert.NoError(t, err)
	chr[0] = 0
	// Map PRG-RAM banks 0 and 1 at $8000 and $A000, and bank 2 at $6000.
	for _, write := range [][2]uint16{{0x4100, 3}, {0x4106, 0x80}, {0x4116, 2},
		{0x4108, 0x80}, {0x4118, 0}, {0x410a, 0x80}, {0x411a, 1}} {
		n.Write(write[0], byte(write[1]))
	}

	n.Write(0x8000, 0x11)
	n.Write(0xbfff, 0x22)
	n.Write(0x6000, 0x33)
	assert.Equal(t, byte(0x11), n.Read(0x8000))
	assert.Equal(t, byte(0x22), n.Read(0xbfff))
	assert.Equal(t, byte(0x33), n.Read(0x6000))
	n.Write(0x4116, 3)
	assert.Equal(t, byte(0), n.Read(0x6000))
	n.Write(0x4116, 2)
	assert.Equal(t, byte(0x33), n.Read(0x6000))

	// The options CHR slice is copied, and pattern reads go through the mapper.
	n.Write(0x2006, 0)
	n.Write(0x2006, 0)
	_ = n.Read(0x2007)
	assert.Equal(t, byte(0x5a), n.Read(0x2007))

	n.Write(0x2000, 0x80)
	n.Advance(30000)
	assert.True(t, n.TakeNMI())
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
		{Mapper: 4095},
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

func TestNativeInspectPPUAndPRGRAM(t *testing.T) {
	n, err := NewNative(NativeOptions{
		CHR:    make([]byte, 8192),
		Mirror: cartridge.MirrorHorizontal,
	})
	assert.NoError(t, err)

	// Palette and nametable bytes written through the PPU ports.
	n.Read(0x2002)
	n.Write(0x2006, 0x3f)
	n.Write(0x2006, 0x01)
	n.Write(0x2007, 0x2a)
	n.Write(0x2006, 0x20)
	n.Write(0x2006, 0x05)
	n.Write(0x2007, 0x17)

	before := n.Cycles()
	value, ok := n.InspectPPU(0x3f01)
	assert.True(t, ok)
	assert.Equal(t, byte(0x2a), value)
	value, ok = n.InspectPPU(0x2005)
	assert.True(t, ok)
	assert.Equal(t, byte(0x17), value)
	assert.Equal(t, before, n.Cycles())

	// Mapper 0 does not implement pattern and PRG-RAM inspection.
	_, ok = n.InspectPPU(0x0000)
	assert.False(t, ok)
	_, ok = n.InspectPRGRAM(0)
	assert.False(t, ok)
}

func TestNativeInspectRainbowPRGRAMAndCHR(t *testing.T) {
	n, err := NewNative(NativeOptions{
		Mapper: 682,
		Mirror: cartridge.MirrorHorizontal,
		CHR:    make([]byte, 8192),
		NES2: &cartridge.NES2Metadata{
			RAMSizes: cartridge.RAMSizes{
				PRGVolatile:    8 * 1024,
				PRGNonvolatile: 8 * 1024,
				CHRVolatile:    32 * 1024,
			},
		},
	})
	assert.NoError(t, err)

	_, ok := n.InspectPRGRAM(0)
	assert.True(t, ok)
	_, ok = n.InspectPRGRAM(16 * 1024)
	assert.False(t, ok)
	_, ok = n.InspectPPU(0x0000)
	assert.True(t, ok)
}

func TestNativeDataPortIncrementsAtPowerUp(t *testing.T) {
	n, err := NewNative(NativeOptions{
		CHR:    make([]byte, 8192),
		Mirror: cartridge.MirrorHorizontal,
	})
	assert.NoError(t, err)

	// No write to $2000 has happened: PPUCTRL is zero, so the address adds 1.
	n.Read(0x2002)
	n.Write(0x2006, 0x20)
	n.Write(0x2006, 0x00)
	for index := range 4 {
		n.Write(0x2007, byte(0x90+index))
	}

	for index := range 4 {
		value, ok := n.InspectPPU(0x2000 + uint16(index))
		assert.True(t, ok)
		assert.Equal(t, byte(0x90+index), value)
	}
}
