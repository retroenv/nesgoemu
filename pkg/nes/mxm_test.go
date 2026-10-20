package nes

import (
	"os"
	"testing"
	"time"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/set"
)

func TestMXMRegisterTapThroughPPU(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = 800
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	// The register tap feeds the mapper's vertical-remap index, independently
	// of debugger write observers. A mirrored $200D write must not hit the tap.
	sys.Bus.NameTable.Write(0x2100, 0x42)

	sys.Bus.Mapper.Write(0x50e0, 1)
	_ = sys.Bus.Mapper.Read(0x50e0)

	for range 17 {
		sys.Bus.Mapper.Write(0x50e1, 0)
	}
	sys.Bus.Mapper.Write(0x50e1, 64)

	sys.Bus.PPU.Write(0x2005, 17)
	sys.Bus.PPU.Write(0x200d, 23)

	assert.Equal(t, byte(0x42), sys.Bus.NameTable.Read(0x2000))
}

func TestMXMSystemBankedNametableInspection(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = 800
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	sys.Bus.Mapper.Write(0x5050, 0x80)
	sys.Bus.Mapper.Write(0x5051, 0x80)
	sys.Bus.Mapper.Write(0x5052, 0)
	sys.Bus.Mapper.Write(0x5053, 1)

	// A banked CPU RAM window uploads into the RAM backing the nametable.
	sys.Bus.Mapper.Write(0x5011, 0x80)
	sys.Bus.Mapper.Write(0x5012, 0)
	sys.Bus.Mapper.Write(0x5013, 0)
	sys.Bus.Memory.Write(0x6407, 0x71)

	value, ok := sys.InspectPPU(0x2007)
	assert.True(t, ok)
	assert.Equal(t, byte(0x71), value)
}

func TestMXMSystemBankedCPUInspection(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = 800
	cart.NES2 = &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{PRGVolatile: 0x100000}}
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	// Banked RAM can live above $8000, and $6000 need not select RAM bank zero.
	for slot := range byte(2) {
		sys.Bus.Mapper.Write(0x5011, 0x80|slot)
		sys.Bus.Mapper.Write(0x5013, slot+1)

		address := uint16(0x6007) + uint16(slot)*0x2000
		sys.Bus.Memory.Write(address, slot+0x42)

		value, ok := sys.Peek(address)
		assert.True(t, ok)
		assert.Equal(t, slot+0x42, value)
	}
}

// The game is not bundled. Set NESGOEMU_MXM_ROM to run this end-to-end check
// against the separately downloaded Former Dawn v3.2 Mesen demo.
func TestMXMFormerDawnDemo(t *testing.T) {
	path := os.Getenv("NESGOEMU_MXM_ROM")
	if path == "" {
		t.Skip("set NESGOEMU_MXM_ROM to the Former Dawn v3.2 ROM")
	}

	f, err := os.Open(path)
	assert.NoError(t, err)
	defer func() { assert.NoError(t, f.Close()) }()

	cart, err := cartridge.LoadFile(f)
	assert.NoError(t, err)

	sys, err := NewSystem(NewOptions(WithCartridge(cart), WithDisabledAudio()))
	assert.NoError(t, err)
	assert.Equal(t, uint16(800), sys.Bus.Mapper.State().ID)

	result := sys.Run(t.Context(), RunOptions{
		Frames:    480,
		MaxCycles: 16_000_000,
		Timeout:   90 * time.Second,
		Inputs: []InputEvent{{
			Frame:   180,
			Buttons: controller.Start,
		}, {Frame: 181}},
	})
	assert.Equal(t, ExitSuccess, result.Reason, result.Error)
	assert.Equal(t, uint64(480), result.CompletedFrames)
	stationary := append([]byte(nil), result.Image.Pix...)

	result = sys.Run(t.Context(), RunOptions{
		Frames:    120,
		MaxCycles: 4_000_000,
		Timeout:   30 * time.Second,
		Inputs:    []InputEvent{{Buttons: controller.Right}},
	})
	assert.Equal(t, ExitSuccess, result.Reason, result.Error)
	assert.NotEqual(t, stationary, result.Image.Pix, "movement must change the rendered game")

	colors := set.New[uint32]()
	for y := range result.Image.Bounds().Dy() {
		for x := range result.Image.Bounds().Dx() {
			c := result.Image.RGBAAt(x, y)
			colors.Add(uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B))
		}
	}

	assert.True(t, colors.Size() > 8, "the game must render more than a blank screen")
}
