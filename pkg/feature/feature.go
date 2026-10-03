// Package feature tracks the hardware features that a program uses.
package feature

import "sync/atomic"

// ID identifies one emulated hardware feature.
type ID uint8

// Hardware feature identifiers. The identifiers are grouped by the component
// that declares them and by their function. Append new identifiers before
// count. Do not reorder existing identifiers because Usage serializes their
// numeric values.
const (
	invalid ID = iota

	// Mapper: memory banking.
	PRGBanking
	CHRBanking
	CHRSourceSelect
	LowBankMapping
	ShiftRegister

	// Mapper: mirroring and nametables.
	Mirroring
	SingleScreenMirroring
	NameTableBanking
	NameTableControl
	NameTableFill

	// Mapper: extended graphics.
	ExtendedAttributes
	BGExtendedMode
	WindowSplit
	SpriteExtendedMode
	OAMRoutines

	// Mapper: storage.
	PRGRAM
	FPGARAM
	FlashProgramming

	// Mapper: interrupts.
	ScanlineIRQ
	CPUCycleIRQ
	VectorRedirection

	// Mapper: timing and system.
	PPUBusTiming
	ESPMessages
	ExpansionAudio

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

var names = [count]string{
	PRGBanking:            "PRG banking",
	CHRBanking:            "CHR banking",
	CHRSourceSelect:       "CHR source select",
	LowBankMapping:        "Low bank mapping",
	ShiftRegister:         "Shift register",
	Mirroring:             "Mirroring",
	SingleScreenMirroring: "Single-screen mirroring",
	NameTableBanking:      "Nametable banking",
	NameTableControl:      "Nametable bank/source control",
	NameTableFill:         "Nametable fill mode",
	ExtendedAttributes:    "Extended attributes",
	BGExtendedMode:        "Background extended mode",
	WindowSplit:           "Window split",
	SpriteExtendedMode:    "Sprite extended mode",
	OAMRoutines:           "Auto OAM routines",
	PRGRAM:                "PRG RAM",
	FPGARAM:               "FPGA RAM",
	FlashProgramming:      "Flash programming",
	ScanlineIRQ:           "Scanline IRQ",
	CPUCycleIRQ:           "CPU cycle IRQ",
	VectorRedirection:     "Vector redirection",
	PPUBusTiming:          "PPU bus timing",
	ESPMessages:           "ESP/Wi-Fi messages",
	ExpansionAudio:        "expansion audio",
	NMI:                   "NMI",
	BackgroundTableHigh:   "Background pattern table $1000",
	SpriteTableHigh:       "Sprite pattern table $1000",
	SpriteSize8x16:        "8x16 sprites",
	VRAMIncrement32:       "VRAM increment 32",
	BackgroundRendering:   "Background rendering",
	SpriteRendering:       "Sprite rendering",
	Grayscale:             "Grayscale",
	ColorEmphasis:         "Color emphasis",
	OAMDMA:                "OAM DMA",
	Pulse1:                "Pulse 1",
	Pulse2:                "Pulse 2",
	Triangle:              "Triangle",
	Noise:                 "Noise",
	DMC:                   "DMC",
	PulseSweep:            "Pulse sweep",
	NoiseShortMode:        "Noise short mode",
	DMCLoop:               "DMC loop",
	DMCIRQ:                "DMC IRQ",
	FrameFiveStep:         "Five-step frame counter",
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
