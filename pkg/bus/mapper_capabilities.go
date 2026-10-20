package bus

import "io"

// BatteryMapper provides persistent cartridge data.
// The system loads this data before emulation and saves it after emulation stops.
type BatteryMapper interface {
	LoadBattery(io.Reader) error
	SaveBattery(io.Writer) error
}

// BatteryBacker reports whether a mapper has data that must persist.
type BatteryBacker interface {
	BatteryBacked() bool
}

// CPUClocker receives elapsed CPU cycles.
// The system calls ClockCPU before it advances the PPU for each CPU cycle.
type CPUClocker interface {
	ClockCPU(cycles uint64)
}

// ExpansionAudioSource supplies a mapper's current normalized audio level.
// The APU samples this after the mapper has advanced for the same CPU cycle.
type ExpansionAudioSource interface {
	ExpansionAudioOutput() float64
}

// MapperResetter resets mapper registers and bank state.
// The system resets the mapper before it resets the PPU and CPU.
type MapperResetter interface {
	Reset()
}

// MemoryInspector exposes mapper memory without emulated bus effects.
// The queries do not advance mapper state, count PPU bus reads, or mark
// features as used.
type MemoryInspector interface {
	// InspectCHR returns the pattern table byte at a PPU address in
	// $0000-$1FFF as the mapper currently maps it.
	InspectCHR(address uint16) byte
	// InspectPRGRAM returns the byte at an offset of the physical PRG-RAM
	// array. Volatile bytes come first, then nonvolatile bytes. It reports
	// false for an offset outside the array.
	InspectPRGRAM(offset int) (byte, bool)
}

// CPUMemoryInspector exposes cartridge CPU mappings without bus effects.
// For $4020-$FFFF, it has priority over the PRG-RAM and PRG-ROM inspectors.
// A board that maps RAM above $8000 or ROM below $8000 needs it.
type CPUMemoryInspector interface {
	InspectCPU(address uint16) (byte, bool)
}

// PPUMemoryInspector exposes PPU address space without bus effects.
type PPUMemoryInspector interface {
	// InspectMemory returns the byte at a PPU address in $0000-$3FFF. It
	// reports false when the board cannot supply it.
	InspectMemory(address uint16) (byte, bool)
}

// NameTableInspector exposes nametable memory without mapper hooks.
type NameTableInspector interface {
	// InspectNameTable returns the byte of the mirrored physical nametable
	// memory at a PPU address in $2000-$3EFF.
	InspectNameTable(address uint16) byte
}

// MemorySizer reports the mapper PRG RAM and CHR RAM sizes in bytes.
type MemorySizer interface {
	MemorySizes() (prgRAM, chrRAM int)
}

// PCMReader provides mapper data for CPU reads from $4011.
// The memory controller uses this value instead of the APU value.
type PCMReader interface {
	ReadPCM() byte
}

// PPURegisterObserver receives each CPU write to a PPU register after the PPU
// applies it. The address is the original bus address, so a board can
// identify writes to register mirrors.
type PPURegisterObserver interface {
	ObservePPURegisterWrite(address uint16, value byte)
}

// PPUTicker receives the current position after each PPU clock advances.
// Rendering is true when background or sprite rendering is enabled.
type PPUTicker interface {
	TickPPU(cycle, scanLine int, rendering bool)
}

// PRGROMMapper reports the physical PRG ROM offset at a CPU address.
// Addresses outside $8000-$FFFF, RAM, and register data return false.
// The query does not read memory or change mapper state.
type PRGROMMapper interface {
	PRGROMOffset(address uint16) (int, bool)
}

// SpriteExtFetcher identifies the OAM entry for a sprite pattern read.
// The PPU uses an OAM index of -1 for an empty slot. It clears the active
// entry with an OAM index of -1 and a sprite size of 0 after each read.
type SpriteExtFetcher interface {
	SetActiveSpriteExt(oamIndex, spriteSize int)
}

// TimingEnabler enables detailed mapper bus timing.
// The PPU calls EnableBusTiming once when it connects to the mapper.
type TimingEnabler interface {
	EnableBusTiming()
}
