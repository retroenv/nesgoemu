package mxm

import "github.com/retroenv/nesgoemu/pkg/bus"

// InspectCPU reads the effective cartridge mapping without latching graphics
// data, acknowledging interrupts, or resetting the frame detector at vectors.
func (m *Mapper) InspectCPU(address uint16) (byte, bool) {
	if address < cpuWindowBase {
		return 0, false
	}

	slot := int(address-cpuWindowBase) / prgBankSize
	if m.prgFlags[slot]&prgGraphicsLatch != 0 {
		return 0, true
	}

	w := m.cpuWindows[slot]
	if slot == alternateSlot && m.altEnabled {
		w.data, w.offset = m.prgROM, int(m.altBank)*prgBankSize
		if m.altRAM {
			w.data = m.ram
		}
	}

	return readWrapped(w.data, w.offset+int(address&prgOffsetMask), m.OpenBus()), true
}

// InspectPRGRAM reads physical RAM without changing the mapper state.
func (m *Mapper) InspectPRGRAM(offset int) (byte, bool) {
	if offset < 0 || offset >= len(m.ram) {
		return 0, false
	}
	return m.ram[offset], true
}

// InspectCHR reads the ordinary mapping without advancing fetch/IRQ state.
func (m *Mapper) InspectCHR(address uint16) byte {
	return m.mappedVRAM(address & (patternTableEnd - 1))
}

// PRGROMOffset exposes effective CPU ROM mappings, including the interrupt bank.
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

// InspectMemory exposes ordinary PPU mappings without advancing fetch state.
// It intentionally does not simulate an extended background fetch: the same
// address can produce different bytes depending on preceding bus transactions.
func (m *Mapper) InspectMemory(address uint16) (byte, bool) {
	if address >= paletteStart {
		return 0, false
	}
	return m.mappedVRAM(address), true
}

// PPUReadMapping reports an ordinary physical CHR-ROM mapping. Advanced fetch
// modes depend on bus history and may generate bytes; WorkRam is not one of
// GraphicsMemory's source kinds. Those contexts report false.
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

// PPUWriteMapping reports false because MXM uploads target cartridge WorkRam,
// which has no GraphicsMemory source kind. Inherited CHR-RAM mappings would
// incorrectly identify a different backing allocation.
func (*Mapper) PPUWriteMapping(uint16) (bus.GraphicsMapping, bool) {
	return bus.GraphicsMapping{}, false
}
