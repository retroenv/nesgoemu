// Package unrom512 implements UNROM-512 cartridge boards.
package unrom512

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

/*
Boards: UNROM-512-8, UNROM-512-16, UNROM-512-32, INL-D-RAM, UNROM-512-F
PRG ROM capacity: 256K/512K
PRG ROM window: 16K + 16K fixed
CHR capacity: 32K
CHR window: 8K

Reference: https://www.nesdev.org/wiki/UNROM_512
*/

const (
	registerStart      = 0x8000
	flashRegisterStart = 0xC000 // Boards with flash use $8000-$BFFF for flash writes.
	registerEnd        = 0xFFFF

	submapperSwitchable = 3 // Bit 7 selects horizontal or vertical mirroring.
)

// New returns a new UNROM-512 mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	m := &mapperUNROM512{
		Base: base,
	}
	m.SetName("UNROM 512")
	m.DeclareFeature(feature.CHRBanking)
	m.DeclareFeature(feature.PRGBanking)
	m.SetChrRAM(make([]byte, 0x8000)) // 32K
	m.Initialize()

	cart := m.Cartridge()
	var submapper byte
	if cart.NES2 != nil {
		submapper = cart.NES2.Submapper
	}

	// The header bits for one-screen and for four-screen mirroring both set the
	// four-screen flag, and the cartridge loader reports both as Mirror4. The
	// emulator does not implement the four-screen board, which uses the last
	// 8 KiB of CHR RAM as nametables. Thus Mirror4 selects one-screen mirroring.
	mirrorMode := cart.Mirror
	switch {
	case submapper == submapperSwitchable:
		m.mirroring = mirrorSwitchable
		m.DeclareFeature(feature.Mirroring)
		if mirrorMode != cartridge.MirrorVertical {
			mirrorMode = cartridge.MirrorHorizontal
		}

	case mirrorMode == cartridge.Mirror4:
		m.mirroring = mirrorOneScreen
		m.DeclareFeature(feature.SingleScreenMirroring)
		mirrorMode = cartridge.MirrorSingle0
	}

	if err := m.SetNameTableMirrorMode(mirrorMode); err != nil {
		return nil, fmt.Errorf("setting UNROM-512 mirror mode: %w", err)
	}

	start := uint16(registerStart)
	if cart.Battery != 0 || submapper == 1 || submapper == 3 || submapper == 4 {
		start = flashRegisterStart
	}

	m.AddWriteHook(start, registerEnd, m.setBanks)

	m.SetPrgWindow(1, -1)

	return m, nil
}

type mapperUNROM512 struct {
	*mapperbase.Base

	mirroring mirrorControl
}

// mirrorControl is the effect of bank register bit 7 on the nametables.
type mirrorControl int

const (
	mirrorFixed      mirrorControl = iota // The header selects horizontal or vertical mirroring.
	mirrorOneScreen                       // Bit 7 selects the lower or upper nametable.
	mirrorSwitchable                      // Bit 7 selects horizontal or vertical mirroring.
)

// setBanks decodes the bank register NCCP PPPP: bits 4-0 select the 16 KB PRG
// ROM bank at $8000, bits 6-5 select the 8 KB CHR RAM bank, and bit 7 controls
// the nametables when the board allows it.
func (m *mapperUNROM512) setBanks(_ uint16, value uint8) error {
	m.MarkFeature(feature.PRGBanking)
	m.MarkFeature(feature.CHRBanking)

	prgBank := value & 0b0001_1111
	m.SetPrgWindow(0, int(prgBank)) // select 16 KB PRG ROM bank at $8000

	chrBank := int(value>>5) & 0b0000_0011
	m.SetChrWindow(0, chrBank)

	upper := value&0x80 != 0
	var mirrorMode cartridge.MirrorMode

	switch m.mirroring {
	case mirrorOneScreen:
		m.MarkFeature(feature.SingleScreenMirroring)
		mirrorMode = cartridge.MirrorSingle0
		if upper {
			mirrorMode = cartridge.MirrorSingle1
		}

	case mirrorSwitchable:
		m.MarkFeature(feature.Mirroring)
		mirrorMode = cartridge.MirrorHorizontal
		if upper {
			mirrorMode = cartridge.MirrorVertical
		}

	default:
		return nil
	}

	if err := m.SetNameTableMirrorMode(mirrorMode); err != nil {
		return fmt.Errorf("setting UNROM-512 mirror mode: %w", err)
	}

	return nil
}
