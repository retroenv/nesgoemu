# nesgoemu - a pure Golang NES Emulator

[![Build status](https://github.com/retroenv/nesgoemu/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/retroenv/nesgoemu/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/retroenv/nesgoemu)
[![Go Report Card](https://goreportcard.com/badge/github.com/retroenv/nesgoemu)](https://goreportcard.com/report/github.com/retroenv/nesgoemu)
[![codecov](https://codecov.io/gh/retroenv/nesgoemu/branch/main/graph/badge.svg?token=NS5UY28V3A)](https://codecov.io/gh/retroenv/nesgoemu)


nesgoemu is a Nintendo Entertainment System (NES) emulator written in Go. It aims for accurate hardware emulation while maintaining clean, maintainable code.

The emulator runs NES ROMs and includes debugging tools for development work. It can run in console mode with no external dependencies, or with a SDL-based GUI for full gaming experience.

## Features

### NES Hardware Emulation
* 6502 CPU emulation using retrogolib
* PPU (Picture Processing Unit) with basic rendering
* APU (Audio Processing Unit) - basic structure implemented
* Memory mappers: NROM, MMC1, CNROM, UxROM, AxROM, GTROM, UNROM512

### Development Tools
* Web-based debugger with HTTP endpoints
* CPU instruction tracing to console/file
* Execution control (entry point, stop address)
* Memory and register inspection via debugger

### Implementation
* Written in pure Go with no CGO dependencies
* Console mode available (no GUI dependencies)
* Cross-platform support where Go and SDL2 are available

## Installation

### Requirements

* Go 1.22 or later
* SDL2 libraries (optional, for GUI mode)

### Install from Go

```bash
go install github.com/retroenv/nesgoemu@latest
```

This installs the binary to your `GOPATH/bin` directory.

### Build from Source

```bash
git clone https://github.com/retroenv/nesgoemu.git
cd nesgoemu
go install .
```

### GUI Dependencies

The emulator can run in console mode without any additional libraries. For the SDL-based GUI, you'll need to install SDL2 development libraries.

See [docs/gui.md](docs/gui.md) for detailed platform-specific installation instructions.

## Usage

### Basic Usage

Run a NES ROM:

```bash
nesgoemu game.nes
```

This opens the game in a SDL window with the default controls.

### Controls

- Arrow keys: D-Pad
- Z: A button
- X: B button  
- Enter: Start
- Backspace: Select

## Documentation

- [docs/advanced-usage.md](docs/advanced-usage.md) - Debugging, automation, and advanced workflows
- [docs/development.md](docs/development.md) - Development guide, building, testing, and contributing
- [docs/gui.md](docs/gui.md) - GUI setup and SDL2 installation

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- NESDev community for NES hardware documentation
- SDL2 for cross-platform multimedia support
