# Architecture

`main.go` loads the ROM, configures options, and starts `pkg/nes`.

```text
ROM file -> cartridge loader -> NES system -> bus -> CPU/PPU/APU/controllers -> GUI or console output
```

The bus connects the hardware components. Mappers translate cartridge reads and
writes into PRG, CHR, and nametable accesses.

## Packages

| Package | Responsibility |
| --- | --- |
| `pkg/nes` | Startup, input, tracing, and GUI or console output |
| `pkg/bus` | Connections between hardware components |
| `pkg/memory` | CPU memory access |
| `pkg/mapper` | Mapper selection and bank switching |
| `pkg/ppu` | PPU registers, memory, palettes, nametables, sprites, tiles, and rendering |
| `pkg/apu` | APU registers; audio output is not implemented |
| `pkg/controller` | Controller state and button mapping |
| `pkg/nes/debugger` | HTTP access to CPU, mapper, palette, and nametable state |
| `internal/testroms` | Test ROMs and expected traces |

## Shared Library and ROM Loading

[retrogolib](https://github.com/retroenv/retrogolib) provides the CPU core
(`arch/cpu/cpu6502`), ROM loading and saving, system constants, application
lifecycle, input types, and SDL support.

`cartridge.LoadFile` reads iNES and NES 2.0 files. Pass the returned cartridge to
`nes.WithCartridge`. The system bus retains that cartridge, so ROM metadata has
one owner. Retrogolib tests the file format; `pkg/nes` tests emulator integration.

`Cartridge.NES2.RAMSizes` gives volatile and nonvolatile PRG and CHR RAM sizes
in bytes. Zero means no memory. For legacy iNES, `Cartridge.NES2` is nil and
`Cartridge.RAM` holds the PRG RAM bank count. This keeps unspecified legacy sizes
distinct from explicit zero sizes. See the [NES 2.0 specification](https://www.nesdev.org/wiki/NES_2.0).

## Mapper Support

ROM header support is separate from mapper execution. Registered mappers are:

- `0`: NROM
- `1`: MMC1
- `2`: UxROM OR variant
- `3`: CNROM
- `7`: AxROM
- `30`: UNROM-512
- `94`: UN1ROM
- `111`: GTROM
- `180`: UxROM AND variant

## Related Documentation

- [usage.md](usage.md) - Runtime flags and controls.
- [advanced-usage.md](advanced-usage.md) - Debugger, tracing, and profiling workflows.
- [development.md](development.md) - Local development workflow.
