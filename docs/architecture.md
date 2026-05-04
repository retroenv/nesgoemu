# Architecture

nesgoemu organizes the emulator around NES hardware components. The top-level command loads a ROM,
creates a cartridge, configures runtime options, and starts the NES system orchestration package.

## Emulator Components

Core NES components currently implemented in the repository:

- CPU bus integration backed by `retrogolib/arch/cpu/m6502`
- PPU registers, addressing, palettes, nametables, sprites, tiles, render state, and screen output
- APU register structure
- Controller input handling
- CPU, PPU, memory, cartridge, and mapper bus wiring
- Web debugger endpoints for CPU, mapper, palette, and nametable inspection

## Runtime Flow

```text
ROM file -> cartridge loader -> NES system -> bus -> CPU/PPU/APU/controllers -> GUI or console output
```

The `pkg/nes` package coordinates startup. The `pkg/bus` package connects the CPU, PPU,
controllers, cartridge, mapper, and memory. Mapper implementations translate cartridge reads and
writes into the correct PRG, CHR, and nametable behavior for the loaded ROM.

## Mapper Support

Supported mapper IDs are currently:

- `0`: NROM
- `1`: MMC1
- `2`: UxROM OR variant
- `3`: CNROM
- `7`: AxROM
- `30`: UNROM-512
- `94`: UN1ROM
- `111`: GTROM
- `180`: UxROM AND variant

## Package Overview

    ├─ main.go                  command-line entry point and runtime option parsing
    ├─ pkg/apu                  APU register and audio-unit structure
    ├─ pkg/bus                  CPU, PPU, controller, mapper, and cartridge interconnects
    ├─ pkg/controller           NES controller state and button mapping
    ├─ pkg/mapper               mapper selection, mapper base helpers, and mapper implementations
    ├─ pkg/memory               RAM and memory access helpers
    ├─ pkg/nes                  system orchestration, input, tracing, GUI toggles, and debugger setup
    ├─ pkg/nes/debugger         HTTP debugger handlers
    ├─ pkg/ppu                  PPU registers, rendering, palettes, sprites, nametables, and tiles
    └─ internal/testroms        validation ROM fixtures and expected traces

## Related Documentation

- [usage.md](usage.md) - Runtime flags and controls.
- [advanced-usage.md](advanced-usage.md) - Debugger, tracing, and profiling workflows.
- [development.md](development.md) - Local development workflow.
