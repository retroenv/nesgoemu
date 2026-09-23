// Package palette handles PPU palette support.
package palette

import (
	"sync/atomic"

	"github.com/retroenv/retrogolib/arch/system/nes"
)

// Palette implements PPU palette support.
type Palette struct {
	data [nes.PaletteSize]atomic.Uint32 // contains color indexes
}

// New returns a new palette manager.
func New() *Palette {
	return &Palette{}
}

// Read a value from the palette address.
func (p *Palette) Read(address uint16) byte {
	base := mirroredPaletteAddressToBase(address)
	return byte(p.data[base].Load())
}

// Write a value to a palette address.
func (p *Palette) Write(address uint16, value byte) {
	base := mirroredPaletteAddressToBase(address)
	p.data[base].Store(uint32(value))
}

// Data returns the palette data as a byte array. Concurrent writes can make
// the result contain values from different points in time.
func (p *Palette) Data() [nes.PaletteSize]byte {
	var data [nes.PaletteSize]byte
	for index := range data {
		data[index] = byte(p.data[index].Load())
	}
	return data
}

func mirroredPaletteAddressToBase(address uint16) uint16 {
	// $3F20-$3FFF are mirrors of $3F00-$3F1F
	address %= nes.PaletteSize

	// $3F10/$3F14/$3F18/$3F1C are mirrors of $3F00/$3F04/$3F08/$3F0C
	if address >= 0x10 && address%4 == 0 {
		address -= 0x10
	}
	return address
}
