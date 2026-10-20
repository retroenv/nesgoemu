package nes

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/system/nes"
)

// Peek reads one byte of CPU address space without emulated bus side effects.
// It reads internal RAM directly, PRG-RAM through a mapper inspector, and
// PRG-ROM through the mapper PRG offset. It reports false when the address has
// no side-effect-free path. A PRG-RAM read needs a mapper that implements
// bus.MemoryInspector; other mappers report false.
func (sys *System) Peek(address uint16) (byte, bool) {
	switch {
	case address <= cpuRAMEnd:
		return sys.memory.InspectRAM(address)

	case address >= prgRAMStart && address < prgRAMEnd:
		inspector, ok := sys.Bus.Mapper.(bus.MemoryInspector)
		if !ok {
			return 0, false
		}
		return inspector.InspectPRGRAM(int(address) - prgRAMStart)

	case address >= nes.CodeBaseAddress:
		return sys.peekPRGROM(address)

	default:
		return 0, false
	}
}

// PeekRange reads a range of CPU address space without side effects. It stops
// at the first address that Peek cannot read.
func (sys *System) PeekRange(address uint16, length int) []byte {
	if length <= 0 {
		return nil
	}
	data := make([]byte, 0, length)
	for offset := range length {
		value, ok := sys.Peek(address + uint16(offset))
		if !ok {
			break
		}
		data = append(data, value)
	}
	return data
}

// PRGLocation reports the PRG bank number and the physical PRG-ROM offset of a
// CPU address. It reports false when the random-access mapper cannot map the
// address. The bank number is -1 when the mapper state does not expose it.
func (sys *System) PRGLocation(address uint16) (bank, prgOffset int, ok bool) {
	mapper, isPRG := sys.Bus.Mapper.(bus.PRGROMMapper)
	if !isPRG {
		return -1, 0, false
	}
	offset, mapped := mapper.PRGROMOffset(address)
	if !mapped {
		return -1, 0, false
	}
	return prgBankNumber(sys.Bus.Mapper.State().PrgWindows, address), offset, true
}

// peekPRGROM reads a byte of PRG-ROM through the mapper PRG offset.
func (sys *System) peekPRGROM(address uint16) (byte, bool) {
	mapper, ok := sys.Bus.Mapper.(bus.PRGROMMapper)
	if !ok {
		return 0, false
	}
	offset, ok := mapper.PRGROMOffset(address)
	if !ok {
		return 0, false
	}
	prg := sys.Bus.Cartridge.PRG
	if offset < 0 || offset >= len(prg) {
		return 0, false
	}
	return prg[offset], true
}

// prgBankNumber derives the bank of a CPU address from the window-to-bank list.
// It returns -1 when the list is empty or its length does not divide the CPU
// window size.
func prgBankNumber(windows []int, address uint16) int {
	count := len(windows)
	if count == 0 || prgWindowSize%count != 0 {
		return -1
	}
	window := (int(address) - nes.CodeBaseAddress) / (prgWindowSize / count)
	if window < 0 || window >= count {
		return -1
	}
	return windows[window]
}

const (
	// prgWindowSize is the size of the CPU PRG address window $8000-$FFFF.
	prgWindowSize = 0x8000
	cpuRAMEnd     = 0x1FFF
	prgRAMStart   = 0x6000
	prgRAMEnd     = 0x8000
)
