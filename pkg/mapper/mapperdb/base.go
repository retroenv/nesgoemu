// Package mapperdb contains all mapper implementations.
package mapperdb

import (
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// Base defines the base mapper interface that contains helper functions for shared functionality.
type Base interface {
	bus.Mapper

	ChrBankCount() int
	SetChrRAM(ram []byte)
	SetChrWindow(window, bank int)
	SetChrWindowSize(size int)

	PrgBankCount() int
	SetPrgRAM(ram []byte)
	SetPrgWindow(window, bank int)
	SetPrgWindowSize(size int)

	NameTable(bank int) []byte
	SetMirrorModeTranslation(translation mapperbase.MirrorModeTranslation)
	SetNameTableCount(count int)
	SetNameTableMirrorMode(mirrorMode cartridge.MirrorMode) error
	SetNameTableMirrorModeIndex(index uint8) error
	SetNameTableWindow(bank int)

	AddReadHook(startAddress, endAddress uint16, hookFunc mapperbase.ReadHookFunc) mapperbase.Hook
	AddWriteHook(startAddress, endAddress uint16, hookFunc mapperbase.WriteHookFunc) mapperbase.Hook
	Cartridge() *cartridge.Cartridge
	Initialize()
	SetName(name string)
}
