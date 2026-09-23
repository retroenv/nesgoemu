# Change Summary

## Overview

The branch adds NTSC APU emulation, Rainbow mapper expansion audio, SDL2
playback, and deterministic WAV recording. It also adds CPU-cycle bus timing
and shared DMA behavior for the APU, OAM, interrupts, and controllers.

## Changes

- **APU emulation:** Add the two pulse channels, triangle channel, noise
  channel, DMC, envelopes, sweep units, length counters, and frame counter.
  Implement register status, interrupt, reset, and channel timing behavior.
- **Audio pipeline:** Mix the five channels with the nonlinear NES formulas.
  Convert the NTSC CPU-rate signal to 44100 Hz with a windowed-sinc filter.
  Apply the analog output filters and store signed 16-bit mono samples in a
  bounded, thread-safe queue.
- **Rainbow expansion audio:** Clock two mapper pulse channels and a saw
  channel, route their output into the APU mix, and expose IPCM reads and
  cycle-stamped register writes. Save their state in Rainbow snapshot format 3.
- **DMC telemetry:** Report each completed sample fetch with its APU cycle,
  source address, and CPU stall duration.
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
| Rainbow audio | Modified | `pkg/bus/mapper_capabilities.go` | Add the optional mapper expansion audio output interface. |
| Foundation | Modified | `pkg/bus/ppu.go` | Require an APU step method on the bus interface. |
| Rainbow audio | Modified | `pkg/feature/feature.go` | Add an expansion audio feature label. |
| Foundation | Modified | `pkg/mapper/mapperbase/base.go` | Route mapper IRQ changes through the shared bus line. |
| Foundation | Modified | `pkg/nes/system.go` | Clock devices from the CPU cycle hook and run DMA bus actions. Also wires the APU and renderer errors. |
| Foundation | Modified | `pkg/nes/reset.go` | Clear pending DMA and reset the APU before the CPU reads its reset vector. |
| Foundation | Modified | `pkg/nes/system_test.go` | Check per-cycle APU, PPU, and mapper clocks. |
| Foundation | Modified | `pkg/nes/rainbow_test.go` | Adapt mapper IRQ timing test to APU frame IRQ and instruction sampling. |
| Foundation | Modified | `internal/testroms/nestest/nestest_no_ppu.log` | Update the expected trace for the new APU status read. |
| DMA | Added | `pkg/nes/dma.go` | Schedule OAM and DMC bus ownership, stalls, alignment, transfer order, and DMC stall counts. |
| DMA | Added | `pkg/nes/dma_test.go` | Check OAM transfer cycles, delayed DMC fetch, and reported stall cost. |
| DMA | Modified | `pkg/ppu/register.go` | Queue OAM DMA through the bus scheduler, with the existing direct path for an unconnected scheduler. |
| DMA | Modified | `pkg/memory/memory.go` | Track bus cycles and retain controller output on adjacent DMA reads; also correct `$4016` and `$4017` strobing. |
| DMA | Modified | `pkg/memory/memory_test.go` | Check adjacent controller reads, strobing, and APU register routing. |
| DMA | Modified | `pkg/controller/controller.go` | Return one after the eight button bits. |
| DMA | Modified | `pkg/controller/controller_test.go` | Check post-button reads and strobe reset. |
| Mapper compatibility | Modified | `pkg/mapper/mapper.go` | Allocate 8 KiB of CHR RAM for legacy iNES cartridges without CHR ROM. |
| Mapper compatibility | Modified | `pkg/mapper/mapper_test.go` | Check reads and writes at both ends of that CHR RAM. |

