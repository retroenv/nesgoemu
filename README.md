# nesgoemu - a pure Go NES emulator

[![Build status](https://github.com/retroenv/nesgoemu/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/retroenv/nesgoemu/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/retroenv/nesgoemu)
[![Go Report Card](https://goreportcard.com/badge/github.com/retroenv/nesgoemu)](https://goreportcard.com/report/github.com/retroenv/nesgoemu)
[![codecov](https://codecov.io/gh/retroenv/nesgoemu/branch/main/graph/badge.svg?token=NS5UY28V3A)](https://codecov.io/gh/retroenv/nesgoemu)

## Installation

```bash
go install github.com/retroenv/nesgoemu@latest
```

**Requirements:**
- Go 1.22 or later
- SDL2 runtime libraries for GUI mode; console mode does not require SDL2

To build from source:

```bash
git clone https://github.com/retroenv/nesgoemu.git
cd nesgoemu
go install .
```

## Overview

nesgoemu is a Nintendo Entertainment System (NES) emulator written in Go. It runs iNES ROMs,
uses the MOS 6502 implementation from [retrogolib](https://github.com/retroenv/retrogolib), and
keeps emulator code organized around NES hardware subsystems.

The emulator can run with an SDL-based graphical interface for interactive play, or in console mode
for tracing, automated test ROM runs, and debugger-driven development workflows.

### Key Design Principles

- **Hardware-focused packages**: CPU bus integration, PPU, APU, controllers, memory, and mappers are separated by NES subsystem
- **retrogolib reuse**: CPU, cartridge, build metadata, GUI, SDL, and input support come from shared retroenv libraries
- **CLI-first operation**: ROM loading, tracing, debug server startup, and headless execution are exposed through command-line flags
- **Debuggable execution**: Console tracing and HTTP debugger endpoints support emulator development and test ROM analysis
- **Portable builds**: The Go build does not require CGO; GUI mode needs SDL2 runtime libraries

## Emulator Support

### Current Support

- **CPU**: MOS 6502 execution through `retrogolib/arch/cpu/m6502`
- **PPU**: Registers, addressing, palettes, nametables, sprites, tiles, render state, and screen output
- **APU**: Register and audio-unit structure
- **Input**: NES controller state and default keyboard mapping
- **Debugging**: CPU tracing plus HTTP endpoints for CPU, mapper, palette, and nametable state
- **Mappers**: NROM, MMC1, UxROM, CNROM, AxROM, UNROM-512, UN1ROM, GTROM, and UxROM AND variants

See [docs/architecture.md](docs/architecture.md) for mapper IDs, runtime flow, and package details.

## Package Overview

    ├─ main.go                  command-line entry point and runtime option parsing
    ├─ docs                     usage, architecture, GUI, and development references
    ├─ internal/testroms        validation ROM fixtures and expected traces
    ├─ pkg/apu                  APU register and audio-unit structure
    ├─ pkg/bus                  CPU, PPU, controller, mapper, and cartridge interconnects
    ├─ pkg/controller           NES controller state and button mapping
    ├─ pkg/mapper               mapper selection, mapper base helpers, and mapper implementations
    ├─ pkg/memory               RAM and memory access helpers
    ├─ pkg/nes                  system orchestration, input, tracing, GUI toggles, and debugger setup
    └─ pkg/ppu                  PPU registers, rendering, palettes, sprites, nametables, and tiles

## Quick Start

### CLI Usage

Run a NES ROM with the default GUI:

```bash
nesgoemu game.nes
```

Run without the GUI:

```bash
nesgoemu -c game.nes
```

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

Show command usage:

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

## Development

```bash
make build
make lint
make test
```

See [docs/development.md](docs/development.md) for coverage, linter installation, and contribution workflow.

## Documentation

- [docs/usage.md](docs/usage.md) - Command-line flags, controls, and common run modes.
- [docs/advanced-usage.md](docs/advanced-usage.md) - Debugging, tracing, automation, test ROMs, and profiling.
- [docs/architecture.md](docs/architecture.md) - Emulator components, mapper support, and package layout.
- [docs/development.md](docs/development.md) - Build, test, lint, and contribution workflow.
- [docs/gui.md](docs/gui.md) - SDL2 setup and console-mode details.

## License

This project is licensed under the Apache License Version 2.0 - see the [LICENSE](LICENSE) file for details.
