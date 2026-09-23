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

The tables give each changed file a role. The **Slice** column shows a possible
review group. Some files contain changes for more than one slice; see the split
notes below.

| Slice | Status | File | Role |
| --- | --- | --- | --- |
| Foundation | Modified | `go.mod` | Select retrogolib with CPU bus-cycle and audio APIs. |
| Foundation | Modified | `go.sum` | Record checksums for that dependency revision. |
| Foundation | Modified | `pkg/bus/bus.go` | Add an OAM request interface and combine APU and mapper IRQ sources. |
| Foundation | Modified | `pkg/bus/ppu.go` | Require an APU step method on the bus interface. |
| Foundation | Modified | `pkg/mapper/mapperbase/base.go` | Route mapper IRQ changes through the shared bus line. |
| Foundation | Modified | `pkg/nes/system.go` | Clock devices from the CPU cycle hook and run DMA bus actions. Also wires the APU and renderer errors. |
| Foundation | Modified | `pkg/nes/reset.go` | Clear pending DMA and reset the APU before the CPU reads its reset vector. |
| Foundation | Modified | `pkg/nes/system_test.go` | Check per-cycle APU, PPU, and mapper clocks. |
| Foundation | Modified | `pkg/nes/rainbow_test.go` | Adapt mapper IRQ timing test to APU frame IRQ and instruction sampling. |
| Foundation | Modified | `internal/testroms/nestest/nestest_no_ppu.log` | Update the expected trace for the new APU status read. |
| DMA | Added | `pkg/nes/dma.go` | Schedule OAM and DMC bus ownership, stalls, alignment, and transfer order. |
| DMA | Added | `pkg/nes/dma_test.go` | Check OAM transfer cycles and delayed DMC fetch. |
| DMA | Modified | `pkg/ppu/register.go` | Queue OAM DMA through the bus scheduler, with the existing direct path for an unconnected scheduler. |
| DMA | Modified | `pkg/memory/memory.go` | Track bus cycles and retain controller output on adjacent DMA reads; also correct `$4016` and `$4017` strobing. |
| DMA | Modified | `pkg/memory/memory_test.go` | Check adjacent controller reads, strobing, and APU register routing. |
| DMA | Modified | `pkg/controller/controller.go` | Return one after the eight button bits. |
| DMA | Modified | `pkg/controller/controller_test.go` | Check post-button reads and strobe reset. |
| Mapper compatibility | Modified | `pkg/mapper/mapper.go` | Allocate 8 KiB of CHR RAM for legacy iNES cartridges without CHR ROM. |
| Mapper compatibility | Modified | `pkg/mapper/mapper_test.go` | Check reads and writes at both ends of that CHR RAM. |

| Slice | Status | File | Role |
| --- | --- | --- | --- |
| APU core | Modified | `pkg/apu/apu.go` | Connect channels, frame clocks, sample output, DMC requests, IRQ state, and reset. |
| APU core | Modified | `pkg/apu/register.go` | Decode channel registers, status bits, frame counter writes, and write observations. |
| APU core | Added | `pkg/apu/apu_test.go` | Check status, IRQ, sample drain, write observations, and reset. |
| APU core | Added | `pkg/apu/dmc/dmc.go` | Implement DMC output, sample reader requests, loop, and IRQ state. |
| APU core | Added | `pkg/apu/dmc/dmc_test.go` | Check DMC DAC, reader, timing, and IRQ behavior. |
| APU core | Added | `pkg/apu/envelope/envelope.go` | Implement envelope start, decay, loop, and constant volume. |
| APU core | Added | `pkg/apu/envelope/envelope_test.go` | Check envelope decay and write timing. |
| APU core | Added | `pkg/apu/framecounter/framecounter.go` | Clock quarter and half frames and manage frame IRQ timing. |
| APU core | Added | `pkg/apu/framecounter/framecounter_test.go` | Check sequence events, write delay, and IRQ edges. |
| APU core | Added | `pkg/apu/lengthcounter/lengthcounter.go` | Implement length table and delayed reload or halt writes. |
| APU core | Added | `pkg/apu/lengthcounter/lengthcounter_test.go` | Check length clocks and reload conflicts. |
| APU core | Added | `pkg/apu/noise/noise.go` | Generate noise with period and mode controls. |
| APU core | Added | `pkg/apu/noise/noise_test.go` | Check noise shift sequence and register effects. |
| APU core | Added | `pkg/apu/pulse/pulse.go` | Generate the two pulse channels with duty, envelope, length, and sweep. |
| APU core | Added | `pkg/apu/pulse/pulse_test.go` | Check pulse period, duty, and output controls. |
| APU core | Added | `pkg/apu/sweep/sweep.go` | Compute pulse sweep targets, mute state, and reload timing. |
| APU core | Added | `pkg/apu/sweep/sweep_test.go` | Check sweep updates and channel-specific negate behavior. |
| APU core | Added | `pkg/apu/triangle/triangle.go` | Generate triangle output with linear and length counters. |
| APU core | Added | `pkg/apu/triangle/triangle_test.go` | Check triangle sequence, gating, and timer behavior. |
| Audio signal | Added | `pkg/apu/mixer/mixer.go` | Combine five channels with nonlinear NES mixing formulas. |
| Audio signal | Added | `pkg/apu/mixer/mixer_test.go` | Check mixer values and channel contributions. |
| Audio signal | Added | `pkg/apu/sampler/sampler.go` | Convert the NTSC CPU-rate signal to output frames with a windowed-sinc filter. |
| Audio signal | Added | `pkg/apu/sampler/sampler_test.go` | Check output count, frequency gain, and alias rejection. |
| Audio signal | Added | `pkg/apu/filter/filter.go` | Apply high-pass and low-pass output filters. |
| Audio signal | Added | `pkg/apu/filter/filter_test.go` | Check filter response and state. |
| Audio signal | Added | `pkg/apu/output/output.go` | Hold signed 16-bit samples in a bounded queue and report queue statistics. |
| Audio signal | Added | `pkg/apu/output/output_test.go` | Check sample conversion, queue limits, and drain behavior. |

