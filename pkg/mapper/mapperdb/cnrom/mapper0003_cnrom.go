// Package cnrom implements CNROM cartridge boards.
package cnrom

import (
	"fmt"
	"math"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes"
)

/*
Boards: CNROM "and similar"
PRG ROM capacity: 16K or 32K
CHR capacity: 32K (2M oversize version)
CHR window: 8K
*/

// New returns a new CNROM mapper.
func New(base *mapperbase.Base) (bus.Mapper, error) {
	m := &mapperCNROM{
		Base: base,
	}
	m.SetName("CNROM")
	m.DeclareFeature(feature.CHRBanking)
	if err := m.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing mapper: %w", err)
	}

	m.AddWriteHook(nes.CodeBaseAddress, math.MaxUint16, m.setChrWindow)

	return m, nil
}

type mapperCNROM struct {
	*mapperbase.Base
}

// SnapshotFormat identifies the direct mapper state format.
func (*mapperCNROM) SnapshotFormat() string { return "cnrom-mapperCNROM-v1" }

func (m *mapperCNROM) setChrWindow(_ uint16, value uint8) error {
	m.MarkFeature(feature.CHRBanking)

	m.SetChrWindow(0, int(value)) // select 8 KB CHR ROM bank for PPU $0000-$1FFF
	return nil
}
