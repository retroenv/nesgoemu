# MXM / Former Dawn demo

Mapper **800, submapper 0** runs the downloadable Former Dawn v3.2 Mesen demo.
The implementation follows the behavior of the MesenMXM executable in the demo
package. Other MXM hardware and game revisions are not tested.

## Mapper revisions and House of Truth

The 2022 House of Truth N8, N8 Pro and PowerPak demos use **mapper 220,
submapper 1**. nesgoemu does not support them. They use some of the same
graphics ideas, but they are not compatible with **mapper 800, submapper 0**.
The `v3.2` label identifies the Former Dawn demo package. It is not a Mesen
engine version or an MXM hardware revision.

House of Truth has control registers in the `$6000` range and reads palette
values at `$6010`–`$601F`. For mapper 800, `$6000` is the start of a switchable
memory window, and the control registers are in the `$5000` range. Thus, a
House of Truth ROM with a mapper 800 header uses the wrong register protocol.

The House packages contain `220.RBF` files, which configure the flashcart
FPGAs. `MAPROUT.BIN` connects mapper IDs to these configurations. The PowerPak
`.MAP` file contains a loader and a Xilinx configuration. These files are
hardware build products. nesgoemu does not load them as mapper plugins. The N8
and PowerPak ROM bytes are the same, but their FPGA builds are different. The
N8 Pro ROM also has a different graphics layout.

The word "palette" has two different meanings:

- The mapper supplies NES palette selections and color indices. Its register
  responses and 8×1 attribute fetches are part of the emulation.
- A `.pal` display file converts the resulting color indices and emphasis bits
  to RGB. See [Display Palette](usage.md#display-palette).

A display palette cannot replace missing mapper responses. Also, FPGA block RAM
that starts with zeros does not prove that palette reads return zero. Logic or
data that the software uploads can supply these responses.

## Running the Former Dawn demo

Build the emulator and run the demo in the GUI:

```bash
go build .
./nesgoemu -m /path/to/former_dawn_demo_v3_2_mesen.nes
```

The GUI needs the [SDL2 runtime](gui.md). `-m` mutes the audio. The emulator
ignores the MXM expansion audio registers. The NES APU operates as usual.

The controls are the usual emulator controls: the arrow keys move, Z is A, X is
B, Enter is Start, and Backspace is Select. Push Enter at the title screen.

To use the colors of the demo package, use
`-palette /path/to/unsatv6_emph.pal`. The seven nonzero emphasis sets in this
file have the same values.

The emulator shows each completed frame without frame blending. The demo
alternates the colors of dialogue portraits, so this alternation is visible.
The Mesen build in the demo package blends frames with its own display filter.

## Implemented behavior

- Four switchable 8 KiB CPU windows at `$6000`–`$DFFF`, a fixed last PRG bank,
  alternate interrupt banking, and the graphics-memory read latch.
- Sixteen 512-byte pattern windows, ROM/RAM source selection, and four banked
  nametables.
- Extended background descriptors and all four attribute modes, metatile
  indirection, two graphics sets, and vertical scanline remapping.
- PPU register write capture and banked PPUDATA writes.
- The dummy attribute fetches of the reference emulator during sprite loading.
  These fetches keep the 42-fetch background/sprite phase gate of MXM aligned.
- Scanline IRQ detection, acknowledgement, and entry-phase reports.
- Synchronous block copies with the forward-overlap behavior of the reference
  emulator.
- RAM, PRG, and ordinary PPU memory inspection without side effects.

## Validation

Synthetic unit tests in `pkg/mapper/mapperdb/mxm` and `pkg/nes` check the
banking, asymmetric register reads, interrupt context, graphics aperture,
block copies, descriptor sources, attribute modes 0 and 2, per-scanline
palettes, metatile animation, vertical remap, PPU register tap, and IRQ
sequence. No unit test checks attribute modes 1 and 3.

An optional end-to-end test loads an external ROM, boots it, pushes Start, and
runs into the game. Then it makes sure that movement changes the picture:

```bash
NESGOEMU_MXM_ROM=/path/to/former_dawn_demo_v3_2_mesen.nes \
  go test ./pkg/nes -run TestMXMFormerDawnDemo -v
```

The repository does not contain the ROM. When the environment variable is not
set, the test skips. The other mapper and system tests do not need the game.

## Limits

- The emulator does not implement the MXM expansion PCM, FIR, and echo audio.
- The implementation is compatible with the emulator in the demo package. The
  behavior of a physical cartridge is not verified.
- The NES PPU limit of eight sprites per scanline applies. The demo package
  enables the optional Mesen sprite-limit removal. This is an emulator
  enhancement, not an MXM hardware function. The published MXM design keeps
  the PPU and its 64-entry sprite list. Smaller CHR banks let each entity
  animate independently.
- Writes after the end of the 1 KiB scanline-remap table and incorrect metatile
  RAM accesses stay inside the tables. The reference emulator does not limit
  these accesses.
- Storage and job registers that the reference emulator does not have use
  open-bus or ignored-write behavior.
- A complete playthrough and physical hardware timing are not verified.

The demo package is available from the
[download page of the developer](https://somethingnerdy.com/former-dawn-playable-demo-downloads/).
The SHA-256 of its ROM is
`39f9e632df94c61d2a31ef47d87c83f8f1db06106cee803cee8d2872381f5d61`.

## Public design references

The [graphics and memory overview](https://somethingnerdy.com/unlocking-the-nes-for-former-dawn/)
of the developer explains why the design has smaller attributes, banking, and
shared CPU/PPU memory.
[Expansion Evolution](https://somethingnerdy.com/expansion-evolution/) describes
the later eight-channel PCM design. These pages give architectural context. The
core in the demo package is the reference for the exact registers, bit layouts,
and timing of this implementation.
