package mxm

import (
	"encoding/binary"
	"slices"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

// graphicsModeFeatures are the features that markGraphicsModes marks.
var graphicsModeFeatures = []feature.ID{
	feature.BGExtendedMode,
	feature.ExtendedAttributes,
	feature.RowAttributes,
	feature.Metatiles,
	feature.GraphicsSets,
	feature.SpriteExtendedMode,
	feature.ScanlineRemap,
}

// graphicsModeTests enable each mode with register writes. Attribute modes 1
// and 2 and metatiles select a palette for each pixel row. Attribute mode 3
// gives no attributes.
var graphicsModeTests = []graphicsModeTest{
	{
		name:   "tile attributes",
		writes: []registerWrite{{regDescriptorMode, 0x80}},
		want:   []feature.ID{feature.BGExtendedMode, feature.ExtendedAttributes},
	},
	{
		name:   "fine-Y attributes",
		writes: []registerWrite{{regDescriptorMode, 0x81}},
		want:   []feature.ID{feature.BGExtendedMode, feature.ExtendedAttributes, feature.RowAttributes},
	},
	{
		name:   "packed row attributes",
		writes: []registerWrite{{regDescriptorMode, 0x82}},
		want:   []feature.ID{feature.BGExtendedMode, feature.ExtendedAttributes, feature.RowAttributes},
	},
	{
		name:   "pattern-only attributes",
		writes: []registerWrite{{regDescriptorMode, 0x83}},
		want:   []feature.ID{feature.BGExtendedMode},
	},
	{
		name:   "metatiles",
		writes: []registerWrite{{regMetatileMode, 1}},
		want: []feature.ID{
			feature.BGExtendedMode,
			feature.ExtendedAttributes,
			feature.RowAttributes,
			feature.Metatiles,
		},
	},
	{
		name:   "two graphics sets",
		writes: []registerWrite{{regPlaneMode, 1}, {regDescriptorMode, 0x80}},
		want:   []feature.ID{feature.BGExtendedMode, feature.ExtendedAttributes, feature.GraphicsSets},
	},
	{
		name:   "wide banks without extended background",
		writes: []registerWrite{{regPlaneMode, 1}},
	},
	{
		name:   "sprite plane",
		writes: []registerWrite{{regPlaneMode, 2}},
		want:   []feature.ID{feature.SpriteExtendedMode},
	},
	{
		name:   "scanline remap",
		writes: []registerWrite{{regRemapMode, 1}},
		want:   []feature.ID{feature.ScanlineRemap},
	},
	{
		name: "disabled modes",
		writes: []registerWrite{
			{regDescriptorMode, 0x01},
			{regMetatileMode, 0},
			{regRemapMode, 0},
		},
	},
}

type registerWrite struct {
	address uint16
	value   byte
}

type graphicsModeTest struct {
	name   string
	writes []registerWrite
	want   []feature.ID
}

func newTestMapper(t *testing.T) (*Mapper, *bus.Bus) {
	t.Helper()

	cart := &cartridge.Cartridge{
		Mapper: 800,
		PRG:    make([]byte, 0x400000),
		CHR:    make([]byte, 0x400000),
		Mirror: cartridge.Mirror4,
		NES2: &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{
			PRGVolatile: 0x100000,
			CHRVolatile: 0x2000,
		}},
	}

	b := &bus.Bus{
		Cartridge: cart,
		NameTable: nametable.New(cart.Mirror),
	}

	mapper, err := New(mapperbase.New(b))
	assert.NoError(t, err)

	m := mapper.(*Mapper)
	b.Mapper = m

	return m, b
}

func featureUsed(m *Mapper, id feature.ID) bool {
	for _, usage := range m.Features() {
		if usage.ID == id {
			return usage.Used
		}
	}

	return false
}

// A matching mapper number does not make the protocol of another submapper
// compatible. The constructor rejects it before it installs nametable hooks or
// memory mappings.
func TestRejectsOtherSubmapper(t *testing.T) {
	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			Mapper: 800,
			NES2:   &cartridge.NES2Metadata{Submapper: 1},
		},
	})

	mapper, err := New(base)
	assert.Nil(t, mapper)
	assert.Error(t, err)
	assert.Equal(t, "MXM submapper 1 is not supported", err.Error())
}

func selectCPU(m *Mapper, flags byte, bank uint16) {
	m.Write(regPRGSelect, flags)
	m.Write(regPRGBankHigh, byte(bank>>8))
	m.Write(regPRGBankLow, byte(bank))
}

