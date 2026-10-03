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
  -m    mute audio output
  -s int
        stop execution at address (default -1)
  -t    print CPU tracing
  -wav string
        write deterministic audio to a new WAV file
  -audio-frames uint
        number of sample frames to record with -wav (default 441000)
```

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

The emulator prints a summary when it stops. The summary shows the ROM details and the hardware
features that the run used. A used feature has an `x` mark. An unused feature has no mark.

```text
Run summary
  Mapper:     682 Rainbow
  PRG ROM:    512 KB
  CHR ROM:    0 bytes
  PRG RAM:    32 KB
  CHR RAM:    32 KB
  Mirroring:  horizontal
  Battery:    no

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

GUI mode plays sound through the SDL2 audio renderer. Audio output needs SDL2
runtime libraries, like the GUI. Console mode does not open an audio device.

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
