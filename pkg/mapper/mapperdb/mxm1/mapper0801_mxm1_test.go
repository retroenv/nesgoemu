package mxm1_test

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	prgBankSize = 0x2000
	chrBankSize = 0x0200
)

func TestMapper801PRGBanks(t *testing.T) {
	b, cart := newMapper801Bus(t)
	m := b.Mapper
	cart.PRG[0x1fc*prgBankSize+7] = 0x41
	cart.PRG[0x0fc*prgBankSize+7] = 0x42
	cart.PRG[len(cart.PRG)-4] = 0x73

	m.Write(0x5022, 0)
	m.Write(0x5010, 0xfc)
	assert.Equal(t, byte(0x42), m.Read(0x6007))
	m.Write(0x5020, 7)
	m.Write(0x5012, 2)
	assert.Equal(t, byte(0x41), m.Read(0x6007))
	assert.Equal(t, byte(0xfc), m.Read(0x5020))
	assert.Equal(t, byte(7), m.Read(0x5022))
	assert.Equal(t, byte(0x73), m.Read(0xfffc))

	m.Write(0x5022, 0x81)
	m.Write(0x5020, 0)
	m.Write(0x5010, 3)
	m.Write(0x8007, 0xa5)
	assert.Equal(t, byte(0x81), m.Read(0x501c))
	assert.Equal(t, byte(0xa5), m.Read(0x8007))
	m.Write(0x5022, 1)
	assert.Equal(t, byte(0), m.Read(0x8007))
	m.Write(0x5022, 0x81)
	assert.Equal(t, byte(0xa5), m.Read(0x8007))

	m.Write(0x5014, 1)
	m.Write(0x5017, 1)
	m.Write(0x5015, 0x80)
	assert.Equal(t, byte(0xa5), m.Read(0xc007))
	value, ok := m.(bus.CPUMemoryInspector).InspectCPU(0xc007)
	assert.True(t, ok)
	assert.Equal(t, byte(0xa5), value)

	m.(bus.MapperResetter).Reset()
	assert.Equal(t, byte(0), m.Read(0x6007))
	assert.Equal(t, byte(0x73), m.Read(0xfffc))
}

func TestMapper801CHRBanks(t *testing.T) {
	b, cart := newMapper801Bus(t)
	m := b.Mapper
	cart.CHR[0x2134*chrBankSize+7] = 0xa5
	cart.CHR[7] = 0x41

	m.Write(0x5021, 8)
	m.Write(0x5011, 0xe1)
	m.Write(0x5013, 0x34)
	m.Write(0x5023, 0x51)
	m.Write(0x5021, 0x18)
	m.Write(0x5013, 0x52)
	assert.Equal(t, byte(0xa5), m.Read(0x1007))
	assert.Equal(t, byte(0x41), m.Read(0x1207))
}

func TestMapper801RemapFetch(t *testing.T) {
	b, cart := newMapper801Bus(t)
	m := b.Mapper

	// Row $F6 wraps to row 6 and selects the lower nametable.
	m.Write(0x5050, 0x80)
	m.Write(0x5051, 2)
	m.Write(0x5053, 1)
	cart.CHR[0x400+5] = 0x02
	m.Write(0x5041, 2)
	m.Write(0x5043, 1)
	cart.CHR[0x800+10] = 0x81
	cart.CHR[0x800+11] = 0x23
	cart.CHR[0x1000+10] = 0xe4
	cart.CHR[0x1000+11] = 0x1b
	cart.CHR[0x123026] = 0x31
	cart.CHR[0x12302e] = 0x32
	cart.CHR[0x023026] = 0x33
	cart.CHR[0x02302e] = 0x34

	m.Write(0x50e0, 1)
	m.Write(0x50e6, 80)
	m.Write(0x50e3, 6)
	_ = m.Read(0x50e0)
	for range 2 * 80 {
		m.Write(0x50e1, 0)
	}
	m.Write(0x50e1, 0)
	m.Write(0x50e1, 240)

	for _, test := range []struct {
		mode, attribute, low, high byte
	}{
		{0x80, 0xaa, 0x31, 0x32},
		{0x82, 0xaa, 0x31, 0x32},
		{0x83, 0xaa, 0x33, 0x34},
	} {
		m.Write(0x5040, test.mode)
		assert.Equal(t, byte(2), b.NameTable.Read(0x2045))
		assert.Equal(t, test.attribute, b.NameTable.Read(0x23c0))
		assert.Equal(t, test.low, m.Read(0x0020))
		assert.Equal(t, test.high, m.Read(0x0028))
	}
}

func TestMapper801RejectsUnsupportedLayout(t *testing.T) {
	b, cart := newMapper801Bus(t)
	cart.NES2.Submapper = 1
	_, err := mapper.New(b)
	assert.Error(t, err)

	cart.NES2.Submapper = 0
	cart.CHR = nil
	_, err = mapper.New(b)
	assert.Error(t, err)
}

func newMapper801Bus(t *testing.T) (*bus.Bus, *cartridge.Cartridge) {
	t.Helper()
	cart := &cartridge.Cartridge{
		Mapper: 801,
		PRG:    make([]byte, 0x400000),
		CHR:    make([]byte, 0x800000),
		Mirror: cartridge.Mirror4,
		NES2: &cartridge.NES2Metadata{RAMSizes: cartridge.RAMSizes{
			PRGVolatile: 2 * prgBankSize,
		}},
	}

	b := mappertest.NewBus(cart)
	m, err := mapper.New(b)
	assert.NoError(t, err)
	b.Mapper = m
	return b, cart
}