func TestPRGBanksReadbackAndInterruptContext(t *testing.T) {
	m, _ := newTestMapper(t)

	m.prgROM[len(m.prgROM)-6] = 0x2d
	assert.Equal(t, byte(0x2d), m.Read(0xfffa))

	for slot := range byte(4) {
		bank := uint16(0x120) + uint16(slot)
		selectCPU(m, slot, bank)
		m.prgROM[int(bank)*prgBankSize+7] = 0x80 + slot

		assert.Equal(t, 0x80+slot, m.Read(0x6007+uint16(slot)*prgBankSize))
		assert.Equal(t, slot, m.Read(regPRGReadback+uint16(slot)*prgReadbackStride))
		assert.Equal(t, byte(1), m.Read(regPRGReadback+1+uint16(slot)*prgReadbackStride))
		assert.Equal(t, byte(bank), m.Read(regPRGReadback+2+uint16(slot)*prgReadbackStride))
	}

	assert.False(t, featureUsed(m, feature.AlternateBank))
	m.Write(regAlternateMode, 1)
	assert.True(t, featureUsed(m, feature.AlternateBank))
	m.Write(regAlternateRAM, 0x80)
	m.Write(regAlternateHigh, 0)
	m.Write(regAlternateLow, 3)
	m.Write(0xc007, 0x55)
	assert.Equal(t, byte(0x55), m.ram[3*prgBankSize+7])

	selectCPU(m, 3, 0x124)
	m.prgROM[0x124*prgBankSize+7] = 0x91
	assert.Equal(t, byte(0x55), m.Read(0xc007))

	m.Write(regAlternateMode, 0)
	assert.Equal(t, byte(0x91), m.Read(0xc007))
}

func TestGraphicsCPUWindowReadHideAndDeferredMapping(t *testing.T) {
	m, _ := newTestMapper(t)

	selectCPU(m, 0xc1, 4)
	m.Write(0x8023, 0x94)
	assert.Equal(t, byte(0x94), m.ram[4*prgBankSize+0x23])
	assert.False(t, featureUsed(m, feature.GraphicsReadLatch))
	assert.Equal(t, byte(0), m.Read(0x8023))
	assert.Equal(t, byte(0x94), m.Read(regGraphicsLatch))
	assert.True(t, featureUsed(m, feature.GraphicsReadLatch))

	// A change of the selection flags alone does not apply a new mapping.
	m.Write(regPRGSelect, 1)
	assert.Equal(t, byte(0x94), m.Read(0x8023))

	selectCPU(m, 0x41, 5)
	m.chrROM[5*prgBankSize+0x23] = 0x67
	assert.Equal(t, byte(0), m.Read(0x8023))
	assert.Equal(t, byte(0x67), m.Read(regGraphicsLatch))

	m.Write(0x8023, 0xff)
	assert.Equal(t, byte(0x67), m.chrROM[5*prgBankSize+0x23])
}

func TestPPUBankingAndBankedWrites(t *testing.T) {
	m, b := newTestMapper(t)

	m.Write(regCHRSelect, 0x81)
	m.Write(regCHRBankHigh, 0)
	m.Write(regCHRBankLow, 3)
	m.Write(0x201, 0x6b)
	assert.Equal(t, byte(0x6b), m.ram[3*chrBankSize+1])
	assert.Equal(t, byte(0x6b), m.Read(0x201))

	m.Write(regNametableMode, 0x80)
	m.Write(regNametableSelect, 0x82)
	m.Write(regNametableHigh, 0)
	m.Write(regNametableLow, 0x21)
	m.ram[0x8405] = 0x42
	assert.Equal(t, byte(0x42), b.NameTable.Read(0x2805))

	m.Write(regWriteBankHigh, 0)
	m.Write(regWriteBankLow, 0xa)
	assert.False(t, featureUsed(m, feature.PPUWriteBanking))
	b.NameTable.Write(0x2805, 0x86)
	assert.Equal(t, byte(0x86), m.ram[0xa2805])
	assert.True(t, featureUsed(m, feature.PPUWriteBanking))
	assert.Equal(t, byte(0x42), m.ram[0x8405])

	m.Write(regNametableMode, 0)
	b.NameTable.Write(0x2805, 0x34)
	assert.Equal(t, byte(0x34), b.NameTable.Read(0x2805))
}

func TestBlockCopySourcesAndForwardOverlap(t *testing.T) {
	m, _ := newTestMapper(t)

	// A copy outside the memory does not happen and does not mark the feature.
	m.Write(regCopyToHigh, 0xff)
	m.Write(regCopyCount, 0xff)
	assert.False(t, featureUsed(m, feature.BlockCopy))

	for source := range byte(4) {
		m.prgROM[7] = 0x11
		m.chrROM[7] = 0x22
		m.ram[7] = 0x33
		m.Write(regCopySource, source)
		m.Write(regCopyDestination, 2) // Copies always go to WorkRam.
		m.Write(regCopyFromLow, 0)
		m.Write(regCopyFromHigh, 0)
		m.Write(regCopyToLow, 1)
		m.Write(regCopyToHigh, 0)
		m.Write(regCopyCount, 1)

		assert.Equal(t, []byte{0x11, 0x33, 0x22, 0x33}[source], m.ram[prgBankSize+7])
	}

	assert.True(t, featureUsed(m, feature.BlockCopy))

	m.ram[2*prgBankSize+7] = 0xff
	m.Write(regCopySource, 1)
	m.Write(regCopyCount, 2)
	assert.Equal(t, byte(0x33), m.ram[2*prgBankSize+7])

	m.Write(regCopyToHigh, 0xff)
	m.Write(regCopyCount, 0xff) // An out-of-range copy is ignored. It does not panic.
}

