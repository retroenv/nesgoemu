package ppu

// WriteEvent records a PPU register write before the register changes.
// Frame, Scanline, and Dot report the current PPU state when the callback runs.
// The system clocks the PPU before each CPU bus access.
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

func (p *PPU) observeWrite(register uint16, value byte) {
	if p.writeObserver == nil {
		return
	}
	var cpuCycles uint64
	if p.bus.CPU != nil {
		cpuCycles = p.bus.CPU.Cycles()
	}
	p.writeObserver(WriteEvent{
		Frame:      p.renderState.Frame(),
		CPUCycles:  cpuCycles,
		Scanline:   p.renderState.ScanLine(),
		Dot:        p.renderState.Cycle(),
		Register:   register,
		PPUAddress: p.addressing.Address(),
		Value:      value,
	})
}

func (p *PPU) writeData(value byte) {
	address := p.addressing.Address()
	p.memory.Write(address, value)
	p.addressing.Increment(p.control.VRAMIncrement)
}
