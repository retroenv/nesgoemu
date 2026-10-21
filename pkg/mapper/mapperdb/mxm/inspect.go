package mxm

import "github.com/retroenv/nesgoemu/pkg/bus"

// InspectPRGRAM reads physical RAM. It does not change the mapper state.
func (m *Mapper) InspectPRGRAM(offset int) (byte, bool) {
	if offset < 0 || offset >= len(m.ram) {
		return 0, false
	}
	return m.ram[offset], true
}

// InspectCHR reads the ordinary mapping. It does not change the fetch or IRQ state.
func (m *Mapper) InspectCHR(address uint16) byte {
	return m.mappedVRAM(address & (patternTableEnd - 1))
}

// PRGROMOffset returns the ROM offset of the effective CPU mapping, including
// the interrupt bank.
func (m *Mapper) PRGROMOffset(address uint16) (int, bool) {
	if address < cpuWindowBase {
		return 0, false
	}

	slot := int(address-cpuWindowBase) / prgBankSize
	w := m.cpuWindows[slot]
	if w.writable || m.prgFlags[slot]&prgGraphicsLatch != 0 || !w.prg {
		return 0, false
	}
	return (w.offset + int(address&prgOffsetMask)) % len(m.prgROM), true
}

// InspectMemory returns the ordinary PPU mapping. It does not change the fetch
// state. The same address can give different bytes after different bus
// accesses, so it does not simulate an extended background fetch.
func (m *Mapper) InspectMemory(address uint16) (byte, bool) {
	if address >= paletteStart {
		return 0, false
	}
	return m.mappedVRAM(address), true
}

// PPUReadMapping reports an ordinary physical CHR-ROM mapping. Advanced fetch
// modes depend on the bus history and can generate bytes. WorkRam is not a
// GraphicsMemory source kind. In these cases the function reports false.
func (m *Mapper) PPUReadMapping(address uint16) (bus.GraphicsMapping, bool) {
	if address >= nameTableStart || m.inhibit != 0 || m.descriptorEnabled ||
		m.metatileEnabled || m.remapEnabled || (m.spriteMode && m.spriteSecond) {

		return bus.GraphicsMapping{}, false
	}

	w := m.ppuWindows[address/chrBankSize]
	if w.writable || len(w.data) == 0 {
		return bus.GraphicsMapping{}, false
	}

	return bus.GraphicsMapping{
		Memory: bus.GraphicsCHRROM,
		Offset: (w.offset + int(address&chrOffsetMask)) % len(w.data),
	}, true
}

// PPUWriteMapping reports false. MXM uploads go to cartridge WorkRam, and
// GraphicsMemory has no source kind for it. The inherited CHR-RAM mapping would
// identify the wrong memory.
func (*Mapper) PPUWriteMapping(uint16) (bus.GraphicsMapping, bool) {
	return bus.GraphicsMapping{}, false
}
