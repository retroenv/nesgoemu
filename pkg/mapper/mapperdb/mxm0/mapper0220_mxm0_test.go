package mxm0

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	testNameTableBase = 0x2000
	testAttributeBase = 0x23c0
)

// syntheticCHR returns CHR ROM data in which all byte values occur.
func syntheticCHR() []byte {
	data := make([]byte, chrSize)
	for i := range data {
		data[i] = byte(i*37 + (i>>8)*11 + i>>3)
	}

	return data
}

func newMapper(t *testing.T, chr []byte) *Mapper {
	t.Helper()

	return newMapperWithMirror(t, chr, cartridge.MirrorHorizontal)
}

func newMapperWithMirror(t *testing.T, chr []byte, mirror cartridge.MirrorMode) *Mapper {
	t.Helper()

	cart := &cartridge.Cartridge{
		Mapper: 220,
		PRG:    make([]byte, prgSize),
		CHR:    chr,
		Mirror: mirror,
	}

	mapper, err := New(mapperbase.New(mappertest.NewBus(cart)))
	assert.NoError(t, err)

	return mapper.(*Mapper)
}

// The logic connects PPU A10 to CIRAM A10. The game header sets four-screen mirroring.
func TestMirroringIsVerticalForAllHeaders(t *testing.T) {
	headers := []cartridge.MirrorMode{cartridge.MirrorHorizontal, cartridge.MirrorVertical, cartridge.Mirror4}
	for _, header := range headers {
		m := newMapperWithMirror(t, make([]byte, chrSize), header)
		assert.Equal(t, cartridge.MirrorVertical, m.MirrorMode())
	}
}

func usedFeature(m *Mapper, id feature.ID) bool {
	for _, usage := range m.Features() {
		if usage.ID == id {
			return usage.Used
		}
	}

	return false
}

func TestCheckMarksFeature(t *testing.T) {
	m := newMapper(t, syntheticCHR())
	assert.False(t, usedFeature(m, feature.CRCCheck))

	m.Write(regCheck, 0)
	assert.False(t, usedFeature(m, feature.CRCCheck))

	m.Write(regCheck, 1)
	assert.True(t, usedFeature(m, feature.CRCCheck))
}

func TestPowerOnDimMode(t *testing.T) {
	const dimColor = 0x1c
	m := newMapper(t, syntheticCHR())
	assert.Equal(t, byte(dimColor), m.Read(paletteRegFirst+dimEntryIndex))
}

func TestRegisterDecoding(t *testing.T) {
	m := newMapper(t, syntheticCHR())
	assert.Equal(t, mappertest.OpenBusValue, m.Read(paletteRegFirst-1))
	assert.Equal(t, mappertest.OpenBusValue, m.Read(paletteRegLast+1))

	// Only bit 0 of a write matters, and only these three addresses decode.
	m.Write(regDimMode+1, 0)
	m.Write(regDimMode+2, 0)
	m.Write(regDimMode+5, 0)
	assert.True(t, m.dimMode)
	m.Write(regPhase, 0b1111_1110)
	assert.Equal(t, 0, m.blendPhase)
	m.Write(regPhase, 1)
	assert.Equal(t, 1, m.blendPhase)
}

func TestInspectionHasNoEffects(t *testing.T) {
	const tileOffset = 0x0123
	chr := syntheticCHR()
	m := newMapper(t, chr)

	_, handled := m.readNameTable(testNameTableBase + tileOffset)
	assert.False(t, handled)
	_, handled = m.readNameTable(testAttributeBase)
	assert.True(t, handled)

	bank := m.patternBank
	count := m.fetchCount
	offset := int(bank)<<patternBankShift | tileOffset

	assert.Equal(t, chr[offset], m.InspectCHR(1<<patternBankShift|tileOffset))
	mapping, ok := m.PPUReadMapping(tileOffset)
	assert.True(t, ok)
	assert.Equal(t, offset, mapping.Offset)

	_, ok = m.InspectMemory(testAttributeBase + 1)
	assert.True(t, ok)
	_, ok = m.InspectMemory(testNameTableBase)
	assert.False(t, ok)
	_, ok = m.PPUWriteMapping(0)
	assert.False(t, ok)
	_, ok = m.InspectPRGRAM(0)
	assert.False(t, ok)

	assert.Equal(t, bank, m.patternBank)
	assert.Equal(t, count, m.fetchCount)
}

func TestAttributeWritesAreDiscarded(t *testing.T) {
	m := newMapper(t, syntheticCHR())
	assert.True(t, m.writeNameTable(testAttributeBase, 1))
	assert.False(t, m.writeNameTable(testNameTableBase, 1))
}

func TestFeatures(t *testing.T) {
	m := newMapper(t, syntheticCHR())
	assert.False(t, usedFeature(m, feature.RowAttributes))

	_, _ = m.readNameTable(testAttributeBase)
	assert.True(t, usedFeature(m, feature.RowAttributes))
}