| Slice | Status | File | Role |
| --- | --- | --- | --- |
| Playback | Added | `pkg/nes/audio.go` | Expose the retrogolib audio backend and start SDL2 playback. |
| Playback | Added | `pkg/nes/audio_test.go` | Check format, callback, playback errors, IRQ independence, and generated tone. |
| Playback | Modified | `pkg/nes/option.go` | Add the option that disables playback. |
| Playback | Modified | `pkg/nes/start.go` | Start and stop audio with the renderer and pass audio errors to its loop. |
| Playback | Modified | `pkg/nes/persistence_test.go` | Adapt the renderer test to the audio error channel. |
| Recording | Added | `pkg/nes/recording.go` | Write a fixed number of queued or newly emulated samples as mono WAV. |
| Recording | Added | `pkg/nes/recording_test.go` | Check WAV format, chunk continuity, cancellation, and write failures. |
| Playback, recording | Modified | `main.go` | Add `-m`, `-wav`, and `-audio-frames`; wire SDL2 audio and create a new WAV file. |

| Slice | Status | File | Role |
| --- | --- | --- | --- |
| ROM validation | Modified | `.gitignore` | Track APU `.nes` files while other ROM files remain ignored. |
| ROM validation | Modified | `Makefile` | Set a configurable two-minute timeout for test and coverage targets. |
| ROM validation | Added | `internal/testroms/apu/apu_test.go` | Run modern and 2005 APU ROMs with their distinct result protocols and reset handling. |
| ROM validation | Added | `internal/testroms/apu/README.md` | Describe ROM provenance, hashes, and test protocol. |
| ROM validation | Added | `internal/testroms/apu/SHA256SUMS` | Record fixture hashes. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/upstream-readme.txt` | Preserve upstream 2005 suite instructions. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/upstream-tests.txt` | Preserve upstream 2005 test descriptions. |
| ROM validation | Added | `internal/testroms/apu/apu_reset/upstream-readme.txt` | Preserve upstream reset suite instructions. |
| ROM validation | Added | `internal/testroms/apu/apu_test/upstream-readme.txt` | Preserve upstream modern suite instructions. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/01.len_ctr.nes` | Check length counter behavior. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/02.len_table.nes` | Check length table values. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/03.irq_flag.nes` | Check frame IRQ flag behavior. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/04.clock_jitter.nes` | Check frame clock phase variation. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/05.len_timing_mode0.nes` | Check length timing in four-step mode. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/06.len_timing_mode1.nes` | Check length timing in five-step mode. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/07.irq_flag_timing.nes` | Check frame IRQ flag timing. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/08.irq_timing.nes` | Check frame IRQ timing. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/09.reset_timing.nes` | Check reset timing. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/10.len_halt_timing.nes` | Check length halt timing. |
| ROM validation | Added | `internal/testroms/apu/apu_2005/11.len_reload_timing.nes` | Check length reload timing. |
| ROM validation | Added | `internal/testroms/apu/apu_reset/4015_cleared.nes` | Check status clearing during reset. |
| ROM validation | Added | `internal/testroms/apu/apu_reset/4017_timing.nes` | Check frame counter timing after reset. |
| ROM validation | Added | `internal/testroms/apu/apu_reset/4017_written.nes` | Check frame counter write state after reset. |
| ROM validation | Added | `internal/testroms/apu/apu_reset/irq_flag_cleared.nes` | Check IRQ flag clearing during reset. |
| ROM validation | Added | `internal/testroms/apu/apu_reset/len_ctrs_enabled.nes` | Check length counter enable state after reset. |
| ROM validation | Added | `internal/testroms/apu/apu_reset/works_immediately.nes` | Check APU operation immediately after reset. |
| ROM validation | Added | `internal/testroms/apu/apu_test/1-len_ctr.nes` | Check modern length counter behavior. |
| ROM validation | Added | `internal/testroms/apu/apu_test/2-len_table.nes` | Check modern length table behavior. |
| ROM validation | Added | `internal/testroms/apu/apu_test/3-irq_flag.nes` | Check modern frame IRQ flag behavior. |
| ROM validation | Added | `internal/testroms/apu/apu_test/4-jitter.nes` | Check modern frame clock jitter. |
| ROM validation | Added | `internal/testroms/apu/apu_test/5-len_timing.nes` | Check modern length counter timing. |
| ROM validation | Added | `internal/testroms/apu/apu_test/6-irq_flag_timing.nes` | Check modern IRQ flag timing. |
| ROM validation | Added | `internal/testroms/apu/apu_test/7-dmc_basics.nes` | Check DMC basics. |
| ROM validation | Added | `internal/testroms/apu/apu_test/8-dmc_rates.nes` | Check DMC rate table. |

