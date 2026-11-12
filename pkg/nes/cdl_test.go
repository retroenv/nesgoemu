package nes

import (
	"bytes"
	"math"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/nesgoemu/pkg/ppu/control"
	"github.com/retroenv/nesgoemu/pkg/ppu/mask"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

type cdlMappingMapper struct {
	mapped bool
	bus.Mapper
	mapping bus.GraphicsMapping
}

func (m *cdlMappingMapper) PPUReadMapping(uint16) (bus.GraphicsMapping, bool) {
	return m.mapping, m.mapped
}

func (*cdlMappingMapper) PPUWriteMapping(uint16) (bus.GraphicsMapping, bool) {
	return bus.GraphicsMapping{}, false
}

func TestCDLRanges(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	r := sys.NewCDLRecorder()
	r.prg[1] = codedatalog.Code | codedatalog.Data
	r.chr[1] = codedatalog.ReadProgrammatically
	prg, err := r.PRGRange(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, []codedatalog.PrgFlag{3}, prg)
	chr, err := r.CHRRange(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, []codedatalog.ChrFlag{2}, chr)
	prg[0] = 0
	chr[0] = 0
	assert.Equal(t, codedatalog.PrgFlag(3), r.prg[1])
	assert.Equal(t, codedatalog.ChrFlag(2), r.chr[1])

	for _, offset := range []int{-1, math.MaxInt} {
		_, err = r.PRGRange(offset, 1)
		assert.Error(t, err)
		_, err = r.CHRRange(offset, 1)
		assert.Error(t, err)
	}
	_, err = r.PRGRange(len(r.prg)-1, 2)
	assert.Error(t, err)
	_, err = r.CHRRange(0, -1)
	assert.Error(t, err)
	_, err = r.PRGRange(1, math.MaxInt)
	assert.Error(t, err)
	_, err = r.CHRRange(len(r.chr), 0)
	assert.NoError(t, err)
}

func TestCDLProgramReadRequiresPhysicalCHRROM(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	r := sys.NewCDLRecorder()
	r.Start()
	defer r.Stop()
	mapper := &cdlMappingMapper{
		Mapper: sys.Bus.Mapper,
	}
	sys.Bus.Mapper = mapper
	for _, source := range []bus.GraphicsMemory{bus.GraphicsCHRRAM, bus.GraphicsFPGARAM, bus.GraphicsCHRROM} {
		mapper.mapping = bus.GraphicsMapping{
			Memory: source,
			Offset: 1,
		}
		mapper.mapped = true
		r.Reset()
		sys.Bus.PPU.Read(register.PPU_DATA)
		want := codedatalog.ChrFlag(0)
		if source == bus.GraphicsCHRROM {
			want = codedatalog.ReadProgrammatically
		}
		assert.Equal(t, want, r.chr[1])
		assert.Equal(t, codedatalog.ChrFlag(0), r.chr[0])
	}
	mapper.mapped = false
	r.Reset()
	sys.Bus.PPU.Read(register.PPU_DATA)
	assert.Equal(t, codedatalog.ChrFlag(0), r.chr[0])
	assert.Equal(t, codedatalog.ChrFlag(0), r.chr[1])
}

func TestCDLRecorderKeepsMixedCodeAndData(t *testing.T) {
	for _, codeFirst := range []bool{false, true} {
		cart := cartridge.New()
		copy(cart.PRG, []byte{0xad, 0x00, 0x90})
		cart.PRG[0x1000] = 0xea
		sys, err := NewSystem(NewOptions(WithCartridge(cart)))
		assert.NoError(t, err)
		r := sys.NewCDLRecorder()
		r.Start()
		pcs := []uint16{0x8000, 0x9000}
		if codeFirst {
			pcs[0], pcs[1] = pcs[1], pcs[0]
		}
		for _, pc := range pcs {
			sys.PC = pc
			_, err = sys.StepSystem()
			assert.NoError(t, err)
		}
		r.Stop()
		assert.Equal(t, codedatalog.Code|codedatalog.Data, r.PRG()[0x1000])
	}
}

func TestCDLRecorderSkipsDummyReads(t *testing.T) {
	cart := cartridge.New()
	copy(cart.PRG, []byte{0xbd, 0xff, 0x80})
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	sys.PC = 0x8000
	sys.X = 1
	r := sys.NewCDLRecorder()
	r.Start()
	_, err = sys.StepSystem()
	assert.NoError(t, err)
	r.Stop()

	flags := r.PRG()
	assert.Equal(t, codedatalog.Code, flags[0])
	assert.Equal(t, codedatalog.Code, flags[1])
	assert.Equal(t, codedatalog.Code, flags[2])
	assert.Equal(t, codedatalog.Data, flags[0x100])
}

func TestCDLRecorderMarksIndirectReads(t *testing.T) {
	for _, opcode := range []byte{0xa1, 0xb1} {
		cart := cartridge.New()
		copy(cart.PRG, []byte{opcode, 0x00, 0xea})
		sys, err := NewSystem(NewOptions(WithCartridge(cart)))
		assert.NoError(t, err)
		sys.PC = 0x8000
		sys.Bus.Memory.Write(1, 0x90)
		r := sys.NewCDLRecorder()
		r.Start()
		for range 2 {
			_, err = sys.StepSystem()
			assert.NoError(t, err)
		}
		r.Stop()
		flags := r.PRG()
		assert.Equal(t, codedatalog.Data|codedatalog.IndirectData, flags[0x1000])
		assert.Equal(t, codedatalog.Code, flags[2])
	}
}

func TestCDLRecorderUpdatesMappingBits(t *testing.T) {
	cart := cartridge.New()
	cart.PRG = cart.PRG[:0x4000]
	cart.PRG[0] = 0xea
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	r := sys.NewCDLRecorder()
	r.Start()
	defer r.Stop()
	for _, pc := range []uint16{0x8000, 0xc000, 0x8000} {
		sys.PC = pc
		_, err = sys.StepSystem()
		assert.NoError(t, err)
		want := codedatalog.Code
		if pc == 0xc000 {
			want |= codedatalog.RomBankMappedHigh
		}
		assert.Equal(t, want, r.PRG()[0])
	}
}

func TestCDLRecorderVectorsAndDMC(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	r := sys.NewCDLRecorder()
	r.Start()
	defer r.Stop()
	r.indirect = true
	r.observeCycle(cpu6502.BusCycle{
		Address: cpu6502.IrqAddress,
		Kind:    cpu6502.BusCycleVector,
	})
	r.observeDMCFetch(apu.DMCFetch{
		Address: 0xc000,
	})
	flags := r.PRG()
	assert.Equal(t, codedatalog.Data|codedatalog.RomBankMappedLow|codedatalog.RomBankMappedHigh, flags[0x7ffe])
	assert.Equal(t, codedatalog.Data|codedatalog.PCMAudio|codedatalog.RomBankMappedHigh, flags[0x4000])
}

func TestCDLRecorderRenderingAndProgramReads(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	r := sys.NewCDLRecorder()
	r.Start()
	device := sys.Bus.PPU
	device.Write(register.PPU_CTRL, control.CTRL_SPR_1000)
	device.Write(register.OAM_ADDR, 0)
	for _, field := range []byte{0, 2, 0, 0} {
		device.Write(register.OAM_DATA, field)
	}
	device.Read(register.PPU_DATA)
	assert.Equal(t, codedatalog.ReadProgrammatically, r.CHR()[0])
	device.Step(ppu.VisibleDots * 2)
	assert.Equal(t, codedatalog.ChrFlag(0), r.CHR()[8])
	device.Write(register.PPU_MASK, mask.MASK_BG|mask.MASK_SPR)
	device.Step(ppu.VisibleDots * 2)
	flags := r.CHR()
	assert.Equal(t, codedatalog.ReadProgrammatically|codedatalog.DrawnOnScreen, flags[0])
	assert.Equal(t, codedatalog.DrawnOnScreen, flags[8])
	assert.Equal(t, codedatalog.DrawnOnScreen, flags[0x1020])
	r.Stop()
	r.Reset()
	device.Step(ppu.VisibleDots * 2)
	assert.Equal(t, codedatalog.ChrFlag(0), r.CHR()[0])
	r.Start()
	device.Step(ppu.VisibleDots * 2)
	r.Stop()
	assert.Equal(t, codedatalog.DrawnOnScreen, r.CHR()[0])
}

func TestCDLRecorderRenderingUsesPhysicalCHRBank(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = 3
	cart.CHR = make([]byte, 0x4000)
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	r := sys.NewCDLRecorder()
	r.Start()
	defer r.Stop()
	sys.Bus.Mapper.Write(0x8000, 1)
	sys.Bus.PPU.Write(register.PPU_MASK, mask.MASK_BG)
	sys.Bus.PPU.Step(ppu.VisibleDots * 2)
	assert.Equal(t, codedatalog.ChrFlag(0), r.CHR()[0])
	assert.Equal(t, codedatalog.DrawnOnScreen, r.CHR()[0x2000])
}

func TestCDLRecorderExportClearsLegacyAnnotation(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	r := sys.NewCDLRecorder()
	r.prg[0] = codedatalog.Code | codedatalog.SubEntryPoint
	var output bytes.Buffer
	assert.NoError(t, r.Write(&output))
	assert.Equal(t, byte(0x01), output.Bytes()[0])
	assert.Equal(t, codedatalog.Code|codedatalog.SubEntryPoint, r.PRG()[0])
}

func TestCDLCoverageUsesFixedPRGSegments(t *testing.T) {
	report := coverage(make([]codedatalog.PrgFlag, 0x4001))
	assert.Len(t, report, 3)
	assert.Equal(t, 0, report[0].Segment)
	assert.Equal(t, 0x2000, report[0].Size)
	assert.Equal(t, 1, report[1].Segment)
	assert.Equal(t, 0x2000, report[1].Size)
	assert.Equal(t, 2, report[2].Segment)
	assert.Equal(t, 1, report[2].Size)
}
