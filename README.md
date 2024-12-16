# nesgoemu - a pure Golang NES Emulator

[![Build status](https://github.com/retroenv/nesgoemu/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/retroenv/nesgoemu/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/retroenv/nesgoemu)
[![Go Report Card](https://goreportcard.com/badge/github.com/retroenv/nesgoemu)](https://goreportcard.com/report/github.com/retroenv/nesgoemu)
[![codecov](https://codecov.io/gh/retroenv/nesgoemu/branch/main/graph/badge.svg?token=NS5UY28V3A)](https://codecov.io/gh/retroenv/nesgoemu)


nesgoemu is a Nintendo Entertainment System (NES) emulator.
It allows you to play your favorite classic NES games directly on your computer.

## Features

* Native Golang: Built entirely in Golang, ensuring a clean and maintainable codebase and making it easy to build and portable across platforms.
* Lightweight: No CGO dependency, resulting in a smaller binary size and faster build times.
* Flexible Interface: Runs without or with a SDL based user interface for streamlined usage.
* Advanced Debugging: Supports outputting of CPU traces and undocumented 6502 CPU opcodes for in-depth analysis.

## Installation

Your system needs to have a recent [Golang](https://go.dev/) version installed.

Check [GUI installation](https://github.com/retroenv/nesgoemu/blob/main/docs/gui.md) to set up the GUI dependencies.

Installation Options:

1. Stable Version:

```
go install github.com/retroenv/nesgoemu@latest
```

This installs the latest stable version and places the `nesgoemu` binary in your system's GOPATH/bin directory.

2. Development Version:

The latest development version can be installed using:

```
git clone https://github.com/retroenv/nesgoemu.git
cd nesgoemu
go install .
```

This builds and install the emulator from the latest code in the development branch.

## Usage

Emulate a ROM:

```
nesgoemu <your_rom_file.nes>
```

## Options

```
usage: nesgoemu [options] <file to emulate>

  -a string
    	listening address for the debug server to use (default "127.0.0.1:8080")
  -c	console mode, disable GUI
  -d	start built-in webserver for debug mode
  -e int
    	entrypoint to start the CPU (default -1)
  -s int
    	stop execution at address (default -1)
  -t	print CPU tracing
```