| Slice | Status | File | Role |
| --- | --- | --- | --- |
| Documentation | Modified | `README.md` | Advertise SDL2 audio playback. |
| Documentation | Modified | `docs/architecture.md` | Describe APU units, clocks, signal path, and audio backend. |
| Documentation | Added | `docs/audio-review.md` | Record accuracy findings, measurements, test results, and open limits. |
| Documentation | Modified | `docs/development.md` | Describe APU architecture and hardware ROM test procedure. |
| Documentation | Modified | `docs/gui.md` | Describe GUI audio and the mute flag. |
| Documentation | Modified | `docs/usage.md` | Describe mute and WAV recording options and output behavior. |

## Possible commit slices

1. **Foundation and DMA:** Dependency update, CPU cycle clocks, IRQ aggregation,
   DMC/OAM scheduler, and bus integration. This is a coupled change. The
   controller corrections fit here because the DMA checks expose them.
2. **APU channels and signal path:** Channels, frame and length timing, mixer,
   sampler, filters, queue, and focused tests. This slice needs the foundation
   APIs. A separate signal-path commit may be possible after the APU core.
3. **Playback and recording:** SDL2 output, mute control, deterministic WAV
   writer, command-line flags, and tests. Playback and recording can be separate
   commits if the shared APU sample output lands first.
4. **ROM validation and documentation:** Fixture runner, ROMs, hashes, timeout,
   user guides, and accuracy review. Keep the runner with the fixtures it uses.
   The legacy CHR RAM fix can be its own small commit with its test.

`pkg/nes/system.go` mixes cycle timing, DMA, APU setup, and renderer error
handling. `main.go` mixes playback and recording. `pkg/apu/apu.go` and
`pkg/apu/apu_test.go` mix channel behavior with output APIs. `pkg/nes/audio_test.go`
mixes playback, IRQ, and signal tests. Those files need hunk-level separation
for the suggested commit boundaries. The slices are review candidates, not a
claim that each can build or pass tests without further checks.

## Verification

- Recorded in `docs/audio-review.md`: `make lint` and `make test` passed during
  the audio review. The review reports that all 25 APU ROM fixtures passed.
- Not run for this summary: build, lint, and test commands. This update changes
  documentation only.

## Notes

- This summary uses the clean-worktree fallback range `main...HEAD`. Local
  `main` exists. The range contains 94 source, test, fixture, and documentation
  files after this generated summary is excluded.
- Audio timing is NTSC-only. Expansion audio is not implemented.
- The audio review records remaining limits for NMI/IRQ overlap, adjacent
  PPUDATA reads, DMC abort quirks, internal-register DMA conflicts, and live SDL
  queue measurements.
