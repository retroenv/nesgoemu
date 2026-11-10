# Usage

nesgoemu runs iNES ROM files from the command line.

## Basic Usage

Run a NES ROM with the default GUI:

```bash
nesgoemu game.nes
```

Run without the GUI:

```bash
nesgoemu -c game.nes
```

## Command-Line Options

```text
usage: nesgoemu [options] <file to emulate>

  -a string
        listening address for the debug server to use (default "127.0.0.1:8080")
  -c    console mode, disable GUI
  -d    start built-in webserver for debug mode
  -e int
        entrypoint to start the CPU (default -1)
  -features
        print the hardware features that the program used when the emulator stops
  -m    mute audio output
  -no-printf
        ignore program text output written to $401B and $401C
  -no-summary
        do not print the run summary when the emulator stops
  -palette string
        RGB .pal file (192 or 1536 bytes)
  -s int
        stop execution at address (default -1)
  -t    print CPU tracing
  -wav string
        write deterministic audio to a new WAV file
  -audio-frames uint
        number of sample frames to record with -wav (default 441000)
```

## Display Palette

The display palette converts the PPU color index and the PPUMASK emphasis bits
to RGB. It does not change palette RAM. The executable contains a default
palette: the `2C02_default.pal` example of
[Pally](https://github.com/Gumball2415/pally), revision `9635e9b`. Pally makes
this palette from a 2C02 NTSC signal model. It has explicit RGB values for all
eight emphasis values. Its license is in
[pkg/ppu/palettes/LICENSE.md](../pkg/ppu/palettes/LICENSE.md).

Use `-palette` to load a different RGB `.pal` file:

```bash
nesgoemu -palette custom.pal game.nes
```

A 1536-byte file supplies 64 colors for each of the eight emphasis values. A
192-byte file supplies only 64 colors. For this size, the emulator calculates
the emphasis colors: each emphasis bit multiplies the two other channels by
3/4. This is an approximation.

The palette applies to the GUI and frame captures. The `ppu.RGB` function
uses the default palette.

## Common Run Modes

Enable CPU tracing:

```bash
nesgoemu -t game.nes
nesgoemu -c -t game.nes > trace.log
```

Start the built-in web debugger:

```bash
nesgoemu -d game.nes
nesgoemu -d -a 127.0.0.1:9000 game.nes
```

Control execution addresses:

```bash
nesgoemu -e 0x8000 game.nes
nesgoemu -s 0x8100 game.nes
nesgoemu -e 0x8000 -s 0x8100 game.nes
```

## Run Summary

The emulator prints a summary when it stops. The summary shows the ROM details and the SDL library
that the GUI and the audio output used. Use the `-features` flag to add the hardware features that
the run used. Use the `-no-summary` flag to print no summary.

In the feature lists, a used feature has an `x` mark. An unused feature has no mark.

The `Submapper` line shows the submapper number of a NES 2.0 header. It is absent for an iNES 1 header,
which has no submapper field.

In each feature list, the used features are first. The features in each part are in alphabetical
order.

The `GUI` and `Audio` lines show the renderer name and the SDL library version. They are absent in
console mode. If SDL3 did not start and SDL2 started instead, the line ends with `(fallback, ...)`
and the reason that SDL3 failed.

Example with the `-features` flag:

```text
Run summary
  Mapper:     682 - Rainbow
  Submapper:  0
  PRG ROM:    512 KB
  CHR ROM:    0 bytes
  PRG RAM:    32 KB
  CHR RAM:    32 KB
  Mirroring:  horizontal
  Battery:    no
  GUI:        sdl3 3.4.2
  Audio:      sdl3 3.4.2

PPU features
  [x] Background rendering
  [x] Sprite rendering
  [ ] 8x16 sprites

Mapper features
  [x] CHR banking
  [x] Extended attributes
  [x] PRG banking
  [ ] Scanline IRQ
```

## Controls

Default GUI controls:

- Arrow keys: D-pad
- `Z`: A button
- `X`: B button
- `Enter`: Start
- `Backspace`: Select

## Console Mode

Console mode disables GUI setup and is useful for test ROMs, tracing, and automated runs:

```bash
nesgoemu -c game.nes
```

For debugger endpoints, batch testing, and profiling examples, see [advanced-usage.md](advanced-usage.md).

## Audio

GUI mode plays sound through the SDL3 audio renderer and uses the SDL2 audio
renderer if SDL3 does not start. Audio output needs SDL3 or SDL2 runtime
libraries, like the GUI. Console mode does not open an audio device.

Mute the audio output:

```bash
nesgoemu -m game.nes
```

Use `-m` when the system has no audio device. Without the flag, a failing audio
device stops the emulator with an error message.

The emulator writes 44100 Hz mono samples. The sample queue holds about 93 ms of
audio. If the emulator runs late, the output writes silence. If it runs ahead,
the oldest samples are dropped.

Record ten seconds of audio from power-on without an audio device:

```bash
nesgoemu -wav capture.wav -audio-frames 441000 game.nes
```

The output is a mono 16-bit WAV file at 44100 Hz. The output path must not
exist. Recording uses emulated time, with no controller input or battery save.
It stops after the specified number of sample frames. See the
[audio review](audio-review.md) for comparison settings and accuracy limits.
