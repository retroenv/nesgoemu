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
| `pkg/apu` | APU registers, sound channels, frame counter, mixing, and audio output |
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

## Audio

The APU contains two pulse channels, a triangle channel, a noise channel, a
delta modulation channel (DMC), and the frame counter. Each unit has its own
package under `pkg/apu`. The parent package decodes the registers, clocks the
units, aggregates the interrupts, and mixes the channel levels.

`pkg/nes` steps the APU once per CPU cycle. The pulse, noise, and DMC timers
advance once per APU cycle, which is every second CPU cycle. The triangle timer
advances on every CPU cycle.

The mixed signal goes to the sampler, which converts it to mono 16-bit samples
at 44100 Hz. The output stage then applies the filter chain of the NES and holds
the samples for playback. `pkg/nes` implements the `audio.Backend` interface of
retrogolib and fills the playback buffer. A full sample queue drops its oldest
samples to keep latency bounded. An empty queue writes silence.

Audio output uses the SDL2 audio renderer from retrogolib. It is active in GUI
mode only. The `-m` flag disables it.

The implementation follows these NESdev wiki pages:

| Topic | Page |
| --- | --- |
| APU overview, register map, status register | https://www.nesdev.org/wiki/APU |
| Pulse channels | https://www.nesdev.org/wiki/APU_Pulse |
| Triangle channel | https://www.nesdev.org/wiki/APU_Triangle |
| Noise channel | https://www.nesdev.org/wiki/APU_Noise |
| DMC channel | https://www.nesdev.org/wiki/APU_DMC |
| Envelope, sweep, and length counter units | https://www.nesdev.org/wiki/APU_Envelope, https://www.nesdev.org/wiki/APU_Sweep, https://www.nesdev.org/wiki/APU_Length_Counter |
| Frame counter and frame interrupt | https://www.nesdev.org/wiki/APU_Frame_Counter |
| Mixer and output filters | https://www.nesdev.org/wiki/APU_Mixer |
| Power-up register values | https://www.nesdev.org/wiki/CPU_power_up_state#APU |
| NTSC clock rates | https://www.nesdev.org/wiki/Cycle_reference_chart |

Register writes take effect when the instruction completes. The emulator does
not model intra-instruction bus timing, and it does not model expansion audio.

## Related Documentation

- [usage.md](usage.md) - Runtime flags and controls.
- [advanced-usage.md](advanced-usage.md) - Debugger, tracing, and profiling workflows.
- [development.md](development.md) - Local development workflow.
