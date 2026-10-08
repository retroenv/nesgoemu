package rainbow

import "github.com/retroenv/nesgoemu/pkg/bus"

// PPUReadMapping returns the physical byte for the current graphics context.
// Flash identification data and absent memory return false.
func (m *Mapper) PPUReadMapping(address uint16) (bus.GraphicsMapping, bool) {
	if address >= patternTableEnd {
		return bus.GraphicsMapping{}, false
	}
	address = m.chrReadAddress(address)
	if m.chrSource == chrSourceNT || m.chrSource == chrSourceFPGA {
		return m.chrSourceMapping(address, 0)
	}
	if m.chrSource == chrSourceROM && m.chrFlash.ID {
		return bus.GraphicsMapping{}, false
	}
	offset := m.chrReadOffset(address)
	return m.chrSourceMapping(address, offset)
}

// PPUWriteMapping returns the physical RAM or flash target.
// Graphics read modes do not change the write mapping.
func (m *Mapper) PPUWriteMapping(address uint16) (bus.GraphicsMapping, bool) {
	if address >= patternTableEnd {
		return bus.GraphicsMapping{}, false
	}
	register, offset, size := m.chrBankMapping(address)
	physical := int(m.chrBanks[register])*size + int(offset)
	return m.chrSourceMapping(address, physical)
}

func (m *Mapper) chrSourceMapping(address uint16, offset int) (bus.GraphicsMapping, bool) {
	switch m.chrSource {
	case chrSourceNT:
		return bus.GraphicsMapping{
			Memory: bus.GraphicsCIRAM,
			Offset: int(address & ciramAddressMask),
		}, true

	case chrSourceFPGA:
		return bus.GraphicsMapping{
			Memory: bus.GraphicsFPGARAM,
			Offset: int(address & (chrWindowSize4K - 1)),
		}, true

	case chrSourceROM:
		if len(m.chrROM) > 0 {
			return bus.GraphicsMapping{
				Memory: bus.GraphicsCHRROM,
				Offset: offset % len(m.chrROM),
			}, true
		}

	case chrSourceRAM:
		if len(m.chrRAM) > 0 {
			return bus.GraphicsMapping{
				Memory: bus.GraphicsCHRRAM,
				Offset: offset % len(m.chrRAM),
			}, true
		}
	}
	return bus.GraphicsMapping{}, false
}
