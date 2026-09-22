# Audio accuracy review

This review found channel, timing, DMA, and resampling defects that can change
the sound. The changes correct those defects with hardware test ROMs and
independent signal measurements. No MesenCE implementation was copied.

The reported difference in a specific game is not yet reproduced. No game
segment or reference recording was supplied. MesenCE was inspected as source;
its application was not used to make a reference recording.

## Reference versions

- nesgoemu baseline: `eff4280229feeaff323af88b5f834ab59f2e485d`.
- [retrogolib bus-cycle support, revision 9f81a91c9f86d10f539a9932428e2985a5ea4027](https://github.com/retroenv/retrogolib/commit/9f81a91c9f86d10f539a9932428e2985a5ea4027).
- [MesenCE, revision 0636c265d441cfd4302da5c2e3828c1b49aeb70a](https://github.com/nesdev-org/MesenCE/tree/0636c265d441cfd4302da5c2e3828c1b49aeb70a).
- [NES test ROMs, revision 95d8f621ae55cee0d09b91519a8989ae0e64753b](https://github.com/christopherpow/nes-test-roms/tree/95d8f621ae55cee0d09b91519a8989ae0e64753b).
- [SingleStepTests/65x02, revision 2f6980a2d95757486c7bee24355c360e40e2a224](https://github.com/SingleStepTests/65x02/tree/2f6980a2d95757486c7bee24355c360e40e2a224).

The module dependency uses retrogolib version
`v0.0.0-20260922153744-9f81a91c9f86`, which contains the CPU bus-cycle API.
No local workspace replacement is required.

## Corrected behavior

| Area | Defect in the baseline | Current behavior and independent check |
| --- | --- | --- |
| Resampling | Whole-cycle box averages convert ultrasonic triangle output into audible tones. | A Blackman-windowed sinc FIR filters before rate conversion. Sine-wave gain and triangle spectrum tests check the result. |
| `$4015` | Frame and DMC interrupt bits are reversed. Existing tests require the wrong masks. | Frame IRQ uses bit 6; DMC IRQ uses bit 7. Register tests and hardware ROMs check both sources and acknowledgement. |
| Envelope | Volume writes restart decay. | Only a length-load write starts decay. A repeated volume write preserves the decay sequence. |
| Sweep | A reload cancels an otherwise due period update. | The update precedes divider reload. Tests cover zero and nonzero dividers. |
| Frame counter | Sequence periods are short; two IRQ assertion opportunities are missing; a pending write stops the old sequence. | Repeated event traces and timing ROMs check sequence length, IRQ edges, and the phase-dependent write delay. |
| Length counter | Reload and halt writes take effect before a simultaneous frame clock. | Writes pass through a latch. A simultaneous decrement cancels a reload of a nonzero counter. Both older edge-timing ROMs pass. |
| DMC DAC | A positive step changes 126 to 127. | An out-of-range two-unit step leaves the DAC unchanged. Tests cover both directions at all 128 levels. |
| DMC reader | Enabling playback fetches immediately; every fetch adds a fixed stall; refill is late. | Load and refill requests use the CPU bus. Fetch completion changes length and IRQ. Output-buffer transfer occurs at the byte boundary. |
| CPU/APU timing | All register accesses occur before the instruction's APU clocks. | retrogolib emits each CPU bus cycle. The system clocks devices before the corresponding access. Dummy reads and both RMW writes reach memory. |
| DMA | Sprite and DMC transfers do not share a bus schedule. | The scheduler waits for a CPU read, handles alignment, and gives DMC reads priority over sprite reads. DMA ROMs check overlaps. |
| Interrupts | The CPU checks live IRQ state at instruction boundaries; an APU or mapper acknowledgement can clear the other source. | Interrupt sampling uses the documented instruction cycles. APU and mapper sources drive a shared OR. CLI, branch, and IRQ/DMA ROMs pass. |
| Reset | Reset recreates channels and loses retained settings. | Reset clears the specified internal state and retains register settings. Power/reset ROMs pass. |

Two bus defects surfaced in the DMA checks. A controller must return one after
its eight button bits. `$4016` strobes both controllers; `$4017` writes only the
APU frame counter. Adjacent DMA reads now retain controller output enable.
Legacy iNES cartridges with no CHR ROM also receive the required CHR RAM.

The hardware requirements come from the NESdev descriptions of
[registers](https://www.nesdev.org/wiki/APU_registers),
[envelopes](https://www.nesdev.org/wiki/APU_Envelope),
[sweep](https://www.nesdev.org/wiki/APU_Sweep),
[length counters](https://www.nesdev.org/wiki/APU_Length_Counter),
[DMC](https://www.nesdev.org/wiki/APU_DMC),
[DMA](https://www.nesdev.org/wiki/DMA),
[interrupt polling](https://www.nesdev.org/wiki/CPU_interrupts), and
[reset](https://www.nesdev.org/wiki/CPU_power_up_state), together with the
test ROM source. Existing unit tests are evidence to inspect, not hardware
specifications.

## Measured audio changes

The triangle probe uses the actual channel, nonlinear mixer, sampler, analog
filters, and signed 16-bit output. It measures the second second of output.

| Triangle period | Source frequency | Measured output frequency | Baseline peak | Current steady output |
| --- | ---: | ---: | ---: | --- |
| 0 | 55,930.398 Hz | 11,830.398 Hz alias | −35.84 dBFS | All S16 samples are zero. |
| 1 | 27,965.199 Hz | 16,134.801 Hz alias | −28.66 dBFS | All S16 samples are zero. |
| 253 | 220.198 Hz | 220.198 Hz | −27.26 dBFS | −27.25 dBFS peak. |

Zero S16 output means that the steady residual is below the quantization
threshold. It does not mean that the floating-point filter has infinite
rejection. Startup transients are excluded from this measurement.

The sampler tests cover 44.1 and 48 kHz. Measured gain error is below 0.3% at
1, 10, and 15 kHz. Gain is below 0.0002, about −74 dB, at 24, 27.965199,
30, and 55.930398 kHz. These limits describe the sampler before the analog
output filters. The exact NTSC ratio produces 77 samples per 3,125 CPU cycles
at 44.1 kHz. Filter delay is approximately 24 output samples, or 0.54 ms.
The nonlinear mixer remains before resampling.

MesenCE uses [timed mixed-output changes and blip_buf](https://github.com/nesdev-org/MesenCE/blob/0636c265d441cfd4302da5c2e3828c1b49aeb70a/Core/NES/NesSoundMixer.cpp)
at 96 kHz, followed by a separate resampling path. The FIR here comes from
standard filter equations. Its tests specify observable gain and timing
limits, without a MesenCE PCM checksum as the expected result.

## Validation results

Both repositories pass `make lint` and `make test`, including their race
checks. The permanent [APU ROM package](../internal/testroms/apu/README.md)
includes all 25 APU fixtures, their hashes, and the upstream descriptions.
It is part of `make test`, like nestest. Its race run took about 29 seconds
on the review machine; the package timeout is now two minutes.

| Independent check | Result |
| --- | --- |
| Eight modern APU ROMs and six reset ROMs | 14 pass, compared with 5 at baseline. |
| Eleven 2005 APU ROMs | 11 pass, including halt, reload, and IRQ timing. |
| nestest | Pass. |
| NES6502 SingleStepTests sample | 61,000 cases pass across all 256 opcodes, with address, data, and read/write cycle checks. |
| DMC/sprite DMA overlap | Both ROMs pass. |
| DMA/controller reads and PPUDATA writes | `dma_4016_read`, `dma_2007_write`, and `read_write_2007` pass. |
| CPU interrupt ROMs | CLI latency, NMI/BRK, IRQ/DMA, and branch delay pass. NMI/IRQ overlap still fails. |

The CPU sample uses 200 cases per opcode, with all 10,000 cases for opcode
`BD`. It is not the complete upstream dataset. Those vectors come from an
independent emulator; they are not direct hardware captures. The hardware
ROMs provide a separate check.

## Differences and limits that remain

The analog filter target is unchanged: high-pass filters at 90 and 440 Hz,
then a low-pass filter at 14 kHz. This follows the
[documented NES mixer output model](https://www.nesdev.org/wiki/APU_Mixer).
It attenuates 100 Hz by about 15.7 dB and 220 Hz by about 7.8 dB. MesenCE's
inspected default path has a much lower DC-removal corner and a different
digital output scale. These choices can still produce a clear bass and
volume difference. A listening comparison must account for them.

The following items remain open:

- `cpu_interrupts_v2/3-nmi_and_irq.nes` produces CRC `9AE2A4DC`, while its
  source expects `B7B2ED22`. The CPU/PPU interrupt overlap needs more analysis.
- The adjacent PPUDATA-read diagnostic `double_2007_read` produces
  `D84F6815`, which differs from its documented hardware results. The
  separate `dma_2007_read` trace matches the documented `5E3DF9C4` result.
- DMC stop/restart abort quirks and all internal-register DMA bus conflicts
  are not fully modeled.
- Timing remains NTSC-only. PAL audio requires separate work. The Rainbow
  mapper supplies expansion audio; other mappers do not.
- Live SDL queue drift and device underruns were not measured. `AudioStats`
  now exposes produced, queued, dropped, and empty-queue playback frames.
  These counters describe the APU queue; SDL has a separate queue.

## Reproduce and compare

From this repository:

```sh
make lint
make test
go test -race -timeout 2m ./internal/testroms/apu -count=1
go run . -wav capture.wav -audio-frames 441000 game.nes
```

The recording command writes exactly ten seconds of 44.1 kHz mono S16 PCM.
It starts from power-on with no controller input or battery save. It requires
a new output path and does not open an audio device. The `System.WriteWAV`
API can instead continue a prepared system state. Recording in two chunks
produces the same PCM as one continuous recording.

Three command-line captures of a synthetic pulse program produced identical
WAV hashes. Each ten-second capture took 1.99 to 2.09 seconds on the review
machine, including process startup. This is about 4.8 to 5.0 times real time
for that workload; it is not a performance result for every game or device.

For a game comparison, use the same ROM, region, initial state, input sequence,
and emulated interval. Record both emulators. Align the signal delay, account
for output gain and filter settings, then compare spectra and listen. If a
difference remains, first compare register-write times and channel events,
then mixed levels, then the output stage. Resolve a disagreement with hardware
evidence or an independent calculation before changing an expectation.
