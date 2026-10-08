package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestPRGROMOffsetBankSwitching(t *testing.T) {
	tests := []struct {
		name   string
		id     uint16
		writes [][2]uint16
		want   [4]int
	}{
		{name: "NROM", id: 0, want: [4]int{0, 0x2000, 0x4000, 0x6000}},
		{name: "CNROM", id: 3, writes: [][2]uint16{{0x8000, 1}}, want: [4]int{0, 0x2000, 0x4000, 0x6000}},
		{name: "UxROM", id: 2, writes: [][2]uint16{{0x8000, 6}}, want: [4]int{0x8000, 0xa000, 0xc000, 0xe000}},
		{name: "UN1ROM", id: 94, writes: [][2]uint16{{0x8000, 8}}, want: [4]int{0x8000, 0xa000, 0xc000, 0xe000}},
		{name: "UxROM AND", id: 180, writes: [][2]uint16{{0x8000, 2}}, want: [4]int{0, 0x2000, 0x8000, 0xa000}},
		{name: "AxROM", id: 7, writes: [][2]uint16{{0x8000, 1}}, want: [4]int{0x8000, 0xa000, 0xc000, 0xe000}},
		{name: "UNROM512", id: 30, writes: [][2]uint16{{0x8000, 2}}, want: [4]int{0x8000, 0xa000, 0xc000, 0xe000}},
		{name: "GTROM", id: 111, writes: [][2]uint16{{0x5000, 1}}, want: [4]int{0x8000, 0xa000, 0xc000, 0xe000}},
		{name: "MMC1", id: 1, writes: [][2]uint16{{0xe000, 0}, {0xe000, 1}, {0xe000, 0}, {0xe000, 0}, {0xe000, 0}, {0x8000, 0}, {0x8000, 0}, {0x8000, 1}, {0x8000, 1}, {0x8000, 0}}, want: [4]int{0x8000, 0xa000, 0xc000, 0xe000}},
		{name: "MMC3 mode 0", id: 4, writes: [][2]uint16{{0x8000, 6}, {0x8001, 3}, {0x8000, 7}, {0x8001, 4}}, want: [4]int{0x6000, 0x8000, 0xc000, 0xe000}},
		{name: "MMC3 mode 1", id: 4, writes: [][2]uint16{{0x8000, 0x46}, {0x8001, 3}, {0x8000, 0x47}, {0x8001, 4}}, want: [4]int{0xc000, 0x8000, 0x6000, 0xe000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cart := &cartridge.Cartridge{
				Mapper: tt.id,
				PRG:    make([]byte, 0x10000),
				CHR:    make([]byte, 0x4000),
			}
			for i := range cart.PRG {
				cart.PRG[i] = byte(i>>8) ^ byte(i)
			}
			m := newTestMapper(t, cart)
			query, ok := m.(bus.PRGROMMapper)
			assert.True(t, ok)
			for _, write := range tt.writes {
				m.Write(write[0], byte(write[1]))
			}
			before := m.State()
			for window, start := range tt.want {
				for _, offset := range []int{0, 1, 0x1fff} {
					address := uint16(0x8000 + window*0x2000 + offset)
					physical, mapped := query.PRGROMOffset(address)
					assert.True(t, mapped)
					assert.Equal(t, start+offset, physical)
					assert.Equal(t, cart.PRG[physical], m.Read(address))
				}
			}
			for _, address := range []uint16{0, 0x6000, 0x7fff} {
				_, mapped := query.PRGROMOffset(address)
				assert.False(t, mapped)
			}
			assert.Equal(t, before, m.State())
		})
	}
}

func TestPRGROMOffsetNROMMirroring(t *testing.T) {
	m := newTestMapper(t, &cartridge.Cartridge{PRG: make([]byte, 0x4000)})
	query, ok := m.(bus.PRGROMMapper)
	assert.True(t, ok)
	for _, address := range []uint16{0x8000, 0xbfff, 0xc000, 0xffff} {
		physical, mapped := query.PRGROMOffset(address)
		assert.True(t, mapped)
		assert.Equal(t, int(address-0x8000)%0x4000, physical)
	}
}
