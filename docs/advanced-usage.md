# Advanced Usage

Advanced debugging, automation, and development workflows for nesgoemu.

## Debugging Tools

### Web Debugger
```bash
nesgoemu -d game.nes                    # Default (localhost:8080)
nesgoemu -d -a "0.0.0.0:9000" game.nes # Custom address
```

**Endpoints:**
- `/cpu` - CPU state and registers
- `/mapper` - Memory mapper state  
- `/ppu/palette` - PPU palette data
- `/ppu/nametables` - PPU nametable data

### CPU Tracing
```bash
nesgoemu -t game.nes                    # Trace to console
nesgoemu -c -t game.nes > trace.log     # Trace to file
nesgoemu -d -t game.nes                 # Trace with debugger
```

### Execution Control
```bash
nesgoemu -e 0x8000 game.nes             # Custom entry point
nesgoemu -s 0x8100 game.nes             # Stop at address
nesgoemu -e 0x8000 -s 0x8100 game.nes   # Entry + stop
```

## Console Mode & Automation

### Headless Operation
```bash
nesgoemu -c game.nes                    # No GUI
nesgoemu -c -t test.nes > results.log   # With output capture
```

### Batch Testing
```bash
#!/bin/bash
for rom in *.nes; do
    echo "Testing $rom..."
    timeout 30s nesgoemu -c -t "$rom" > "logs/${rom%.nes}.log" 2>&1
    [ $? -eq 0 ] && echo "✓ $rom passed" || echo "✗ $rom failed"
done
```

## Test ROMs & Validation

### Standard Test ROMs
```bash
nesgoemu nestest.nes                    # CPU instruction accuracy
nesgoemu 1.Branch_Basics.nes            # Branch instructions
nesgoemu -c -t nestest.nes > results.log # With analysis
```

### ROM Compatibility
```bash
nesgoemu -c -e 0x8000 -s 0x8001 rom.nes # Quick compatibility check
nesgoemu -c -t unknown_rom.nes > trace.log # Full execution trace
```

## Performance Analysis

### Profiling
```bash
go run -cpuprofile=cpu.prof . game.nes  # CPU profiling
go run -memprofile=mem.prof . game.nes  # Memory profiling
go tool pprof cpu.prof                  # Analyze profiles
```

### Benchmarking
```bash
time nesgoemu -c -s 0x9000 game.nes     # Performance timing
/usr/bin/time -v nesgoemu -c game.nes    # Memory usage
```

## Build Options

```bash
go build -ldflags="-s -w" .     # Production build (smaller binary)
```