| Slice | Status | File | Role |
| --- | --- | --- | --- |
| APU core, Rainbow audio | Modified | `pkg/apu/apu.go` | Connect channels, frames, output, DMC requests, IRQ, reset, DMC fetch telemetry, and optional mapper audio. |
| APU core | Modified | `pkg/apu/register.go` | Decode channel registers, status bits, frame counter writes, and write observations. |
| APU core, Rainbow audio | Added | `pkg/apu/apu_test.go` | Check status, IRQ, sample drain, write observations, reset, DMC telemetry, and mapper mixing. |
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
| Rainbow audio | Added | `pkg/mapper/mapperdb/rainbow/audio.go` | Decode audio registers, expose mapper output and IPCM data, and observe writes. |
| Rainbow audio | Added | `pkg/mapper/mapperdb/rainbow/audio_state.go` | Clock two pulse channels and one saw channel and hold their register state. |
| Rainbow audio | Added | `pkg/mapper/mapperdb/rainbow/audio_test.go` | Check channel sequences, output routing, volume, defaults, and write observations. |
| Rainbow audio | Modified | `pkg/mapper/mapperdb/rainbow/features.go` | Declare expansion audio as a mapper feature. |
| Rainbow audio | Modified | `pkg/mapper/mapperdb/rainbow/features_test.go` | Check that an audio register write marks the feature used. |
| Rainbow audio | Modified | `pkg/mapper/mapperdb/rainbow/irq.go` | Clock audio each CPU cycle and return IPCM data on PCM reads. |
| Rainbow audio | Modified | `pkg/mapper/mapperdb/rainbow/rainbow.go` | Store audio state and set power-on output and volume defaults. |
| Rainbow audio | Modified | `pkg/mapper/mapperdb/rainbow/register.go` | Dispatch writes to Rainbow audio registers. |
| Rainbow audio | Modified | `pkg/mapper/mapperdb/rainbow/registers.go` | Define Rainbow audio register addresses. |
| Rainbow audio | Modified | `pkg/mapper/mapperdb/rainbow/reset.go` | Reset audio routing and master volume registers. |
| Rainbow audio | Modified | `pkg/mapper/mapperdb/rainbow/state.go` | Save and validate audio state; advance the snapshot format to version 3. |

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
| Documentation | Modified | `docs/architecture.md` | Describe APU units, clocks, signal path, Rainbow audio, and audio backend. |
| Documentation | Added | `docs/audio-review.md` | Record accuracy findings, measurements, test results, and open limits, including the Rainbow-only expansion audio scope. |
| Documentation | Modified | `docs/development.md` | Describe APU architecture and hardware ROM test procedure. |
| Documentation | Modified | `docs/gui.md` | Describe GUI audio and the mute flag. |
| Documentation | Modified | `docs/usage.md` | Describe mute and WAV recording options and output behavior. |

## Commit extraction plan

The branch already has focused commits for register-write observations
(`2bfb22d`), sample draining (`eff4280`), Rainbow audio (`4ad68ad`), and DMC
fetch telemetry (`ebc5c0a`). The main extraction targets are the broad initial
APU commit (`d295273`) and the hardware-validation commit (`f5ca079`). The
following order gives each proposed commit one reason to exist.

