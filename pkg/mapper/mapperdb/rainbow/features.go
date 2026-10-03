package rainbow

import "github.com/retroenv/nesgoemu/pkg/feature"

func (m *Mapper) declareFeatures() {
	m.DeclareFeature(feature.BGExtendedMode)
	m.DeclareFeature(feature.CHRBanking)
	m.DeclareFeature(feature.CHRSourceSelect)
	m.DeclareFeature(feature.CPUCycleIRQ)
	m.DeclareFeature(feature.ESPMessages)
	m.DeclareFeature(feature.ExpansionAudio)
	m.DeclareFeature(feature.ExtendedAttributes)
	m.DeclareFeature(feature.FlashProgramming)
	m.DeclareFeature(feature.FPGARAM)
	m.DeclareFeature(feature.LowBankMapping)
	m.DeclareFeature(feature.NameTableControl)
	m.DeclareFeature(feature.NameTableFill)
	m.DeclareFeature(feature.OAMRoutines)
	m.DeclareFeature(feature.PPUBusTiming)
	m.DeclareFeature(feature.PRGBanking)
	m.DeclareFeature(feature.PRGRAM)
	m.DeclareFeature(feature.ScanlineIRQ)
	m.DeclareFeature(feature.SpriteExtendedMode)
	m.DeclareFeature(feature.VectorRedirection)
	m.DeclareFeature(feature.WindowSplit)
}
