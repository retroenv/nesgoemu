package bus

import (
	"image"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// APU represents the Audio Processing Unit.
type APU interface {
	cpu6502.BasicMemory
}

// PPU represents the Picture Processing Unit.
type PPU interface {
	cpu6502.BasicMemory

	Image() *image.RGBA
	Palette() Palette
	Step(cycles int)
}

// Palette represents the PPU palette.
type Palette interface {
	cpu6502.BasicMemory

	Data() [nes.PaletteSize]byte
}

// NameTable represents a name table interface.
type NameTable interface {
	cpu6502.BasicMemory

	Data() [nes.NameTableCount][]byte
	MirrorMode() cartridge.MirrorMode
	SetMirrorMode(mirrorMode cartridge.MirrorMode)
	SetVRAM(vram []byte)

	Fetch(address uint16)
	Value() byte
}
