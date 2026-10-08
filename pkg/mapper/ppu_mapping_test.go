package mapper

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestPPUMappingThroughMapperConstruction(t *testing.T) {
	tests := []struct {
		name   string
		id     uint16
		ram    bool
		writes [][2]uint16
		banks  [8]int
	}{
		{name: "NROM", id: 0, banks: [8]int{0, 1, 2, 3, 4, 5, 6, 7}},
		{name: "CNROM", id: 3, writes: [][2]uint16{{0x8000, 1}}, banks: [8]int{8, 9, 10, 11, 12, 13, 14, 15}},
		{name: "MMC1", id: 1, writes: [][2]uint16{{0xa000, 0}, {0xa000, 1}, {0xa000, 0}, {0xa000, 0}, {0xa000, 0}}, banks: [8]int{8, 9, 10, 11, 12, 13, 14, 15}},
		{name: "MMC3", id: 4, writes: [][2]uint16{{0x8000, 0}, {0x8001, 2}, {0x8000, 1}, {0x8001, 4}, {0x8000, 2}, {0x8001, 6}, {0x8000, 3}, {0x8001, 7}, {0x8000, 4}, {0x8001, 8}, {0x8000, 5}, {0x8001, 9}}, banks: [8]int{2, 3, 4, 5, 6, 7, 8, 9}},
		{name: "MMC3 inverted", id: 4, writes: [][2]uint16{{0x8000, 0}, {0x8001, 2}, {0x8000, 1}, {0x8001, 4}, {0x8000, 2}, {0x8001, 6}, {0x8000, 3}, {0x8001, 7}, {0x8000, 4}, {0x8001, 8}, {0x8000, 5}, {0x8001, 9}, {0x8000, 0x80}}, banks: [8]int{6, 7, 8, 9, 2, 3, 4, 5}},
		{name: "UxROM RAM", id: 2, ram: true, banks: [8]int{0, 1, 2, 3, 4, 5, 6, 7}},
		{name: "UN1ROM RAM", id: 94, ram: true, banks: [8]int{0, 1, 2, 3, 4, 5, 6, 7}},
		{name: "UxROM AND RAM", id: 180, ram: true, banks: [8]int{0, 1, 2, 3, 4, 5, 6, 7}},
		{name: "AxROM RAM", id: 7, ram: true, banks: [8]int{0, 1, 2, 3, 4, 5, 6, 7}},
		{name: "UNROM512 RAM", id: 30, ram: true, writes: [][2]uint16{{0x8000, 0x40}}, banks: [8]int{16, 17, 18, 19, 20, 21, 22, 23}},
		{name: "GTROM RAM", id: 111, ram: true, writes: [][2]uint16{{0x5000, 0x10}}, banks: [8]int{8, 9, 10, 11, 12, 13, 14, 15}},
		{name: "Rainbow", id: 682, banks: [8]int{0, 1, 2, 3, 4, 5, 6, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cart := &cartridge.Cartridge{
				Mapper: tt.id,
				PRG:    make([]byte, 0x10000),
			}
			source := bus.GraphicsCHRRAM
			if !tt.ram {
				cart.CHR = make([]byte, 0x8000)
				source = bus.GraphicsCHRROM
			}
			m := newTestMapper(t, cart)
			query, ok := m.(bus.PPUMappingInspector)
			assert.True(t, ok)
			for _, write := range tt.writes {
				m.Write(write[0], byte(write[1]))
			}
			for window, bank := range tt.banks {
				for _, offset := range []int{0, 1, 0x3ff} {
					address := uint16(window*0x400 + offset)
					want := bus.GraphicsMapping{
						Memory: source,
						Offset: bank*0x400 + offset,
					}
					read, mapped := query.PPUReadMapping(address)
					assert.True(t, mapped)
					assert.Equal(t, want, read)
					write, writable := query.PPUWriteMapping(address)
					assert.Equal(t, tt.ram || tt.id == 682, writable)
					if tt.ram {
						assert.Equal(t, want, write)
						m.Write(address, 0xa5)
						assert.Equal(t, byte(0xa5), m.Read(address))
					}
				}
			}
		})
	}
}
