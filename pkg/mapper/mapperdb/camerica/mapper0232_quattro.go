package camerica

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
)

const (
	quattroPageRegisterStart = 0xc000
	quattroPagesPerBlock     = 4
)

// NewQuattro creates a mapper 232 board after it checks the cartridge memory layout.
func NewQuattro(base *mapperbase.Base) (bus.Mapper, error) {
	cart := base.Cartridge()
	if err := validateBoard(cart, "mapper 232"); err != nil {
		return nil, err
	}
	if err := validateSubmapper(cart, "mapper 232"); err != nil {
		return nil, err
	}

	m := &mapper232{
		Base:    base,
		aladdin: cart.NES2 != nil && cart.NES2.Submapper == 1,
	}
	m.SetName("Camerica Quattro")
	m.SetPrgRAM(nil)
	m.SetChrRAM(make([]byte, 0x2000))
	m.SetPrgWindowSize(0x4000)
	m.DeclareFeature(feature.PRGBanking)
	m.Initialize()
	m.updateBanks()
	m.AddWriteHook(0x8000, 0xffff, m.selectPRG)

	return m, nil
}

type mapper232 struct {
	*mapperbase.Base

	block   int
	page    int
	aladdin bool
}

func (m *mapper232) selectPRG(address uint16, value byte) error {
	if address < quattroPageRegisterStart {
		m.block = int(value >> 3 & 3)
		if m.aladdin {
			// Submapper 1 is the Aladdin Deck Enhancer variant. The outer bank
			// number has its two bits swapped.
			// https://www.nesdev.org/wiki/NES_2.0_submappers#232:_Quattro
			m.block = (m.block >> 1) | (m.block << 1 & 2)
		}
	} else {
		m.page = int(value & 3)
	}

	m.updateBanks()
	m.MarkFeature(feature.PRGBanking)

	return nil
}

func (m *mapper232) updateBanks() {
	first := m.block * quattroPagesPerBlock
	m.SetPrgWindow(0, first+m.page)
	m.SetPrgWindow(1, first+quattroPagesPerBlock-1)
}
