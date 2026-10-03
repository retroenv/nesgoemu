// Package bus provides a system Bus connecting all main system parts.
package bus

import (
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// Bus contains all NES sub system components.
// Since many components access other components, this structure
// allows an easy access and reduces the import dependencies and
// initialization order issues.
type Bus struct {
	Cartridge *cartridge.Cartridge // used by Mapper

	APU         APU
	Controller1 Controller          // used by Memory
	Controller2 Controller          // used by Memory
	CPU         CPU                 // used by PPU
	DMA         OAMRequester        // Optional CPU bus scheduler for OAM transfers.
	Mapper      Mapper              // used by Memory and PPU
	Memory      cpu6502.BasicMemory // used by CPU
	NameTable   NameTable           // used by CPU and Mapper
	OpenBus     OpenBus             // used by Mapper
	PPU         PPU                 // used by Memory

	apuIRQ    bool
	mapperIRQ bool
}

// OAMRequester schedules an OAM transfer for the next CPU read cycle.
type OAMRequester interface {
	RequestOAM(page byte)
}

// OpenBus provides the value of the CPU data bus latch.
// https://www.nesdev.org/wiki/Open_bus_behavior#CPU_open_bus
type OpenBus interface {
	OpenBus() byte
}

// SetAPUIRQ sets the APU source without changing the mapper source.
func (b *Bus) SetAPUIRQ(active bool) {
	b.apuIRQ = active
	b.updateIRQ()
}

// SetMapperIRQ sets the mapper source without changing the APU source.
func (b *Bus) SetMapperIRQ(active bool) {
	b.mapperIRQ = active
	b.updateIRQ()
}

func (b *Bus) updateIRQ() {
	if b.CPU != nil {
		b.CPU.SetIRQ(b.apuIRQ || b.mapperIRQ)
	}
}
