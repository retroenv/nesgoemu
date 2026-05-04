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
  -s int
        stop execution at address (default -1)
  -t    print CPU tracing
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
