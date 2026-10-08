package mapperbase

import "github.com/retroenv/nesgoemu/pkg/bus"

// PPUReadMapping returns the physical CHR byte at a pattern-table address.
// The query does not call read hooks or change state.
func (b *Base) PPUReadMapping(address uint16) (bus.GraphicsMapping, bool) {
	for _, hook := range b.readHooks {
		if address >= hook.startAddress && address <= hook.endAddress && !hook.onlyProxy {
			return bus.GraphicsMapping{}, false
		}
	}
	return b.chrMapping(address, false)
}

// PPUWriteMapping returns the physical CHR RAM byte at a pattern-table address.
// Writes without CHR RAM and writes that a hook handles return false.
func (b *Base) PPUWriteMapping(address uint16) (bus.GraphicsMapping, bool) {
	for _, hook := range b.writeHooks {
		if address >= hook.startAddress && address <= hook.endAddress && !hook.onlyProxy {
			return bus.GraphicsMapping{}, false
		}
	}
	return b.chrMapping(address, true)
}

func (b *Base) chrMapping(address uint16, write bool) (bus.GraphicsMapping, bool) {
	if address >= chrMemSize {
		return bus.GraphicsMapping{}, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.chrBanks) == 0 || b.chrBankMapper == nil || (write && len(b.chrRAM) == 0) {
		return bus.GraphicsMapping{}, false
	}
	source := bus.GraphicsCHRROM
	if len(b.bus.Cartridge.CHR) == 0 {
		source = bus.GraphicsCHRRAM
	}
	bank, offset := b.chrBankMapper(address)
	return bus.GraphicsMapping{
		Memory: source,
		Offset: bank*b.chrWindowSize + int(offset),
	}, true
}
