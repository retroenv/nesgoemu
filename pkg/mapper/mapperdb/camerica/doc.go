// Package camerica implements Camerica and Codemasters cartridge boards:
// mapper 71 (BF9093 and BF9097) and mapper 232 Quattro (BF9096).
//
// # Common behavior
//
// The mapper 71 page states 8 KiB CHR RAM, no PRG RAM, and no bus conflicts.
// The mapper 232 page does not state this, but the emulator uses the same
// layout for both mappers. The emulator rejects CHR ROM, PRG RAM,
// nonvolatile CHR RAM, a battery, a trainer, and four-screen mirroring. It
// accepts 64, 128, or 256 KiB PRG ROM and selects bank zero at power-on. The
// lockout latch at $E000-$FFFF is not emulated.
//
// # Mapper 71
//
// A write to $C000-$FFFF selects the 16 KiB PRG bank at $8000-$BFFF. The
// last bank is fixed at $C000-$FFFF. The BF9093 has 4 bank bits and the
// BF9097 has 3 bank bits.
//
// NES 2.0 submapper 0 has fixed mirroring. Submapper 1 is the Fire Hawk board
// (BF9097). It has up to 128 KiB PRG ROM and a mirroring register at
// $8000-$9FFF, where bit 4 selects single-screen nametable 0 or 1. Fire Hawk
// uses cycle-timed code for the change. The emulator applies the change at
// once, which Kevtris reports as sufficient for the game.
//
// A legacy iNES header has fixed mirroring until the first write to
// $9000-$9FFF. After that write, the Fire Hawk register is active. The mapper
// 71 page attributes this rule to FCEUX.
//
// # Mapper 232
//
// A write to $8000-$BFFF selects a 64 KiB block with bits 4-3. A write to
// $C000-$FFFF selects the 16 KiB page at $8000-$BFFF with bits 1-0. Page 3
// of the block is fixed at $C000-$FFFF. Mirroring is fixed, because only the
// BF9097 drives CIRAM A10.
//
// NES 2.0 submapper 0 is the normal board. Submapper 1 is the Aladdin Deck
// Enhancer variant, which swaps the two bits of the outer bank number.
//
// # Limits of mapper 232
//
// The bit order of the outer bank is a project decision. The emulator uses
// the NESdev order: bit 3 is the low block bit, and submapper 1 swaps the
// bits. The MAME default BF9096 board uses the same order. Change this order
// only with a Quattro ROM that has a known block layout, or with hardware
// test results. The sources disagree with the NESdev order:
//   - The NESdev BF9096 pinout lists $8000.4 as PRG A16 and $8000.3 as
//     PRG A17.
//   - A forum post that reverse-engineers a Quattro PCB reports that the chip
//     latches D4 into A16 and D3 into A17. The chip can be a different
//     version. A second forum author gives the same order.
//   - The Kevtris document says "Bits 4 and 5 select which 64K bank". It does
//     not give the weight of each bit.
//
// No source states how an iNES file orders the 64 KiB blocks. MAME notes that
// it needs correct dumps to know if its "alt" board with swapped bits exists.
//
// No source states a minimum PRG ROM size or the power-on register state.
// The Kevtris document lists the boards BIC-43 and BIC-48 with a maximum of
// 256 KiB PRG ROM, but it does not link boards to chips.
//
// # Hardware references
//
// https://www.nesdev.org/wiki/INES_Mapper_071
// https://www.nesdev.org/wiki/INES_Mapper_232
// https://www.nesdev.org/wiki/Camerica_BF909x_pinout
// https://www.nesdev.org/wiki/NES_2.0_submappers#071:_Codemasters
// https://www.nesdev.org/wiki/NES_2.0_submappers#232:_Quattro
// https://forums.nesdev.org/viewtopic.php?t=15297
// https://kevtris.org/mappers/camerica/index.html
package camerica