| Order | Proposed commit | Extracted content | Check before the next commit |
| --- | --- | --- | --- |
| 1 | `apu: add channel and signal units` | Add the channel, envelope, sweep, frame-counter, length-counter, mixer, sampler, filter, and output packages with their paired tests under `pkg/apu/`. Leave system wiring for order 3. | Run package tests for each new APU unit. |
| 2 | `mapper: provide legacy chr ram` | Move `pkg/mapper/mapper.go` and `pkg/mapper/mapper_test.go` together. This fix has no audio dependency. | Run `go test ./pkg/mapper -count=1`. |
| 3 | `nes: clock apu and schedule dma by cpu cycle` | Add the retrogolib dependency update, top-level APU and register logic, CPU cycle hook, IRQ aggregation, OAM/DMC scheduler, reset, PPU DMA request, controller bus fixes, and their tests. Include `internal/testroms/nestest/nestest_no_ppu.log`. | Run focused `pkg/apu`, `pkg/nes`, `pkg/memory`, `pkg/controller`, and `pkg/ppu` tests, then `make test`. |
| 4 | `apu: expose register write observations` | Keep the focused `2bfb22d` API and its tests before the timing review. | Run `go test ./pkg/apu -count=1`. |
| 5 | `apu: expose deterministic sample drain` | Keep the focused `eff4280` API and its tests before WAV recording. | Run `go test ./pkg/apu -count=1`. |
| 6 | `nes: play apu audio through sdl2` | Add `pkg/nes/audio.go`, the `-m` flag, playback setup and error handling, and playback tests. Split these changes from `main.go`, `pkg/nes/system.go`, and `pkg/nes/audio_test.go`. | Run `go test ./pkg/nes -count=1`, then `make test`. |
| 7 | `nes: record deterministic wav audio` | Add `pkg/nes/recording.go` and its tests. Add the `-wav` and `-audio-frames` parts of `main.go`. | Run `go test ./pkg/nes -count=1`. |
| 8 | `apu: correct hardware timing and signal output` | Extract the frame, length, envelope, sweep, DMC, sampler, and bus-timing corrections from `f5ca079`, with their regression tests. Keep the DMC scheduler and APU changes in the same checkpoint when they depend on each other. | Run `make test` and `make lint`. |
| 9 | `testroms: add apu hardware fixtures` | Add the 25 ROMs, upstream notes, hashes, `internal/testroms/apu/apu_test.go`, the `.gitignore` exception, and the Makefile timeout. Keep fixture files with the runner that reads them. | Run `go test -race -timeout 2m ./internal/testroms/apu -count=1`, then `make test`. |
| 10 | `rainbow: emulate expansion audio` | Keep the mapper audio implementation, optional bus interface, feature flag, APU mix hook, and tests together. Keep snapshot version 3 with its new audio fields. | Run `go test ./pkg/mapper/mapperdb/rainbow -count=1`, `go test ./pkg/apu -count=1`, then `make test`. |
| 11 | `apu: expose dmc fetch telemetry` | Keep the observer API with the DMA stall count and tests in `pkg/apu` and `pkg/nes`. | Run `go test ./pkg/apu -count=1` and `go test ./pkg/nes -count=1`. |
| 12 | `docs: describe audio behavior and limits` | Add the user and architecture guides, hardware review, and this summary after the code and results that they describe are in place. | Check links, review the diff, and run `git diff --check`. |

Orders 4, 5, 10, and 11 already have useful commit boundaries. Keep them in
the dependency order shown in the table. Extract orders 1 to 3 and 6 to 9
from the broad implementation commits.

Split `main.go` between playback flags and WAV recording. Split
`pkg/nes/system.go` between CPU bus timing and renderer audio errors. Split
`pkg/apu/apu.go` and `pkg/apu/apu_test.go` between core behavior, later timing
corrections, Rainbow mixing, and telemetry. Split `pkg/nes/audio_test.go`
between playback, IRQ, and signal tests. Place each test hunk with the behavior
it checks. Rebuild each proposed boundary from the final diff, then run its
checks; file-only grouping is insufficient for these files.

This plan changes existing committed history if executed. The branch contains
merge commits, so extraction needs a separate history-rewrite task with its
own validation. This document does not authorize a rebase, reset, staging,
or new commits.

## Verification

- Recorded in `docs/audio-review.md`: `make lint` and `make test` passed during
  the earlier audio review. The review reports that all 25 APU ROM fixtures
  passed. These results precede the Rainbow audio and DMC telemetry commits.
- Not run for this summary: build, lint, and test commands. This update changes
  documentation only.

## Notes

- This summary uses the clean-worktree fallback range `main...HEAD`. Local
  `main` exists. The range contains 107 files after this generated summary is
  excluded.
- Audio timing is NTSC-only. Rainbow expansion audio is implemented; other
  mappers do not supply expansion audio.
- The audio review records remaining limits for NMI/IRQ overlap, adjacent
  PPUDATA reads, DMC abort quirks, internal-register DMA conflicts, and live SDL
  queue measurements.
