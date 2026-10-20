package ppu

import "github.com/retroenv/nesgoemu/pkg/bus"

// Snapshot is a side-effect-free copy of the PPU register and render state.
// Read it while emulation is stopped, or from the emulation goroutine.
type Snapshot struct {
	Scanline int
	Dot      int
	Frame    uint64

	Control byte
	Mask    byte
	Status  byte

	V          uint16 // Current VRAM address v.
	T          uint16 // Temporary VRAM address t.
	FineX      uint16 // Fine X scroll x.
	WriteLatch bool   // Write toggle w.
	ReadBuffer byte   // Delayed data read buffer.
}

// Snapshot returns a copy of the PPU register and render state.
func (p *PPU) Snapshot() Snapshot {
	return Snapshot{
		Scanline:   p.renderState.ScanLine(),
		Dot:        p.renderState.Cycle(),
		Frame:      p.renderState.Frame(),
		Control:    p.control.Value(),
		Mask:       p.mask.Value(),
		Status:     p.status.Value(),
		V:          p.addressing.Address(),
		T:          p.addressing.TempAddress(),
		FineX:      p.fineX,
		WriteLatch: p.addressing.Latch(),
		ReadBuffer: p.dataReadBuffer,
	}
}

// PeekVRAM returns one byte of PPU address space $0000-$3FFF without bus
// effects. Pattern table bytes come from the mapper inspector, or from CHR-ROM
// when the mapper reports a CHR-ROM read mapping. It reports false when the
// board cannot supply the byte. CHR-RAM without an inspector reports false.
func (p *PPU) PeekVRAM(address uint16) (byte, bool) {
	if value, ok := p.InspectMemory(address); ok {
		return value, true
	}
	return p.peekPatternTable(address)
}

// peekPatternTable reads a pattern table byte from CHR-ROM through the mapper
// read mapping.
func (p *PPU) peekPatternTable(address uint16) (byte, bool) {
	address &= 0x3FFF
	if address >= 0x2000 {
		return 0, false
	}
	inspector, ok := p.bus.Mapper.(bus.PPUMappingInspector)
	if !ok {
		return 0, false
	}
	mapping, ok := inspector.PPUReadMapping(address)
	if !ok || mapping.Memory != bus.GraphicsCHRROM {
		return 0, false
	}
	chr := p.bus.Cartridge.CHR
	if mapping.Offset < 0 || mapping.Offset >= len(chr) {
		return 0, false
	}
	return chr[mapping.Offset], true
}
