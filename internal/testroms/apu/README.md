The APU tests run 25 unmodified ROMs by Shay Green (Blargg): eight APU
tests, six power/reset tests, and eleven frame-counter tests from 2005.
They use the full NES system, including CPU bus timing, DMA, and audio sampling.
They do not require a GUI, audio device, network connection, or saved game.

The fixtures come from [nes-test-roms at revision
95d8f621ae55cee0d09b91519a8989ae0e64753b](https://github.com/christopherpow/nes-test-roms/tree/95d8f621ae55cee0d09b91519a8989ae0e64753b).
The local `apu_2005` directory comes from upstream `blargg_apu_2005.07.30`.
The local `apu_test` ROMs come from upstream `apu_test/rom_singles`.
`SHA256SUMS` records the exact fixture bytes. The test checks this manifest.
The upstream descriptions are retained as text references beside the ROMs.

Run the tests with:

```sh
go test -race -timeout 2m ./internal/testroms/apu -count=1
```

`make test` includes this package. To run one ROM:

```sh
go test -timeout 2m ./internal/testroms/apu -run 'TestAPU/apu_test/4-jitter' -v
```

The later ROMs identify their result area with bytes `$DE $B0 $61` at
`$6001` through `$6003`. The runner must observe a running status before it
accepts completion. It waits 200,000 CPU cycles before each requested reset.
Success is status `$00`; another completion code fails the test and prints
the ROM's diagnostic text. The 2005 ROMs instead stop in a loop and display
`$01` in the nametable. The runner checks that separate protocol.

Each ROM has a limit of 30 million CPU cycles. The package timeout also stops
a stalled host process. Missing fixtures, invalid hashes, an unknown result,
and a timeout all fail the test.

These ROMs test CPU-visible behavior. They cannot prove that the audible
spectrum is correct. The sampler and channel tests cover that separate
requirement. If a ROM fails, compare its source with hardware evidence before
changing either the emulator or a test expectation.
