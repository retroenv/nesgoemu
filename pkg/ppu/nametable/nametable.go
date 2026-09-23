// Package nametable handles PPU nametables.
package nametable

import (
	"sync"
	"sync/atomic"

	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

const (
	baseAddress = 0x2000 // $2000 contains the nametables
	// VramSize is the size of the nametable buffer.
	// It is normally mapped to the 2kB NES internal VRAM, providing 2 nametables with a mirroring configuration
	// controlled by the cartridge, but it can be partly or fully remapped to RAM on the cartridge,
	// allowing up to 4 simultaneous nametables
	VramSize = nes.NameTableCount * nes.NameTableSize
)

type readHook struct {
	call func(uint16) (uint8, bool)
}

// NameTable routes PPU background reads and writes.
// Each nametable has 1024 bytes. The first 960 bytes select tiles in 32 columns
// and 30 rows. Each tile covers 8 by 8 pixels, so one nametable covers 256 by
// 240 pixels. The last 64 bytes hold attribute data for background palettes.
// The NES has 2 KiB of CIRAM. Cartridge mirroring maps four nametable regions
// to that RAM. A cartridge can map a region to its own RAM or ROM instead.
// See https://www.nesdev.org/wiki/Nametable
// See https://www.nesdev.org/wiki/Mirroring
// See https://www.nesdev.org/wiki/PPU_memory_map
type NameTable struct {
	mu sync.RWMutex

	vram []byte

	mirrorMode cartridge.MirrorMode

	readHook  atomic.Pointer[readHook] // optional mapper read interception
	writeHook func(uint16, byte) bool

	value atomic.Uint32
}

// New returns a new nametable manager.
func New(mirrorMode cartridge.MirrorMode) *NameTable {
	return &NameTable{
		mirrorMode: mirrorMode,
	}
}

// Data returns the nametable data as byte arrays.
func (n *NameTable) Data() [nes.NameTableCount][]byte {
	nameTableIndexes := n.mirrorMode.NametableIndexes()
	data := [nes.NameTableCount][]byte{}

	n.mu.RLock()
	for table := range nes.NameTableCount {
		nameTableIndex := nameTableIndexes[table]
		base := nameTableIndex * nes.NameTableSize
		b := n.vram[base : base+nes.NameTableSize]
		data[table] = b
	}
	n.mu.RUnlock()
	return data
}

// SetVRAM sets the VRAM data buffer. This gets called by the mapper to allow nametable switching.
func (n *NameTable) SetVRAM(vram []byte) {
	n.mu.Lock()
	n.vram = vram
	n.mu.Unlock()
}

// MirrorMode returns the set mirror mode.
func (n *NameTable) MirrorMode() cartridge.MirrorMode {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.mirrorMode
}

// SetMirrorMode sets the mirror mode.
func (n *NameTable) SetMirrorMode(mirrorMode cartridge.MirrorMode) {
	n.mu.Lock()
	n.mirrorMode = mirrorMode
	n.mu.Unlock()
}

// SetReadHook installs an optional mapper-provided read interception function.
// The hook receives the raw PPU address and returns (value, true) if it handles
// the read, or (0, false) to fall through to the default CIRAM read.
func (n *NameTable) SetReadHook(hook func(uint16) (uint8, bool)) {
	if hook == nil {
		n.readHook.Store(nil)
		return
	}
	n.readHook.Store(&readHook{call: hook})
}

// SetWriteHook installs a mapper write hook. A true result stops the default write.
func (n *NameTable) SetWriteHook(hook func(uint16, byte) bool) {
	n.mu.Lock()
	n.writeHook = hook
	n.mu.Unlock()
}

// ReadCIRAM reads a physical CIRAM address without mirroring or mapper hooks.
func (n *NameTable) ReadCIRAM(address uint16) byte {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.vram[address&0x07FF]
}

// WriteCIRAM writes a physical CIRAM address without mirroring or mapper hooks.
func (n *NameTable) WriteCIRAM(address uint16, value byte) {
	n.mu.Lock()
	n.vram[address&0x07FF] = value
	n.mu.Unlock()
}

// Read a value from the nametable address.
func (n *NameTable) Read(address uint16) byte {
	hook := n.readHook.Load()
	if hook != nil {
		if v, ok := hook.call(address); ok {
			return v
		}
	}

	n.mu.RLock()
	base := n.mirroredNameTableAddressToBase(address)
	value := n.vram[base]
	n.mu.RUnlock()
	return value
}

// Write a value to a nametable address.
func (n *NameTable) Write(address uint16, value byte) {
	n.mu.RLock()
	hook := n.writeHook
	n.mu.RUnlock()
	if hook != nil && hook(address, value) {
		return
	}

	n.mu.Lock()
	base := n.mirroredNameTableAddressToBase(address)
	n.vram[base] = value
	n.mu.Unlock()
}

// Fetch a byte from the address and store it in the internal value storage for later retrieval.
func (n *NameTable) Fetch(address uint16) {
	value := n.Read(address)
	n.value.Store(uint32(value))
}

// Value returns the earlier fetched value.
func (n *NameTable) Value() byte {
	return byte(n.value.Load())
}

func (n *NameTable) mirroredNameTableAddressToBase(address uint16) uint16 {
	address = (address - baseAddress) % (nes.NameTableCount * nes.NameTableSize)
	table := address / nes.NameTableSize
	offset := address % nes.NameTableSize

	nameTableIndexes := n.mirrorMode.NametableIndexes()
	nameTableIndex := nameTableIndexes[table]

	base := nameTableIndex*nes.NameTableSize + offset
	return base
}