func TestDescriptorSourcesAndEightScanlinePalettes(t *testing.T) {
	m, b := newTestMapper(t)

	m.Write(regDescriptorMode, 0x80)

	for _, flags := range []byte{0, 0x40, 0x80, 0xc0} {
		m.Write(regDescriptorSelect, flags)
		descriptors := m.chrROM
		patterns := m.chrROM

		if flags&0x80 != 0 {
			descriptors = m.ram
		}

		if flags&0x40 != 0 {
			patterns = m.ram
		}

		copy(descriptors[10:], []byte{0xc0, 3})
		patterns[0x3420] = 0x95
		patterns[0x3428] = 0x59

		_ = b.NameTable.Read(0x2005)
		assert.Equal(t, byte(0xff), b.NameTable.Read(0x23c0))
		assert.Equal(t, byte(0x95), m.Read(0x420))
		assert.Equal(t, byte(0x59), m.Read(0x428))
	}

	m.Write(regDescriptorSelect, 0)
	m.Write(regDescriptorMode, 0x82)
	m.Write(regManualScroll, 1)
	m.chrROM[0x80a] = 0x1b
	m.chrROM[0x80b] = 0xe4

	for fine, expected := range []byte{0, 0x55, 0xaa, 0xff, 0xff, 0xaa, 0x55, 0} {
		m.Write(regVerticalPosition, byte(fine))
		_ = b.NameTable.Read(0x2005)
		assert.Equal(t, expected, b.NameTable.Read(0x23c0))
		_ = m.Read(0x420)
		_ = m.Read(0x428)
	}
}

func TestMetatilesAndAnimationIndirection(t *testing.T) {
	m, b := newTestMapper(t)

	m.Write(regMetatileMode, 1)
	m.Write(regMetatileStride, 0)
	m.Write(regMetatileFrame, 7)
	binary.BigEndian.PutUint16(m.ram[0xb800a:], 0xc002)
	binary.BigEndian.PutUint16(m.ram[0xc002e:], 3)
	copy(m.ram[0x8000c:], []byte{0x10, 0x20, 0x30, 0x40})
	m.ram[0xb0001] = 0x0b

	for parity := range byte(2) {
		m.Write(regPlaneMode, 1)
		m.Write(regPlaneSelect, parity)

		assert.Equal(t, byte(0), b.NameTable.Read(0x2005))
		assert.Equal(t, []byte{0xaa, 0xff}[parity], b.NameTable.Read(0x23c0))
		assert.Equal(t, []byte{0x10, 0x30}[parity], m.Read(0x420))
		assert.Equal(t, []byte{0x20, 0x40}[parity], m.Read(0x428))
	}

	assert.True(t, featureUsed(m, feature.MetatileAnimation))
}

// The test checks all graphics mode features after each set of register writes.
func TestGraphicsModeFeatures(t *testing.T) {
	for _, test := range graphicsModeTests {
		t.Run(test.name, func(t *testing.T) {
			m, _ := newTestMapper(t)
			for _, write := range test.writes {
				m.Write(write.address, write.value)
			}

			for _, id := range graphicsModeFeatures {
				assert.Equal(t, slices.Contains(test.want, id), featureUsed(m, id), id)
			}
		})
	}
}

func TestVerticalRemapAndReadInhibit(t *testing.T) {
	m, b := newTestMapper(t)

	m.Write(regRemapMode, 1)
	_ = m.Read(regRemapMode)

	for i := range 1024 {
		m.Write(regRemapData, byte(i))
	}

	assert.Equal(t, uint16(1024), m.remapCursor)

	for page := range byte(4) {
		m.Write(regRemapPage, page)
		m.Write(regManualScroll, 1)
		m.Write(regVerticalPosition, 13)
		m.remapTable[int(page)*256+13] = 57
		b.NameTable.Write(0x20e5, 0x72)

		assert.Equal(t, byte(0x72), b.NameTable.Read(0x2005))
		_ = m.Read(0x420)
	}

	m.Write(regReadInhibit, 2)
	assert.Equal(t, byte(0), b.NameTable.Read(0x2005))

	m.ClockCPU(1)
	assert.Equal(t, byte(0), b.NameTable.Read(0x2005))

	m.ClockCPU(1)
	assert.Equal(t, byte(0x72), b.NameTable.Read(0x2005))
}

