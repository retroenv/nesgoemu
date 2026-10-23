// Package feature tracks the hardware features that a program uses.
package feature

import (
	"slices"
	"sync/atomic"
)

// ID identifies one emulated hardware feature.
type ID uint8

// Hardware feature identifiers. The identifiers are grouped by the component
// that declares them and by their function. Add a new identifier to its group.
// The mapper groups are in alphabetical order. The PPU groups follow the PPU
// registers, and the APU group follows the APU channels. The order of the
// identifiers does not change the reports, which use the declaration order of
// each component. The numeric values can change when identifiers are added.
// Use the name to identify a feature outside one run.
const (
	invalid ID = iota

	// Mapper: memory banking.
	AlternateBank
	CHRBanking
	CHRSourceSelect
	GraphicsReadLatch
	LowBankMapping
	PRGBanking
	ShiftRegister

	// Mapper: mirroring and nametables.
	Mirroring
	NameTableBanking
	NameTableControl
	NameTableFill
	PPUWriteBanking
	SingleScreenMirroring

	// Mapper: extended graphics.
	BGExtendedMode
	ExtendedAttributes
	GraphicsSets
	MetatileAnimation
	Metatiles
	OAMRoutines
	RowAttributes
	ScanlineRemap
	SpriteExtendedMode
	WindowSplit

	// Mapper: storage.
	BlockCopy
	FlashProgramming
	FPGARAM
	PRGRAM

	// Mapper: interrupts.
	CPUCycleIRQ
	ScanlineIRQ
	VectorRedirection

	// Mapper: timing and system.
	ESPMessages
	ExpansionAudio
	PPUBusTiming
	PPURegisterCapture

	// PPU: control register.
	NMI
	BackgroundTableHigh
	SpriteTableHigh
	SpriteSize8x16
	VRAMIncrement32

	// PPU: mask register.
	BackgroundRendering
	SpriteRendering
	Grayscale
	ColorEmphasis

	// PPU: OAM.
	OAMDMA

	// APU: channels and modes.
	Pulse1
	Pulse2
	Triangle
	Noise
	DMC
	PulseSweep
	NoiseShortMode
	DMCLoop
	DMCIRQ
	FrameFiveStep

	count
)

// names is in alphabetical order of the identifiers.
var names = [count]string{
	AlternateBank:         "Alternate bank",
	BackgroundRendering:   "Background rendering",
	BackgroundTableHigh:   "Background pattern table $1000",
	BGExtendedMode:        "Background extended mode",
	BlockCopy:             "Block copy",
	CHRBanking:            "CHR banking",
	CHRSourceSelect:       "CHR source select",
	ColorEmphasis:         "Color emphasis",
	CPUCycleIRQ:           "CPU cycle IRQ",
	DMC:                   "DMC",
	DMCIRQ:                "DMC IRQ",
	DMCLoop:               "DMC loop",
	ESPMessages:           "ESP/Wi-Fi messages",
	ExpansionAudio:        "expansion audio",
	ExtendedAttributes:    "Extended attributes",
	FlashProgramming:      "Flash programming",
	FPGARAM:               "FPGA RAM",
	FrameFiveStep:         "Five-step frame counter",
	GraphicsReadLatch:     "Graphics read latch",
	GraphicsSets:          "Two graphics sets",
	Grayscale:             "Grayscale",
	LowBankMapping:        "Low bank mapping",
	MetatileAnimation:     "Metatile animation",
	Metatiles:             "Metatile backgrounds",
	Mirroring:             "Mirroring",
	NameTableBanking:      "Nametable banking",
	NameTableControl:      "Nametable bank/source control",
	NameTableFill:         "Nametable fill mode",
	NMI:                   "NMI",
	Noise:                 "Noise",
	NoiseShortMode:        "Noise short mode",
	OAMDMA:                "OAM DMA",
	OAMRoutines:           "Auto OAM routines",
	PPUBusTiming:          "PPU bus timing",
	PPURegisterCapture:    "PPU register capture",
	PPUWriteBanking:       "Banked PPU writes",
	PRGBanking:            "PRG banking",
	PRGRAM:                "PRG RAM",
	Pulse1:                "Pulse 1",
	Pulse2:                "Pulse 2",
	PulseSweep:            "Pulse sweep",
	RowAttributes:         "8x1 attributes",
	ScanlineIRQ:           "Scanline IRQ",
	ScanlineRemap:         "Scanline remap",
	ShiftRegister:         "Shift register",
	SingleScreenMirroring: "Single-screen mirroring",
	SpriteExtendedMode:    "Sprite extended mode",
	SpriteRendering:       "Sprite rendering",
	SpriteSize8x16:        "8x16 sprites",
	SpriteTableHigh:       "Sprite pattern table $1000",
	Triangle:              "Triangle",
	VectorRedirection:     "Vector redirection",
	VRAMIncrement32:       "VRAM increment 32",
	WindowSplit:           "Window split",
}

// Usage reports whether one supported feature was used during a run.
type Usage struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
	Used bool   `json:"used"`
}

// User reports the supported features of a component and their usage.
type User interface {
	Features() []Usage
}

// Set holds the supported features of one component and their usage.
// Declare every supported feature before emulation starts. Mark a feature
// when the program uses it.
type Set struct {
	declared  []ID
	supported [count]bool
	used      [count]atomic.Bool
}

// NewSet creates an empty feature set.
func NewSet() *Set {
	return &Set{}
}

// Declare adds a supported feature to the inventory in declaration order.
// A duplicate or invalid identifier is ignored.
func (s *Set) Declare(id ID) {
	if id <= invalid || id >= count || s.supported[id] {
		return
	}

	s.supported[id] = true
	s.declared = append(s.declared, id)
}

// Undeclare removes a feature from the inventory. An undeclared identifier is
// ignored.
func (s *Set) Undeclare(id ID) {
	if id <= invalid || id >= count || !s.supported[id] {
		return
	}

	s.supported[id] = false
	s.used[id].Store(false)
	s.declared = slices.DeleteFunc(s.declared, func(declared ID) bool { return declared == id })
}

// Mark records that a supported feature was used. An unknown identifier is ignored.
func (s *Set) Mark(id ID) {
	if id > invalid && id < count && s.supported[id] {
		s.used[id].Store(true)
	}
}

// Features returns the full feature inventory in declaration order.
func (s *Set) Features() []Usage {
	usage := make([]Usage, 0, len(s.declared))
	for _, id := range s.declared {
		usage = append(usage, Usage{
			ID:   id,
			Name: names[id],
			Used: s.used[id].Load(),
		})
	}

	return usage
}
