package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

type registerAddressTest struct {
	name string
	got  uint16
	want uint16
}

var registerAddressTests = [...]registerAddressTest{
	{"PRG control", regPRGControl, 0x4100},
	{"low bank upper start", regLowBankUpperStart, 0x4106},
	{"low bank upper end", regLowBankUpperEnd, 0x4107},
	{"high bank upper start", regHighBankUpperStart, 0x4108},
	{"high bank upper end", regHighBankUpperEnd, 0x410F},
	{"FPGA bank", regFPGABankSelect, 0x4115},
	{"low bank lower start", regLowBankLowerStart, 0x4116},
	{"low bank lower end", regLowBankLowerEnd, 0x4117},
	{"high bank lower start", regHighBankLowerStart, 0x4118},
	{"high bank lower end", regHighBankLowerEnd, 0x411F},
	{"CHR control", regCHRControl, 0x4120},
	{"background extension offset", regBGExtModeOffset, 0x4121},
	{"fill tile", regFillTile, 0x4124},
	{"fill attribute", regFillAttribute, 0x4125},
	{"nametable bank start", regNTBankStart, 0x4126},
	{"nametable bank end", regNTBankEnd, 0x4129},
	{"nametable control start", regNTControlStart, 0x412A},
	{"nametable control end", regNTControlEnd, 0x412D},
	{"window nametable bank", regNTWindowBank, 0x412E},
	{"window nametable control", regNTWindowControl, 0x412F},
	{"CHR bank upper start", regCHRBankUpperStart, 0x4130},
	{"CHR bank upper end", regCHRBankUpperEnd, 0x413F},
	{"CHR bank lower start", regCHRBankLowerStart, 0x4140},
	{"CHR bank lower end", regCHRBankLowerEnd, 0x414F},
	{"scanline IRQ latch", regScanIRQLatch, 0x4150},
	{"scanline IRQ control", regScanIRQControl, 0x4151},
	{"scanline IRQ acknowledge", regScanIRQAcknowledge, 0x4152},
	{"scanline IRQ offset", regScanIRQOffset, 0x4153},
	{"scanline IRQ jitter", regScanIRQJitter, 0x4154},
	{"CPU cycle parity", regCycleIRQParity, 0x4157},
	{"CPU IRQ reload upper", regCycleIRQReloadUpper, 0x4158},
	{"CPU IRQ reload lower", regCycleIRQReloadLower, 0x4159},
	{"CPU IRQ control", regCycleIRQControl, 0x415A},
	{"CPU IRQ acknowledge", regCycleIRQAcknowledge, 0x415B},
	{"FPGA auto address upper", regFPGAAutoAddressUpper, 0x415C},
	{"FPGA auto address lower", regFPGAAutoAddressLower, 0x415D},
	{"FPGA auto increment", regFPGAAutoIncrement, 0x415E},
	{"FPGA auto data", regFPGAAutoData, 0x415F},
	{"platform version", regPlatformVersion, 0x4160},
	{"IRQ status", regIRQStatus, 0x4161},
	{"vector control", regVectorControl, 0x416B},
	{"NMI vector upper", regNMIVectorUpper, 0x416C},
	{"NMI vector lower", regNMIVectorLower, 0x416D},
	{"IRQ vector upper", regIRQVectorUpper, 0x416E},
	{"IRQ vector lower", regIRQVectorLower, 0x416F},
	{"window split start", regWindowSplitStart, 0x4170},
	{"window split end", regWindowSplitEnd, 0x4175},
	{"ESP control", regESPControl, 0x4190},
	{"ESP status", regESPStatus, 0x4191},
	{"ESP start", regESPStart, 0x4192},
	{"ESP receive page", regESPReceivePage, 0x4193},
	{"ESP transmit page", regESPTransmitPage, 0x4194},
	{"sprite bank lower start", regSpriteBankLowerStart, 0x4200},
	{"sprite bank lower end", regSpriteBankLowerEnd, 0x423F},
	{"sprite bank upper", regSpriteBankUpper, 0x4240},
	{"OAM slow page", regOAMSlowPage, 0x4241},
	{"OAM extended page", regOAMExtendedPage, 0x4242},
	{"OAM limit", regOAMLimit, 0x4243},
	{"OAM slow routine", oamRoutineStart, 0x4280},
	{"OAM sprite routine", oamSpriteRoutineStart, 0x4282},
}

// TestRegisterAddressesMatchSpecification keeps the literal addresses in one
// test. Other tests use constants and do not repeat the address literals.
func TestRegisterAddressesMatchSpecification(t *testing.T) {
	for _, test := range registerAddressTests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.got)
		})
	}
}
