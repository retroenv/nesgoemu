package unrom512

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func newTestMapper(t *testing.T, cart *cartridge.Cartridge) (bus.Mapper, *mapperbase.Base) {
	t.Helper()

	if cart.PRG == nil {
		cart.PRG = make([]byte, 0x8000*2)
	}

	base := mapperbase.New(&bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
	})
	m, err := New(base)
	assert.NoError(t, err)

	return m, base
}

func featureUsed(user feature.User, id feature.ID) bool {
	for _, usage := range user.Features() {
		if usage.ID == id {
			return usage.Used
		}
	}
	return false
}

func featureDeclared(user feature.User, id feature.ID) bool {
	for _, usage := range user.Features() {
		if usage.ID == id {
			return true
		}
	}
	return false
}

func TestMapperUNROM512(t *testing.T) {
	prg := make([]byte, 0x8000*2)
	m, base := newTestMapper(t, &cartridge.Cartridge{
		PRG:    prg,
		Mirror: cartridge.Mirror4, // The loader reports one-screen headers as Mirror4.
	})

	chr := make([]byte, 0x8000)
	base.SetChrRAM(chr)
	base.Initialize()

	chr[0x1010] = 0x03 // bank 0
	chr[0x3010] = 0x04 // bank 1
	assert.Equal(t, 0x03, m.Read(0x1010))

	prg[0x0010] = 0x03 // bank 0
	prg[0x8010] = 0x04 // bank 1
	assert.Equal(t, 0x03, m.Read(0x8010))
	assert.Equal(t, cartridge.MirrorSingle0, m.MirrorMode())

	m.Write(0x8000, 0b1010_0010) // select mirror mode 1, chr bank 1, prg bank 2
	assert.Equal(t, 0x04, m.Read(0x8010))

	assert.Equal(t, cartridge.MirrorSingle1, m.MirrorMode())
	assert.True(t, featureUsed(base, feature.SingleScreenMirroring))
	assert.False(t, featureDeclared(base, feature.Mirroring))
}

// A header with horizontal or vertical mirroring fixes the mirroring. Bit 7
// of the bank register has no effect.
func TestMapperUNROM512FixedMirroring(t *testing.T) {
	for _, mode := range []cartridge.MirrorMode{cartridge.MirrorHorizontal, cartridge.MirrorVertical} {
		m, base := newTestMapper(t, &cartridge.Cartridge{Mirror: mode})

		m.Write(0x8000, 0x80)
		assert.Equal(t, mode, m.MirrorMode())
		assert.False(t, featureDeclared(base, feature.Mirroring))
		assert.False(t, featureDeclared(base, feature.SingleScreenMirroring))
	}
}

// In submapper 3, bit 7 selects horizontal or vertical mirroring. The register
// is at $C000-$FFFF.
func TestMapperUNROM512Submapper3(t *testing.T) {
	m, base := newTestMapper(t, &cartridge.Cartridge{
		Mirror: cartridge.MirrorHorizontal,
		NES2:   &cartridge.NES2Metadata{Submapper: 3},
	})

	m.Write(0x8000, 0x80)
	assert.Equal(t, cartridge.MirrorHorizontal, m.MirrorMode())

	m.Write(0xC000, 0x80)
	assert.Equal(t, cartridge.MirrorVertical, m.MirrorMode())
	assert.True(t, featureUsed(base, feature.Mirroring))

	m.Write(0xC000, 0)
	assert.Equal(t, cartridge.MirrorHorizontal, m.MirrorMode())
}

// A board with a battery uses $8000-$BFFF for flash writes. The bank register
// is at $C000-$FFFF.
func TestMapperUNROM512FlashBoardRegisterRange(t *testing.T) {
	prg := make([]byte, 0x4000*3)
	prg[0x4010], prg[0x8010] = 0x01, 0x02
	m, _ := newTestMapper(t, &cartridge.Cartridge{
		PRG:     prg,
		Battery: 1,
	})

	m.Write(0x8000, 1)
	assert.Equal(t, 0x00, m.Read(0x8010))

	m.Write(0xC000, 2)
	assert.Equal(t, 0x02, m.Read(0x8010))
}
