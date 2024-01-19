# nesgoemu - Emulator for NES ROMs

[![Build status](https://github.com/retroenv/nesgoemu/actions/workflows/go.yaml/badge.svg?branch=main)](https://github.com/retroenv/nesgoemu/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/retroenv/nesgoemu)
[![Go Report Card](https://goreportcard.com/badge/github.com/retroenv/nesgoemu)](https://goreportcard.com/report/github.com/retroenv/nesgoemu)
[![codecov](https://codecov.io/gh/retroenv/nesgoemu/branch/main/graph/badge.svg?token=NS5UY28V3A)](https://codecov.io/gh/retroenv/nesgoemu)


nesgoemu allows you to emulate ROMs for the Nintendo Entertainment System (NES).

## Features

* Offers the GUI in SDL or OpenGL mode
* Can be used headless without a GUI
* Supports outputting of CPU traces
* Supports undocumented 6502 CPU opcodes

## Installation

Your system needs to have a recent [Golang](https://go.dev/) version installed.

Check [GUI installation](https://github.com/retroenv/nesgoemu/blob/main/docs/gui.md) to set up the GUI dependencies.

Install the latest stable version by running:

```
go install github.com/retroenv/nesgoemu@latest
```

The latest development version can be installed using:

```
git clone https://github.com/retroenv/nesgoemu.git
cd nesgoemu
go build .
# use the dev version:
./nesgoemu  
```

## Usage

Emulate a ROM:

```
nesgoemu example.nes
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
