// Package mapperbase provides common functionality for most mappers.
package mapperbase

import (
	"fmt"
	"sync"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

const (
	chrMemSize           = 0x2000 // 8K
	defaultChrWindowSize = 0x2000 // 8K
	defaultPrgWindowSize = 0x4000 // 16K
	prgMemSize           = 0x8000 // 32K

	prgRAMStart = 0x6000
	prgRAMEnd   = 0x7FFF

	defaultPrgRAMSize = 0x2000 // 8K, the size that a legacy iNES file implies
)

// bankMapper maps an address to a bank number and offset into that bank.
type bankMapper func(address uint16) (int, uint16)

// Base provides common functionality for most mappers.
type Base struct {
	mu   sync.RWMutex
	bus  *bus.Bus
	name string // optional

	features *feature.Set

	chrRAM []byte
	prgRAM []byte

	mirrorModeTranslation MirrorModeTranslation
	nameTableCount        int
	nameTableBanks        []bank

	chrWindowSize int
	prgWindowSize int

	chrBanks []bank
	prgBanks []bank

	chrWindows []int
	prgWindows []int

	chrBankMapper bankMapper
	prgBankMapper bankMapper

	readHooks  []*readHook
	writeHooks []*writeHook
}

// New creates a new mapper base.
func New(systemBus *bus.Bus) *Base {
	return &Base{
		bus: systemBus,

		features: feature.NewSet(),

		chrWindowSize: defaultChrWindowSize,
		prgWindowSize: defaultPrgWindowSize,

		nameTableCount: 1,
	}
}

// State returns the current state of the mapper.
func (b *Base) State() bus.MapperState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	state := bus.MapperState{
		ID:         b.bus.Cartridge.Mapper,
		Name:       b.name,
		ChrWindows: b.chrWindows,
		PrgWindows: b.prgWindows,
	}

	return state
}

// DeclareFeature adds a supported feature to the mapper inventory.
func (b *Base) DeclareFeature(id feature.ID) {
	b.features.Declare(id)
}

// Features returns the mapper feature inventory in declaration order.
func (b *Base) Features() []feature.Usage {
	return b.features.Features()
}

// MarkFeature records that a supported mapper feature was used.
func (b *Base) MarkFeature(id feature.ID) {
	b.features.Mark(id)
}

// MemorySizes returns the mapper PRG RAM and CHR RAM sizes in bytes.
func (b *Base) MemorySizes() (prgRAM, chrRAM int) {
	return len(b.prgRAM), len(b.chrRAM)
}

// SetName sets the name of the mapper.
func (b *Base) SetName(name string) {
	b.mu.Lock()
	b.name = name
	b.mu.Unlock()
}

// Read a byte from a CHR or PRG memory address.
func (b *Base) Read(address uint16) uint8 {
	var value byte

	for _, hook := range b.readHooks {
		if address >= hook.startAddress && address <= hook.endAddress {
			value, err := hook.hookFunc(address)
			if err != nil {
				panic(fmt.Sprintf("read hook error: %v", err))
			}
			if !hook.onlyProxy {
				return value
			}
		}
	}

	switch {
	case address < 0x2000:
		if len(b.chrBanks) == 0 {
			// The video memory bus is multiplexed with the low byte of the
			// address. A read with no CHR memory returns it.
			// https://www.nesdev.org/wiki/Open_bus_behavior#PPU_open_bus
			return uint8(address)
		}

		b.mu.RLock()
		bankNr, offset := b.chrBankMapper(address)
		bank := &b.chrBanks[bankNr]
		value = bank.data[offset]
		b.mu.RUnlock()

	case address >= prgRAMStart && address <= prgRAMEnd && len(b.prgRAM) > 0:
		b.MarkFeature(feature.PRGRAM)

		// A PRG RAM that is smaller than its window repeats inside it, the address
		// decoding covers only the size of the RAM.
		offset := int(address-prgRAMStart) % len(b.prgRAM)
		value = b.prgRAM[offset]

	case address >= nes.CodeBaseAddress:
		b.mu.RLock()
		bankNr, offset := b.prgBankMapper(address)
		bank := &b.prgBanks[bankNr]
		value = bank.data[offset]
		b.mu.RUnlock()

	default:
		// Addresses without memory return the value of the CPU data bus.
		// https://www.nesdev.org/wiki/Open_bus_behavior#CPU_open_bus
		value = b.OpenBus()
	}
	return value
}

// Write a byte to a CHR or PRG memory address.
func (b *Base) Write(address uint16, value uint8) {
	for _, hook := range b.writeHooks {
		if address >= hook.startAddress && address <= hook.endAddress {
			if err := hook.hookFunc(address, value); err != nil {
				panic(fmt.Errorf("write hook error: %w", err))
			}
			if !hook.onlyProxy {
				return
			}
		}
	}

	switch {
	case address < 0x2000 && len(b.chrRAM) > 0:
		b.mu.Lock()
		bankNr, offset := b.chrBankMapper(address)
		bank := &b.chrBanks[bankNr]
		bank.data[offset] = value
		b.mu.Unlock()

	case address >= prgRAMStart && address <= prgRAMEnd && len(b.prgRAM) > 0:
		b.MarkFeature(feature.PRGRAM)

		// A PRG RAM that is smaller than its window repeats inside it, the address
		// decoding covers only the size of the RAM.
		offset := int(address-prgRAMStart) % len(b.prgRAM)
		b.prgRAM[offset] = value

	default:
		// Addresses without memory ignore writes.
		// https://www.nesdev.org/wiki/CPU_memory_map
	}
}

// Initialize the mapper base with default settings.
func (b *Base) Initialize() {
	b.chrBankMapper = b.defaultChrBankMapper
	b.prgBankMapper = b.defaultPrgBankMapper

	b.setDefaultBankSizes()
	b.setBanks()
	b.setWindows()
}

// Cartridge returns the current cartridge.
func (b *Base) Cartridge() *cartridge.Cartridge {
	return b.bus.Cartridge
}

// NameTableMemory returns the PPU name-table memory.
func (b *Base) NameTableMemory() bus.NameTable {
	return b.bus.NameTable
}

// OpenBus returns the value of the CPU data bus.
func (b *Base) OpenBus() byte {
	if b.bus.OpenBus == nil {
		return 0
	}

	return b.bus.OpenBus.OpenBus()
}

// SetMapperIRQ sets the mapper IRQ input state.
func (b *Base) SetMapperIRQ(active bool) {
	b.bus.SetMapperIRQ(active)
}

func (b *Base) defaultChrBankMapper(address uint16) (int, uint16) {
	offset := address % uint16(b.chrWindowSize)
	windowNr := address / uint16(b.chrWindowSize)
	bankNr := b.chrWindows[windowNr]
	return bankNr, offset
}

func (b *Base) defaultPrgBankMapper(address uint16) (int, uint16) {
	address -= nes.CodeBaseAddress
	offset := address % uint16(b.prgWindowSize)
	windowNr := address / uint16(b.prgWindowSize)
	bankNr := b.prgWindows[windowNr]
	return bankNr, offset
}
