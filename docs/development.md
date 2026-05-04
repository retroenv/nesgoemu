# Development

Development workflow, project structure, testing, and contribution notes for nesgoemu.

## Common Commands

```bash
make build
make lint
make test
make test-coverage
make test-coverage-web
make install-linters
```

`make build` runs `CGO_ENABLED=0 go build ./...`. `make lint` runs `golangci-lint` and
`retrogolint`. `make test` runs the full test suite with the race detector.

## Prerequisites

- Go 1.22 or later
- Make
- `golangci-lint` and `retrogolint` for `make lint`
- SDL2 runtime libraries only when running GUI mode

Install the configured linter versions with:

```bash
make install-linters
```

## Local Workflow

```bash
git clone https://github.com/retroenv/nesgoemu.git
cd nesgoemu
make build
make test
make lint
```

Recommended development cycle:

1. Make a focused change.
2. Run `go fmt ./...` if Go files changed.
3. Run `make test`.
4. Run `make lint` before sending changes.
5. Validate emulator behavior with a relevant ROM or test fixture.

## Architecture

Core NES hardware components:

- **CPU** (`pkg/bus/cpu.go`): 6502 processor integration through retrogolib.
- **PPU** (`pkg/ppu/`): Picture Processing Unit registers, memory, and rendering.
- **APU** (`pkg/apu/`): Audio Processing Unit register structure.
- **Bus** (`pkg/bus/`): System interconnect for CPU, PPU, controllers, mapper, and cartridge state.
- **Mappers** (`pkg/mapper/`): Cartridge memory banking and nametable mirroring.
- **System** (`pkg/nes/`): Emulator startup, options, tracing, GUI toggle, and debugger setup.

See [architecture.md](architecture.md) for the package map and supported mapper IDs.

## Testing

### Unit Tests

```bash
make test
make test-coverage
go test ./pkg/controller -v
```

### Test ROMs

The tracked nestest fixture validates CPU execution against a checked-in trace:

```bash
go test ./internal/testroms/nestest
```

Run the fixture through the command-line binary:

```bash
go run . -c -t -e 0xc000 -s 0x0001 internal/testroms/nestest/nestest.nes > trace.log
```

## Debugging Tools

### Web Debugger

```bash
go run . -d test_rom.nes
curl http://127.0.0.1:8080/cpu
curl http://127.0.0.1:8080/mapper
```

### CPU Tracing

```bash
go run . -t test_rom.nes
go run . -c -t test_rom.nes > trace.log
```

### Development Flags

```bash
go run . -c test_rom.nes
go run . -e 0x8000 test_rom.nes
go run . -s 0x8100 test_rom.nes
```

See [usage.md](usage.md) and [advanced-usage.md](advanced-usage.md) for the full command-line reference.

## Code Standards

- Code formatted with `go fmt ./...`.
- Linter clean with `make lint`.
- Tests pass with `make test`.
- Public APIs are documented.
- Errors are handled or explicitly justified.
- Packages stay organized by NES hardware subsystem.
- Interfaces are used where they support testing or hardware component boundaries.

## Release Checks

Before tagging a release, run:

```bash
go fmt ./...
make lint
make test
go vet ./...
```

Snapshot and release builds use GoReleaser:

```bash
make release-snapshot
make release
```

`make release` expects the current git state and tag to be ready for publishing.

## Contributing

Pull request checklist:

- Branch is up to date with main.
- Tests pass with `make test`.
- Linter passes with `make lint`.
- Go files are formatted with `go fmt ./...`.
- New behavior has tests or a documented validation path.
- Commit messages are clear and scoped.

Focus on one feature or fix per pull request.

## Resources

- [NESDev Wiki](https://www.nesdev.org/wiki/Nesdev_Wiki): NES hardware reference.
- [Effective Go](https://go.dev/doc/effective_go): Go best practices.
- [golangci-lint](https://golangci-lint.run/): Go linter documentation.