// irqCPU records the IRQ line. The embedded nil interface panics on all other
// CPU methods. The tests do not call them.
type irqCPU struct {
	bus.CPU
	active bool
}

var _ bus.CPU = (*irqCPU)(nil)

func (c *irqCPU) SetIRQ(active bool) { c.active = active }

func TestIRQFetchDetectorAcknowledgeAndNMIVector(t *testing.T) {
	m, b := newTestMapper(t)
	cpu := &irqCPU{}
	b.CPU = cpu

	m.Write(regIRQEnable, 1)
	m.Write(regIRQTarget, 1)

	for range 3 {
		_ = b.NameTable.Read(0x2000)
	}

	assert.Equal(t, byte(0), m.Read(regScanline))

	_ = b.NameTable.Read(0x2001)
	for range 3 {
		_ = b.NameTable.Read(0x2000)
	}

	assert.True(t, cpu.active)

	m.ClockCPU(1)
	assert.Equal(t, byte(3), m.Read(regIRQAcknowledge))
	assert.False(t, cpu.active)

	m.ClockCPU(2)
	assert.Equal(t, byte(5), m.Read(regIRQAcknowledge))

	_ = m.Read(0xfffa)
	assert.Equal(t, byte(0), m.Read(regScanline))
	assert.False(t, m.inFrame)

	// Inspector queries do not start another scanline.
	state := m.registers
	_ = m.InspectCHR(0)
	_, _ = m.InspectMemory(0x2000)
	assert.Equal(t, state, m.registers)
}

func TestPPUWriteTapAndReset(t *testing.T) {
	m, _ := newTestMapper(t)

	m.ObservePPURegisterWrite(0x2005, 17)
	assert.False(t, featureUsed(m, feature.PPURegisterCapture))
	_ = m.Read(0)
	assert.Equal(t, byte(17), m.vertical)
	assert.True(t, featureUsed(m, feature.PPURegisterCapture))

	m.ObservePPURegisterWrite(0x200d, 99)
	_ = m.Read(0)
	assert.Equal(t, byte(17), m.vertical)

	selectCPU(m, 0x81, 3)
	m.Write(0x8000, 0x42)
	m.Reset()
	assert.Equal(t, byte(0x42), m.ram[3*prgBankSize])
	assert.Equal(t, [5]uint16{0, 0, 1, 2, 0xffff}, m.prgBanks)
}

func TestSpriteSlotDummyAttributeFetch(t *testing.T) {
	m, b := newTestMapper(t)

	// The 32 visible tiles each supply a name, attribute and two patterns.
	for tile := range 32 {
		m.TickPPU(tile*8+1, 20, true)
		_ = b.NameTable.Read(0x2000 + uint16(tile))
		_ = b.NameTable.Read(0x23c0)
		_ = m.Read(0)
		_ = m.Read(8)
	}

	assert.Equal(t, uint32(32), m.nameReads)

	// nesgoemu makes two nametable reads in each sprite slot. The demo expects
	// an attribute address in the second read. It counts only one name read.
	for slot := range 8 {
		m.TickPPU(257+8*slot, 20, true)
		_ = b.NameTable.Read(0x2000)
		m.TickPPU(259+8*slot, 20, true)
		_ = b.NameTable.Read(0x2000)
		assert.Equal(t, uint16(0x23c0), m.lastAddress)
		m.TickPPU(261+8*slot, 20, true)
		_ = m.Read(0)
		_ = m.Read(8)
	}

	assert.Equal(t, uint32(40), m.nameReads)

	for tile := range 2 {
		m.TickPPU(321+8*tile, 20, true)
		_ = b.NameTable.Read(0x2000 + uint16(tile))
		_ = b.NameTable.Read(0x23c0)
		_ = m.Read(0)
		_ = m.Read(8)
	}

	assert.Equal(t, uint32(42), m.nameReads)

	m.TickPPU(259, 241, true)
	_ = b.NameTable.Read(0x2000)
	assert.Equal(t, uint16(0x2000), m.lastAddress)
}

func TestPhysicalMappingInspection(t *testing.T) {
	m, _ := newTestMapper(t)

	m.Write(regCHRSelect, 0)
	m.Write(regCHRBankLow, 7)
	mapping, ok := m.PPUReadMapping(0x21)
	assert.True(t, ok)
	assert.Equal(t, bus.GraphicsMapping{
		Memory: bus.GraphicsCHRROM,
		Offset: 7*chrBankSize + 0x21,
	}, mapping)

	m.Write(regDescriptorMode, 0x80)
	_, ok = m.PPUReadMapping(0)
	assert.False(t, ok)

	_, ok = m.PPUWriteMapping(0)
	assert.False(t, ok)
}
