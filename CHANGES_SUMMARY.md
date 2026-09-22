# Change Summary

## Overview

The branch adds NTSC APU emulation, SDL2 audio playback, and deterministic WAV
recording. It also adds CPU-cycle bus timing and shared DMA behavior that the
APU, OAM, interrupts, controllers, and open-bus logic require.

## Changes

- **APU emulation:** Add the two pulse channels, triangle channel, noise
  channel, DMC, envelopes, sweep units, length counters, and frame counter.
  Implement register status, interrupt, reset, and channel timing behavior.
- **Audio pipeline:** Mix the five channels with the nonlinear NES formulas.
  Convert the NTSC CPU-rate signal to 44100 Hz with a windowed-sinc filter.
  Apply the analog output filters and store signed 16-bit mono samples in a
  bounded, thread-safe queue.
- **Playback and recording:** Connect GUI mode to the retrogolib SDL2 audio
  backend. Add `-m` to disable playback. Add `-wav` and `-audio-frames` to write
  deterministic WAV output without an audio device. Report audio-device errors
  through the renderer loop.
- **Bus and DMA timing:** Clock the APU, PPU, mapper, and memory accesses from
  each CPU bus cycle. Add one scheduler for DMC and OAM DMA, including halt,
  alignment, priority, and transfer timing. Keep APU and mapper IRQ sources
  independent before they drive the CPU IRQ line.
- **Compatibility fixes:** Preserve controller output during adjacent DMA
  reads, return one after the eight controller button bits, strobe both
  controllers only through `$4016`, and provide 8 KiB of CHR RAM for legacy
  iNES cartridges that have no CHR ROM.
- **Validation assets:** Add 25 APU hardware-test ROMs, fixed SHA-256 hashes,
  protocol-aware runners, and unit and integration tests for channels,
  resampling, playback, recording, DMA, interrupts, reset, controller reads,
  and legacy CHR RAM.
- **Documentation and dependencies:** Document audio architecture, operation,
  accuracy measurements, known limits, and test procedures. Update retrogolib
  to the revision that provides CPU bus-cycle hooks and audio support. Increase
  the configurable full-test timeout to two minutes for the hardware ROM tests.

## Files

| Status | Files | Purpose |
| --- | --- | --- |
| Added | `pkg/apu/{dmc,envelope,filter,framecounter,lengthcounter,mixer,noise,output,pulse,sampler,sweep,triangle}/`, `pkg/apu/apu_test.go` | Implement and test the APU channels, timing units, mixer, resampler, filters, and sample queue. |
| Added | `pkg/nes/audio.go`, `pkg/nes/audio_test.go`, `pkg/nes/dma.go`, `pkg/nes/dma_test.go`, `pkg/nes/recording.go`, `pkg/nes/recording_test.go` | Add audio playback, shared DMA scheduling, and deterministic WAV recording. |
| Added | `internal/testroms/apu/` | Add 25 fixed hardware ROM fixtures, hashes, upstream notes, and the ROM test runner. |
| Added | `docs/audio-review.md` | Record accuracy evidence, measured output, known limits, and reproduction commands. |
| Modified | `pkg/apu/apu.go`, `pkg/apu/register.go`, `pkg/bus/bus.go`, `pkg/bus/ppu.go`, `pkg/nes/option.go`, `pkg/nes/reset.go`, `pkg/nes/start.go`, `pkg/nes/system.go`, `pkg/nes/system_test.go`, `pkg/nes/persistence_test.go`, `pkg/nes/rainbow_test.go`, `pkg/ppu/register.go` | Integrate APU clocks, register access, IRQ aggregation, DMA, reset, playback, and system lifecycle behavior. |
| Modified | `pkg/controller/controller.go`, `pkg/controller/controller_test.go`, `pkg/memory/memory.go`, `pkg/memory/memory_test.go`, `pkg/mapper/mapper.go`, `pkg/mapper/mapper_test.go`, `pkg/mapper/mapperbase/base.go` | Correct controller, open-bus, IRQ, and legacy CHR RAM behavior found during DMA validation. |
| Modified | `main.go`, `go.mod`, `go.sum`, `.gitignore`, `Makefile`, `internal/testroms/nestest/nestest_no_ppu.log` | Add command-line audio controls, update retrogolib, track ROM fixtures, extend test timeouts, and update the expected APU status trace. |
| Modified | `README.md`, `docs/architecture.md`, `docs/development.md`, `docs/gui.md`, `docs/usage.md` | Document playback, recording, architecture, test fixtures, and user options. |

## Verification

- Passed: `make lint`.
- Passed: `make test` — includes the race-enabled full suite and the 25 APU
  hardware ROM fixtures.
- Passed: `go test ./pkg/memory -count=1`.
- Passed: `go test ./pkg/mapper -count=1`.
- Passed: `go test ./pkg/ppu -count=1`.
- Passed: `go test ./pkg/nes -count=1`.
- Passed: `go test ./internal/testroms/openbus -count=1`.

## Notes

- This summary uses the clean-worktree fallback range `main...HEAD`. Local
  `main` exists. The range contains 94 files, 5,630 insertions, and 51
  deletions.
- Audio timing is NTSC-only. Expansion audio is not implemented.
- The audio review records remaining limits for NMI/IRQ overlap, adjacent
  PPUDATA reads, DMC abort quirks, internal-register DMA conflicts, and live SDL
  queue measurements.
