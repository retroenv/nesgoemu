# Advanced Usage

Advanced debugging, automation, validation, and performance workflows for nesgoemu.

## Debugging Tools

### Web Debugger

Start the debugger on the default address:

```bash
nesgoemu -d game.nes
```

Start the debugger on a custom address:

```bash
nesgoemu -d -a 127.0.0.1:9000 game.nes
```

Available endpoints:

| Endpoint | Description |
|----------|-------------|
| `/cpu` | CPU registers, flags, cycles, and interrupt state |
| `/cpu/pause` | Reserved pause endpoint; currently not implemented |
| `/mapper` | Current mapper state |
| `/ppu/palette` | PPU palette data |
| `/ppu/mirrormode` | Current nametable mirror mode |
| `/ppu/nametables` | PPU nametable data |

Example requests:

```bash
curl http://127.0.0.1:8080/cpu
curl http://127.0.0.1:8080/mapper
curl http://127.0.0.1:8080/ppu/mirrormode
```

### CPU Tracing

Print CPU trace output to the console:

```bash
nesgoemu -t game.nes
```

Capture trace output without opening the GUI:

```bash
nesgoemu -c -t game.nes > trace.log
```

Run tracing and the web debugger together:

```bash
nesgoemu -d -t game.nes
```

### Execution Control

The `-e` and `-s` flags accept decimal or Go-style base-prefixed integers such as `0x8000`.

```bash
nesgoemu -e 0x8000 game.nes
nesgoemu -s 0x8100 game.nes
nesgoemu -e 0x8000 -s 0x8100 game.nes
```

## Console Mode and Automation

### Headless Operation

Console mode disables GUI setup and is useful for automated runs:

```bash
nesgoemu -c game.nes
nesgoemu -c -t game.nes > trace.log
```

### Batch Testing

```bash
#!/usr/bin/env bash
set -euo pipefail

mkdir -p logs

for rom in *.nes; do
    echo "Testing ${rom}"
    if timeout 30s nesgoemu -c -t "${rom}" > "logs/${rom%.nes}.log" 2>&1; then
        echo "PASS ${rom}"
    else
        echo "FAIL ${rom}"
    fi
done
```

## Test ROMs and Validation

The tracked nestest fixture lives under `internal/testroms/nestest` and is covered by a Go test:

```bash
go test ./internal/testroms/nestest
```

Run it through the command-line binary with the same entry and stop addresses used by the test:

```bash
nesgoemu -c -t -e 0xc000 -s 0x0001 internal/testroms/nestest/nestest.nes > trace.log
```

Compare traces against the checked-in reference when needed:

```bash
diff -u internal/testroms/nestest/nestest_no_ppu.log trace.log
```

For unknown ROMs, start with console mode and tracing before using GUI mode:

```bash
nesgoemu -c -t unknown.nes > trace.log
```

## Performance Analysis

nesgoemu does not currently expose `-cpuprofile` or `-memprofile` application flags. For repeatable
profiling of the checked-in nestest workload, use Go test profiling:

```bash
go test -run TestNestest -cpuprofile cpu.prof ./internal/testroms/nestest
go tool pprof cpu.prof
```

For command-line timing of a ROM run:

```bash
time nesgoemu -c -s 0x9000 game.nes
/usr/bin/time -v nesgoemu -c game.nes
```

## Build Options

Build the local command:

```bash
go build .
```

Build a smaller release-style binary:

```bash
go build -ldflags="-s -w" .
```

Project Make targets are documented in [development.md](development.md).
