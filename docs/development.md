# Development Guide

Development workflow, project structure, and contribution guidelines for nesgoemu.

## Quick Start

### Prerequisites
- Go 1.22+, Make, golangci-lint
- SDL2 libraries (only required at runtime for GUI mode)

### Workflow
```bash
git clone https://github.com/retroenv/nesgoemu.git
cd nesgoemu
make build test lint
go fmt ./...
```

**Development cycle:** Make changes → `make test lint` → Test with ROMs → Commit

## Architecture

Core NES hardware components:

- **CPU** (`pkg/bus/cpu.go`) - 6502 processor using retrogolib
- **PPU** (`pkg/ppu/`) - Picture Processing Unit with scanline rendering  
- **APU** (`pkg/apu/`) - Audio Processing Unit (structure only)
- **Bus** (`pkg/bus/`) - System interconnect and memory mapping
- **Mappers** (`pkg/mapper/`) - Cartridge memory banking (NROM, MMC1, CNROM, UxROM, AxROM, GTROM, UNROM-512)

## Testing

### Unit Tests
```bash
make test                    # Run all tests
make test-coverage          # With coverage
go test ./pkg/controller -v # Specific package
```

### Test ROMs
```bash
go run . nestest.nes        # CPU instruction accuracy
go run . 1.Branch_Basics.nes # Branch instructions
go run . -c -t nestest.nes > trace.log # With tracing
```

Test ROMs validate CPU instructions, memory mapping, and system timing.

## Debugging Tools

### Web Debugger
```bash
go run . -d test_rom.nes
curl http://127.0.0.1:8080/cpu     # CPU state
curl http://127.0.0.1:8080/mapper  # Mapper state
```

### CPU Tracing
```bash
go run . -t test_rom.nes           # Trace to console
go run . -c -t test_rom.nes > log  # Trace to file
```

### Development Flags
```bash
go run . -c test_rom.nes        # Console mode (no GUI)
go run . -e 0x8000 test_rom.nes # Custom entry point
go run . -s 0x8100 test_rom.nes # Stop at address
```

## Code Standards

### Requirements
- Code formatted: `go fmt ./...`
- Linter clean: `make lint` (0 issues)
- Tests pass: `make test`
- Public APIs documented
- Error handling (never ignore errors)

### Project Patterns
- Package organization by hardware component
- Interfaces for mockable components
- Structured error handling with wrapping
- Minimal external dependencies

## Contributing

### Pull Request Checklist
- [ ] Branch up to date with main
- [ ] Tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] Code formatted (`go fmt ./...`)
- [ ] New functionality has tests
- [ ] Clear commit messages

### Process
1. Fork repository
2. Create feature branch
3. Make changes with tests
4. Ensure linter passes
5. Submit pull request

Focus on single feature/fix per PR with clear descriptions.

## Build Targets

```bash
make build           # Build binary
make test            # Run tests
make test-coverage   # Run with coverage
make lint            # Run linter
```

## Resources

- [NESDev Wiki](https://wiki.nesdev.com/) - NES hardware reference
- [Effective Go](https://golang.org/doc/effective_go.html) - Go best practices
- [golangci-lint](https://golangci-lint.run/) - Linting tool