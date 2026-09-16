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
| `pkg/mapper` | Mapper construction |
| `pkg/mapper/mapperbase` | Shared banking, hooks, and nametable support |
| `pkg/mapper/mapperdb` | Mapper catalog and hardware-family implementations |
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

See the [supported mapper tables](../README.md#supported-mappers) for mapper IDs
and hardware documentation.

`pkg/mapper` creates the shared mapper base and delegates mapper selection to
the catalog in `pkg/mapper/mapperdb`. Each hardware family has its own package
under the catalog. Mapper implementation files use a four-digit mapper number,
such as `mapper0001_mmc1.go`.

### Optional Mapper Capabilities

All mappers implement `bus.Mapper`. A mapper can also implement one or more
small interfaces from `pkg/bus/mapper_capabilities.go`:

| Interface | Behavior |
| --- | --- |
| `BatteryMapper` | Loads and saves persistent cartridge data while emulation is stopped. |
| `CPUClocker` | Receives each elapsed CPU cycle before the system advances the PPU. |
| `MapperResetter` | Resets mapper registers and bank state before the PPU and CPU reset. |
| `PCMReader` | Supplies the value for a CPU read from `$4011` instead of the APU. |
| `PPUTicker` | Receives the current cycle, scanline, and rendering state after each PPU clock advances. |
| `SpriteExtFetcher` | Receives the OAM index and sprite height before each sprite pattern read. An index of `-1` identifies an empty sprite slot. The PPU clears the active entry with `(-1, 0)` after the read. |
| `TimingEnabler` | Enables detailed bus timing once when the PPU connects to the mapper. |

Components detect these interfaces at run time. A mapper that does not implement
an optional interface keeps the standard emulator behavior.

## Related Documentation

- [usage.md](usage.md) - Runtime flags and controls.
- [advanced-usage.md](advanced-usage.md) - Debugger, tracing, and profiling workflows.
- [development.md](development.md) - Local development workflow.
