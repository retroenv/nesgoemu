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

func usedFeature(p *PPU, id feature.ID) bool {
	for _, usage := range p.Features() {
		if usage.ID == id {
			return usage.Used
		}
	}

	return false
}

func TestFeaturesMarkHiddenLeftColumn(t *testing.T) {
	p := newTestPPU()

	p.Write(register.PPU_MASK, mask.MASK_BG_CLIP|mask.MASK_SPR_CLIP)
	assert.False(t, usedFeature(p, feature.BackgroundLeftHidden), "a hidden layer must be visible to count")
	assert.False(t, usedFeature(p, feature.SpriteLeftHidden))

	p.Write(register.PPU_MASK, mask.MASK_BG|mask.MASK_BG_CLIP|mask.MASK_SPR|mask.MASK_SPR_CLIP)
	assert.False(t, usedFeature(p, feature.BackgroundLeftHidden), "a shown left column is not hidden")
	assert.False(t, usedFeature(p, feature.SpriteLeftHidden))

	p.Write(register.PPU_MASK, mask.MASK_BG)
	assert.True(t, usedFeature(p, feature.BackgroundLeftHidden))
	assert.False(t, usedFeature(p, feature.SpriteLeftHidden))

	p.Write(register.PPU_MASK, mask.MASK_SPR)
	assert.True(t, usedFeature(p, feature.SpriteLeftHidden))
}

// stepToVisibleLine moves the PPU to a visible line and then turns rendering
// on. The test bus cannot fetch tiles, so no step runs with rendering on.
func stepToVisibleLine(p *PPU) {
	for p.renderState.ScanLine() >= visibleScanLines || p.renderState.ScanLine() < 10 {
		p.Step(341)
	}

	p.Write(register.PPU_MASK, mask.MASK_BG)
}

func TestFeaturesMarkPolledStatusFlags(t *testing.T) {
	p := newTestPPU()
	stepToVisibleLine(p)

	p.Read(register.PPU_STATUS)
	assert.False(t, usedFeature(p, feature.Sprite0HitRead))

	p.status.SetSpriteZeroHit(true)
	assert.False(t, usedFeature(p, feature.Sprite0HitRead), "a flag that is not read is not used")
	p.Read(register.PPU_STATUS)
	assert.True(t, usedFeature(p, feature.Sprite0HitRead))
}

func TestFeaturesIgnoreStatusReadsOutsideVisibleLines(t *testing.T) {
	p := newTestPPU()
	p.status.SetSpriteZeroHit(true)

	p.Read(register.PPU_STATUS)
	assert.False(t, usedFeature(p, feature.Sprite0HitRead), "rendering is off")

	for p.renderState.ScanLine() < 245 || p.renderState.ScanLine() > 255 {
		p.Step(341)
	}

	p.Write(register.PPU_MASK, mask.MASK_BG)
	p.status.SetSpriteZeroHit(true)
	p.Read(register.PPU_STATUS)
	assert.False(t, usedFeature(p, feature.Sprite0HitRead), "a vertical blank wait loop is not polling")
}

func TestFeaturesMarkMidFrameScrollWrites(t *testing.T) {
	p := newTestPPU()

	p.Write(register.PPU_SCROLL, 0)
	p.Write(register.PPU_ADDR, 0x20)
	assert.False(t, usedFeature(p, feature.MidFrameScroll), "rendering is off")

	stepToVisibleLine(p)
	assert.True(t, p.renderState.ScanLine() < visibleScanLines)
	p.Write(register.PPU_SCROLL, 8)
	assert.True(t, usedFeature(p, feature.MidFrameScroll))
}

func TestFeaturesIgnoreScrollWritesInVerticalBlank(t *testing.T) {
	p := newTestPPU()
	for p.renderState.ScanLine() < 245 || p.renderState.ScanLine() > 255 {
		p.Step(341)
	}

	p.Write(register.PPU_MASK, mask.MASK_BG)
	p.Write(register.PPU_ADDR, 0x20)
	p.Write(register.PPU_ADDR, 0x00)
	assert.False(t, usedFeature(p, feature.MidFrameScroll))
}

func TestFeaturesMarkOAMDataPortWrites(t *testing.T) {
	p := newTestPPU()

	p.Write(register.OAM_ADDR, 0)
	assert.False(t, usedFeature(p, feature.OAMDataWrites))

	p.Write(register.OAM_DATA, 0x10)
	assert.True(t, usedFeature(p, feature.OAMDataWrites))
	assert.False(t, usedFeature(p, feature.OAMDMA))
}
