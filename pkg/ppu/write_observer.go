package ppu

// WriteEvent records a PPU register write before the register changes.
// Frame, Scanline, and Dot report the current PPU state when the callback runs.
// The system clocks the PPU after each CPU instruction.
type WriteEvent struct {
	CPUCycles uint64
	Dot       int
	Frame     uint64
	Scanline  int

	PPUAddress uint16
	Register   uint16
	Value      byte
}

// ObserveWrites replaces the optional PPU register write observer.
// The emulation goroutine calls it before it applies each write.
// A nil observer disables it. The observer must not change system state.
func (p *PPU) ObserveWrites(observer func(WriteEvent)) {
	p.writeObserver = observer
}
