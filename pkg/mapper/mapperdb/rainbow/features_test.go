package rainbow

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/set"
)

func usedFeatures(m *Mapper) set.Set[feature.ID] {
	used := set.New[feature.ID]()
	for _, feature := range m.Features() {
		if feature.Used {
			used.Add(feature.ID)
		}
	}
	return used
}

func TestFeaturesStartUnused(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	features := m.Features()
	assert.NotEmpty(t, features, "mapper must declare its supported features")

	for _, feature := range features {
		assert.False(t, feature.Used, "%s must start unused", feature.Name)
	}
}

func TestMemorySizesReportsAllocatedRAM(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	prgRAM, chrRAM := m.MemorySizes()
	assert.Equal(t, 32768, prgRAM)
	assert.Equal(t, 32768, chrRAM)
}

func TestFeaturesMarkRegisterHandlers(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regWindowSplitStart, 0)
	m.Write(regScanIRQLatch, 0)
	m.Write(regESPControl, 0)
	m.Write(regAudioPulse1Control, 0)

	used := usedFeatures(m)
	assert.True(t, used.Contains(feature.WindowSplit))
	assert.True(t, used.Contains(feature.ScanlineIRQ))
	assert.True(t, used.Contains(feature.ESPMessages))
	assert.True(t, used.Contains(feature.ExpansionAudio))
	assert.False(t, used.Contains(feature.OAMRoutines), "an untouched feature must stay unused")
	assert.False(t, used.Contains(feature.FPGARAM), "an untouched feature must stay unused")
}

func TestFeaturesMarkBankRegisterWrites(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regHighBankLowerStart, 1)
	m.Write(regCHRBankLowerStart, 1)

	used := usedFeatures(m)
	assert.True(t, used.Contains(feature.PRGBanking))
	assert.True(t, used.Contains(feature.CHRBanking))
}

func TestFeaturesMarkFPGARAMAccessPaths(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Read(fpgaFixedStart)
	assert.True(t, usedFeatures(m).Contains(feature.FPGARAM))

	m = newTestMapper(t, 0x8000, 0x2000)
	m.Write(fpgaFixedStart, 1)
	assert.True(t, usedFeatures(m).Contains(feature.FPGARAM))

	m = newTestMapper(t, 0x8000, 0x2000)
	m.lowBanks[0] = uint16(prgLowBankSourceFPGA << prgLowBankSourceShift)
	m.Read(prgRAMStart)
	assert.True(t, usedFeatures(m).Contains(feature.FPGARAM))

	m = newTestMapper(t, 0x8000, 0x2000)
	m.lowBanks[0] = uint16(prgLowBankSourceFPGA << prgLowBankSourceShift)
	m.Write(prgRAMStart, 1)
	assert.True(t, usedFeatures(m).Contains(feature.FPGARAM))

	m = newTestMapper(t, 0x8000, 0x2000)
	m.chrSource = chrSourceFPGA
	m.Read(0)
	assert.True(t, usedFeatures(m).Contains(feature.FPGARAM))

	m = newTestMapper(t, 0x8000, 0x2000)
	m.chrSource = chrSourceFPGA
	m.Write(0, 1)
	assert.True(t, usedFeatures(m).Contains(feature.FPGARAM))
}

func TestFeaturesMarkPPUBusTimingOnFirstRead(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.EnableBusTiming()
	assert.False(t, usedFeatures(m).Contains(feature.PPUBusTiming))

	m.Read(0)
	assert.True(t, usedFeatures(m).Contains(feature.PPUBusTiming))
}
