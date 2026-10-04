// Package mmc3 implements standard Sharp MMC3 cartridge boards.
// It supports CHR ROM or CHR RAM, up to 8 KiB of PRG RAM, and fixed four-screen
// nametables. NES 2.0 submapper 0 is supported. Other explicit submappers return
// an error. MMC6, NEC MMC3, MC-ACC, T9552, and mapper 119 TQROM are not implemented.
//
// The A12 filter uses nine elapsed PPU dots. It does not model the M2 phase.
// The PPU reports memory transactions, but not all address bus changes from
// $2006 or idle dots. With the background pattern table at $1000, this
// approximation can add counter clocks at the start of a line. Hardware test
// ROM results are required before claims of accurate IRQ timing.
//
// Hardware references:
// https://www.nesdev.org/wiki/MMC3
// https://www.nesdev.org/wiki/NES_2.0_submappers#004:_MMC3
package mmc3
