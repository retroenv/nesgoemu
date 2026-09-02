# nesgoemu

[![CI](https://github.com/retroenv/nesgoemu/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/retroenv/nesgoemu/actions/workflows/go.yaml)
[![Codecov](https://codecov.io/gh/retroenv/nesgoemu/graph/badge.svg)](https://codecov.io/gh/retroenv/nesgoemu)
[![Release](https://img.shields.io/github/v/release/retroenv/nesgoemu)](https://github.com/retroenv/nesgoemu/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/retroenv/nesgoemu.svg)](https://pkg.go.dev/github.com/retroenv/nesgoemu)
[![License](https://img.shields.io/github/license/retroenv/nesgoemu)](LICENSE)
![LLM assisted: human reviewed](https://img.shields.io/badge/LLM%20assisted-human%20reviewed-6f42c1)

A pure Go Nintendo Entertainment System emulator with SDL2 graphics,
headless execution, CPU tracing, and HTTP debugging.

## Features

* **NES emulation** - Emulates the CPU, graphics, controllers, memory, and common cartridge mappers
* **Graphical and headless modes** - Runs interactively through SDL2 or without a window for automation
* **Development tools** - Provides CPU tracing, execution control, and an HTTP debugger
* **Portable builds** - Builds without CGO for Linux, macOS, and Windows

## Quick Start

### Installation

Download a binary for Linux, macOS, or Windows from
[Releases](https://github.com/retroenv/nesgoemu/releases), or install from
source with Go 1.22 or newer:

```bash
go install github.com/retroenv/nesgoemu@latest
```

GUI mode requires SDL2 runtime libraries. See the [GUI setup guide](docs/gui.md)
for platform-specific installation instructions. Console mode does not require SDL2.

### Basic Usage

Run an iNES ROM with the default graphical interface:

```bash
nesgoemu game.nes
```

Run without the graphical interface:

```bash
nesgoemu -c game.nes
```

See the [usage guide](docs/usage.md) for command-line options and controls,
the [advanced usage guide](docs/advanced-usage.md) for tracing and debugging,
and the [architecture guide](docs/architecture.md) for internals and mapper support.
