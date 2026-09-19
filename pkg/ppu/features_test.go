package ppu

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/ppu/control"
	"github.com/retroenv/nesgoemu/pkg/ppu/mask"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/set"
)

func newTestPPU() *PPU {
	system := &bus.Bus{Cartridge: cartridge.New()}
	system.Mapper = mapper.NewMockMapper(system)
	return New(system)
}

func TestFeaturesStartUnused(t *testing.T) {
	p := newTestPPU()

	for _, feature := range p.Features() {
		assert.False(t, feature.Used, "%s must be unused before a write", feature.Name)
	}
}

func TestFeaturesMarkControlAndMaskBits(t *testing.T) {
	p := newTestPPU()

	controlValue := control.CTRL_NMI | control.CTRL_8x16 | control.CTRL_BG_1000 |
		control.CTRL_SPR_1000 | control.CTRL_INC_32
	p.Write(register.PPU_CTRL, byte(controlValue))
	p.Write(register.PPU_MASK, mask.MASK_BG|mask.MASK_SPR|mask.MASK_MONO|mask.MASK_TINT_RED)

	used := set.New[feature.ID]()
	for _, feature := range p.Features() {
		if feature.Used {
			used.Add(feature.ID)
		}
	}

	for _, id := range []feature.ID{
		feature.NMI, feature.BackgroundTableHigh, feature.SpriteTableHigh, feature.SpriteSize8x16,
		feature.VRAMIncrement32, feature.BackgroundRendering, feature.SpriteRendering, feature.Grayscale,
		feature.ColorEmphasis,
	} {
		assert.True(t, used.Contains(id), "feature %d must be marked used", id)
	}
	assert.False(t, used.Contains(feature.OAMDMA), "OAM DMA must stay unused")
}
