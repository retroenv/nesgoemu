package bus

import (
	"io"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// BatteryMapper provides persistent cartridge data.
// The system loads this data before emulation and saves it after emulation stops.
type BatteryMapper interface {
	LoadBattery(io.Reader) error
	SaveBattery(io.Writer) error
}

// CPUClocker receives elapsed CPU cycles.
// The system calls ClockCPU before it advances the PPU for each CPU cycle.
type CPUClocker interface {
	ClockCPU(cycles uint64)
}

// MapperResetter resets mapper registers and bank state.
// The system resets the mapper before it resets the PPU and CPU.
type MapperResetter interface {
	Reset()
}

// PCMReader provides mapper data for CPU reads from $4011.
// The memory controller uses this value instead of the APU value.
type PCMReader interface {
	ReadPCM() byte
}

// PPUTicker receives the current position after each PPU clock advances.
// Rendering is true when background or sprite rendering is enabled.
type PPUTicker interface {
	TickPPU(cycle, scanLine int, rendering bool)
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

// MapperState contains the current state of the mapper.
type MapperState struct {
	ID   uint16 `json:"id"`
	Name string `json:"name"`

	ChrWindows []int `json:"chrWindows"`
	PrgWindows []int `json:"prgWindows"`
}

// Mapper represents a mapper memory access interface.
type Mapper interface {
	cpu6502.BasicMemory

	MirrorMode() cartridge.MirrorMode
	State() MapperState
}
