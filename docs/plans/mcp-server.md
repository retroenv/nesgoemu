# Plan: MCP server for nesgoemu

Status: proposed (2026-10-09)
Depends on: retrogolib `mcp` package (`retrogolib/docs/plans/mcp-package.md`),
stages 1-2; retrogolib stage 4 helpers (CDL writer, `cpu6502.Decode`).
Companion: retrodisasm MCP server (`retrodisasm/docs/plans/mcp-server.md`).

## Goal

Let an AI agent drive the emulator headless: load a ROM, step and run, set
breakpoints, inspect CPU, memory, PPU and APU state without side effects, take
screenshots, record code/data logs, and find which code touches a RAM address.
The agent combines this with the retrodisasm server to produce named, commented
disassemblies.

Protocol: MCP revision `2026-07-28`, stdio transport first. The protocol has no
sessions, so emulator instances are explicit handles: `create_machine` returns
a `machine_id`, and every other tool takes it.

## Current state (from the code)

- Headless mode exists. `nes.NewSystem(opts)`, `sys.StepSystem()` and
  `sys.Run(ctx, RunOptions)` need no GUI or SDL (`pkg/nes/system.go`,
  `pkg/nes/run.go`). `nes.Start` is unsuitable: it blocks, paces to real time,
  writes the summary and battery file, and has no control surface.
- Registers: `sys.CPU` is embedded; `sys.State()` returns `cpu6502.State`.
  `Step` does not take the CPU mutex, so reads during a run race.
- Memory reads through `sys.Bus.Memory.Read` have side effects (PPU latches,
  controller shift, mapper registers). Side-effect-free paths today:
  `sys.InspectRAM` (`$0000-$1FFF`), `InspectOAM`, `Palette().Data()`,
  `NameTable.Data()`/`ReadCIRAM`, mapper reads outside register ranges,
  `PRGROMMapper.PRGROMOffset`, `PPUMappingInspector`.
- Hooks are fixed at construction: `WithCPUPreExecutionHook` (one), cycle hook
  owned by `System.clockCPUCycle`, `WithCPUOptions` for the stack event hook.
  Runtime observers exist for CPU writes, PPU writes, VRAM writes, APU
  registers, DMC fetches and mapper graphics. There is no CPU read observer.
- Stop control: only `-s` stop address and `Run`'s RAM completion condition.
  No breakpoint list.
- No CDL recording, no save states (Rainbow mapper has its own), no logger,
  no disassembler beyond trace formatting. Trace output and the summary go to
  stdout. Panics exist in memory, PPU register, mapper hook, tracing and
  sampler paths.
- `main.go` uses the standard `flag` package with `ExitOnError`.
- Partial synchronization: RAM, mapper banks, nametable, palette, controller
  and features are protected; PPU internals, screen buffers, APU, DMA and
  mapper registers are not.

## Design decisions

- **Ownership model:** a `Machine` owns one `*nes.System` and a `sync.Mutex`.
  Every tool locks the machine for the duration of the call. The emulator only
  advances inside tool calls (`step`, `run_frames`, `run_until`). There is no
  background emulation goroutine in stdio mode, so there are no data races and
  runs are deterministic. A wall-clock `timeout` on every run tool keeps a call
  bounded. Live GUI attachment, which needs a background goroutine, is the last
  stage.
- **Registry:** `internal/mcpserver/registry.go` maps `machine_id` (random
  12-byte base32) to `*Machine`, with a cap (`-max-machines`, default 4) and an
  idle expiry (`-machine-ttl`, default 30 min). Tool descriptions state the
  expiry so the agent knows to recreate.
- **State across calls without protocol sessions:** MCP `2026-07-28` has no
  sessions, but the server process keeps the emulator alive between calls and
  the agent addresses it with `machine_id`. This is the pattern the
  specification recommends ("Stateful Tools": server-minted handles passed as
  ordinary arguments). Playing a game level by level is a loop of
  `run_frames(inputs)`, `screenshot`, `read_memory`, decide, repeat.
- **Checkpoints by deterministic replay:** every machine records an event log:
  `{frame, kind: input|poke|reset, data}`. Emulation is deterministic and only
  advances inside tool calls, so "frame N of the log" is a complete checkpoint.
  Restore = new `System` + replay to N; fork = new machine replaying the same
  log; rewind = replay to N-k. Headless replay runs far faster than real time,
  so a few minutes of gameplay replays in seconds. The log is written under a
  root directory, so a machine survives a server restart (Claude Code gives
  stdio servers a 30-minute idle timeout and does not reconnect them). Full
  save states (serializing CPU, PPU, APU, DMA, RAM and mappers) become an
  optimization for long logs, not a prerequisite.
- **Side-effect-free inspection is mandatory** for every read tool. Reads that
  must go through the bus (for example writing to a register to test a
  behavior) are separate tools with `readOnlyHint: false`.
- **Address vocabulary (shared with retrodisasm):** `{"bank", "address",
  "prg_offset"}` objects; `prg_offset` from `PRGROMMapper.PRGROMOffset` is the
  key that retrodisasm understands. Inputs accept integer, `"$C000"` or
  `"0xC000"`.
- **Images:** screenshots are PNG in an `image` content block plus a text block
  with frame number and CPU state. Clone the front buffer before encoding as
  `run.go` `cloneRGBA` does.
- **Logging:** introduce `retrogolib/log` in `main.go` only, output to stderr,
  pass the logger as the first parameter into `mcpserver`. In MCP mode set
  `WithTracingTarget(io.Discard)` or a ring buffer, `WithSummaryTarget(nil)`,
  `WithDisabledGUI()`, `WithDisabledAudio()`, `WithSavePath("")` unless the
  tool asks for battery persistence.
- **Entry point:** `nesgoemu mcp [-transport stdio|http] [-addr 127.0.0.1:0]
  [-root DIR ...] [-max-machines N] [-machine-ttl 30m] [-debug]`. `main.go`
  checks `os.Args[1] == "mcp"` before the existing flag parsing. ROM paths are
  restricted to the roots.
- **Package layout:** `internal/mcpserver/` (server, registry, machine, roots)
  and `internal/mcpserver/tools_*.go` per tool group. Emulator-side additions
  go into `pkg/nes` and `pkg/ppu` as public API so the GUI debugger can reuse
  them.

## Stages

### Stage 0: prerequisites

- retrogolib `mcp` stage 1 and 2; retrogolib stage 4 (`codedatalog.Save`,
  `cpu6502.Decode`). Pin in `go.mod`.
- Client support is known (retrogolib `docs/plans/mcp-client-compatibility.md`,
  measured 2026-10-09): Claude Code 2.1.292 or later works with default
  settings. Codex CLI, OpenCode, Gemini CLI and Cline still use the legacy
  `initialize` handshake and cannot connect. `docs/mcp.md` must state this.
- Commit or stash the current uncommitted formatting changes in `pkg/nes` and
  the Rainbow mapper so the MCP work starts from a clean tree.

### Stage 1: emulator preparation (no MCP yet)

All items are public API in `pkg/`, each with unit tests.

1. **Runtime hook dispatcher** (`pkg/nes/hooks.go`): `System` installs one
   pre-execution hook and one stack hook at construction that forward to a
   `hookSet` stored in an `atomic.Pointer`. Add `sys.AddPreExecutionHook(fn)
   (remove func())`, `sys.AddStackHook(fn) (remove func())`,
   `sys.AddBusCycleObserver(fn func(cpu6502.BusCycle)) (remove func())`. The
   bus-cycle observer is called from `clockCPUCycle` only when the pointer is
   non-nil, so the hot path stays one atomic load when unused. Measure with the
   existing benchmarks or a new one before and after.
2. **Peek API** (`pkg/nes/inspect.go`): `sys.Peek(addr uint16) (byte, bool)` for
   CPU space without side effects: RAM via `InspectRAM`, `$6000-$7FFF` via
   mapper read when the mapper is not Rainbow-register-mapped there, `$8000+`
   via `PRGROMOffset` into `cart.PRG`, everything else returns `false`.
   `sys.PeekRange(addr, n)`. `sys.PRGLocation(addr) (bank int, prgOffset int,
   ok bool)` derived from `MapperState.PrgWindows`.
3. **PPU inspection** (`pkg/ppu/inspect.go`): export a `Snapshot()` struct with
   scanline, dot, frame, ctrl, mask, status, v, t, x, w, read buffer, and
   `PeekVRAM(addr)` via `PPUReadMapping` (CHR from mapper, CIRAM via nametable,
   palette). Guard with the existing locks where they exist; document that
   callers hold the machine lock.
4. **Breakpoint engine** (`pkg/nes/breakpoint.go`): `BreakpointSet` with
   `Add(Breakpoint) ID`, `Remove(ID)`, `List()`. Kinds: `exec` (PC), `read`,
   `write`, `access` on a CPU address range, optional `bank` match via
   `PRGLocation`, optional value match for writes, hit count. Implemented on
   the pre-execution hook (exec) and bus-cycle observer (read/write).
   `sys.RunUntil(ctx, RunUntilOptions) RunUntilResult` extends `Run` with
   `Breakpoints`, `MaxInstructions`, and returns `Reason` (`breakpoint`,
   `frames`, `instructions`, `cycles`, `timeout`, `context`, `invalid_instruction`)
   plus the breakpoint ID.
5. **CDL recorder** (`pkg/nes/cdl.go`): `Recorder` fed by the pre-execution hook
   (Code, SubEntryPoint on JSR targets, IndirectCode on JMP indirect targets)
   and the bus-cycle observer (Data, IndirectData for indexed/indirect reads,
   PCMAudio from the APU DMC fetch observer), with bank flags from
   `MapperState`. CHR flags from PPU fetches (DrawnOnScreen) and `$2007`
   reads (ReadProgrammatically). Export with `codedatalog.Save`. Start, stop,
   reset, `Coverage()` per bank.
6. **Trace ring buffer** (`pkg/nes/trace.go`): capture nestest-format lines into
   a bounded ring (default 10k) instead of stdout. Reuse `printTraceStep`
   formatting with a side-effect-free operand formatter built on `Peek`,
   which also removes the documented bus-read side effect of tracing.
7. **Disassembler** (`pkg/nes/disasm.go`): `sys.Disassemble(addr, count)
   []Instruction{Location, Bytes, Mnemonic, Operand, Size}` using
   `cpu6502.Decode` and `parameter.String`.
8. **Panic boundaries:** wrap `StepSystem`-driving loops in the new run
   functions with `recover` that converts to an error with the PC and the
   panic text, so one bad ROM does not kill the server.
9. **Logger:** add `retrogolib/log`; `main.go` creates it; `nes.Options` gets
   `WithLogger(*log.Logger)` for the few places that print today.

Acceptance: `make build-all test lint` passes; nestest still matches its log
through the new trace buffer; a benchmark shows no measurable slowdown with no
observers installed.

### Stage 2: minimum server

Tools:

| Tool | Hints | Purpose |
| --- | --- | --- |
| `create_machine` | write | `rom` path, `entrypoint`, `battery` (bool). Returns `machine_id`, ROM summary (mapper, sizes, mirroring, CRCs in retrodisasm `[rom]` format), initial CPU state |
| `destroy_machine` | destructive | frees the machine |
| `list_machines` | read | ids, ROM names, frame counters, idle time |
| `reset` | write | `sys.Reset()`; optional `hard` recreates the system |
| `step` | write | `instructions` (default 1, max 1e6); returns CPU state before and after, instructions executed, cycles, frames completed |
| `run_frames` | write | `frames`, optional `inputs[]` `{frame, controller, buttons}`, `timeout` (default 10 s); returns `RunResult` fields |
| `cpu_state` | read | registers, flags as letters and byte, cycles, frame, pending interrupts, current `PRGLocation` of PC |
| `read_memory` | read | `space` (`cpu`, `ppu`, `oam`, `prg`, `chr`, `ram`), `address`, `length` (max 4096); returns hex dump text and bytes array; uses Peek paths only |
| `write_memory` | write | `space` `cpu` RAM/PRG-RAM only, `address`, `bytes`; refuses register ranges |
| `set_buttons` | write | `controller` 1/2, `buttons` names, `hold_frames` (0 = until changed) |
| `screenshot` | read | PNG image content plus frame and CPU text; `scale` 1-4 |
| `machine_info` | read | mapper state (PRG/CHR windows → bank numbers), mirroring, features used, battery status |

Resources: `nesgoemu://docs/mcp` (this tool reference, embedded) and
`nesgoemu://machines/{id}/screen` as a resource template returning the PNG.

Server instructions: explain machines and expiry, the address object, that
reads are side-effect free, that `run_*` tools advance time and nothing else
does, and that `cdl_export` produces a file retrodisasm reads with `cdl`.

Also: `main.go` routing, `internal/mcpserver/cmd.go` with `retrogolib/cli`,
`docs/mcp.md` with Claude Code setup (`claude mcp add --transport stdio nesgoemu
-- nesgoemu mcp -root /path/to/roms`) and a `.mcp.json` example, README link.

Tests: `mcptest.Client` against a synthetic cartridge (`cartridge.New()` with
a short program and vectors): create, step, cpu_state values, read_memory of
RAM after a store, screenshot decodes as 256x240 PNG, path outside roots
rejected, machine expiry, max machines, unknown machine id returns an
`isError` result with the recreate hint.

Acceptance: from Claude Code, "load this ROM, run 120 frames, and show me the
screen" works with three tool calls.

### Stage 3: debugging

| Tool | Purpose |
| --- | --- |
| `add_breakpoint` | kind, address or range, optional bank, optional value, optional `once`; returns id |
| `remove_breakpoint`, `list_breakpoints` | manage; list includes hit counts |
| `run_until` | runs with breakpoints until one hits or `frames`/`instructions`/`cycles`/`timeout`; returns reason, breakpoint id, CPU state, and the last N trace lines |
| `disassemble` | `address`, `count`, optional `bank`; returns instruction rows with location objects |
| `trace_start`, `trace_stop`, `get_trace` | ring buffer control; `get_trace` pages with `limit` and `offset` |
| `call_stack` | reconstructed from the stack event hook: list of `{return_address, caller_location, kind}` for JSR and interrupt frames |
| `step_over`, `step_out` | built on `run_until` with a temporary exec breakpoint from the stack events |
| `checkpoint` | names the current frame in the machine's event log; returns `checkpoint_id` and writes the log to `<root>/<machine>.events.json` |
| `restore` | `machine_id`, `checkpoint_id` or `frame`: resets and replays the log to that frame on the same machine |
| `fork_machine` | new machine from a checkpoint of another machine, for "try this jump, else fork again" loops |
| `restore_machine` | `rom`, `events` file: recreates a machine after a server restart by replaying the file |
| `event_log` | returns the recorded inputs and pokes, paged, so the agent can edit and replay a run |

Replay rules: a replay uses the same ROM (checksum stored in the log), the
same battery policy, and replays `write_memory` pokes at their frame and
instruction count. `step` calls record their instruction counts too, so a log
is exact to the instruction. A test replays a 2000-frame run with inputs and
pokes and asserts identical RAM, CPU state and frame hash.

Acceptance: nestest headless through MCP: create at `0xC000`, `run_until`
PC `0x0001`, `get_trace` equals the reference log (reuse the existing
comparison). Checkpoint, fork and restore produce bit-identical machines.

### Stage 4: reverse-engineering analysis

| Tool | Purpose |
| --- | --- |
| `cdl_start`, `cdl_stop`, `cdl_reset`, `cdl_export` | record and write a `.cdl` file inside roots for retrodisasm `-cdl` |
| `coverage` | executed and read bytes per PRG bank, as counts and percentages, from the CDL recorder |
| `bank_state` | current PRG and CHR windows with bank numbers (`MapperState`), plus a history of bank switches since the last call when the mapper write observer sees them; this is what retrodisasm `set_bank_mapping` consumes |
| `find_accessors` | runs `frames` with a read/write observer on a RAM address or range; returns PCs with location objects, counts, and the values written; groups by instruction |
| `memory_search` | cheat-search style: `start` snapshot, then `filter` with `equal`, `changed`, `unchanged`, `increased`, `decreased`, `value`; returns candidate addresses with current and previous values; keeps the candidate set per machine |
| `ram_diff` | snapshot RAM, run `frames` with optional inputs, return changed bytes |
| `watch_writes` | like `find_accessors` but returns the write log in order with frame and scanline (from `memory.WriteEvent` and PPU write events) |

Acceptance: on a test ROM the agent can find the frame counter address with
`memory_search` in three calls and name it through retrodisasm
`update_annotations`.

### Stage 5: PPU and APU inspection

| Tool | Purpose |
| --- | --- |
| `ppu_state` | the stage 1 snapshot |
| `nametables` | all four tables as tile index grids (text) and optionally as a rendered PNG with attributes |
| `palette`, `oam` | decoded entries (colors as RGB, sprites as x, y, tile, attributes) |
| `pattern_tables` | render CHR banks to PNG with a chosen palette |
| `apu_state` | channel registers decoded, frame counter, DMC address and length, expansion audio if any |
| `record_audio` | `frames`; returns a WAV blob via `WriteWAV` on a cloned run or writes to a path in roots |

Acceptance: `nametables` output of a known screen matches the screenshot.

### Stage 6: snapshots, live view, HTTP

- **Save states:** serialize CPU (`cpu6502.State` plus a new restore path for
  cycles and interrupt latches, which needs a retrogolib change), PPU, APU,
  RAM, CIRAM, OAM, palette, DMA, and mapper state (Rainbow's `SaveState` is
  the template; `mapperbase` gets a generic one). Tools `save_state` and
  `load_state`; `restore`, `fork_machine` and `rewind` switch from replay to
  states when one exists. This is the largest item and only an optimization:
  stage 3 checkpoints already give the agent retries and recovery. Build it
  when replay times for long play sessions become a problem.
- **Live view:** `attach_display` opens the SDL window for a machine and runs
  emulation on a background goroutine paced to real time; tool calls then
  pause the goroutine through a command channel. This reintroduces the
  threading model and is the only stage that needs it.
- **HTTP transport** with the retrogolib stage 3 handler.
- **Legacy debugger:** route `pkg/nes/debugger` endpoints to the same
  `Machine` API, or remove the package once the MCP server covers it.

## Testing strategy

- Unit tests in `pkg/nes` for every stage 1 addition, using synthetic
  cartridges and the fake clock components from `system_test.go`.
- `internal/mcpserver` integration tests through `mcptest.Client`.
- ROM-based tests skip when the ROM is missing (`.nes` files are gitignored
  except the APU set). nestest and the APU ROMs give deterministic end-to-end
  checks for `run_until`, `cdl_export` and `coverage`.
- `make test` runs with `-race`; the single-lock machine design must keep it
  clean. A test that calls tools concurrently on one machine guards this.

## Risks

- Client support for revision `2026-07-28` (see the retrogolib plan).
- Hot-path cost of the bus-cycle observer. One atomic load per cycle is the
  budget; measure in stage 1 before building on it.
- Panics in emulator code. The recover boundary converts them into tool errors
  and logs them; also fix the ones that MCP use hits first (memory read on
  unhandled address, tracing on unsupported addressing mode).
- Side effects in reads. Every read tool must route through `Peek`. A test
  asserts that `read_memory` of `$2002` does not clear the vblank flag.
- Large outputs. `read_memory` caps at 4 KiB, traces page, screenshots are
  PNG not raw.
- Battery files. Default off in MCP mode; `create_machine` with
  `battery: true` reads and writes `<rom>.sav` inside roots.

## Open questions

1. Should `run_frames` accept input scripts in a compact text form
   (`"A:10-20,Right:0-60"`) in addition to the structured list? Recommendation:
   structured only in stage 2; add the text form if agents struggle.
2. Should machines persist across server restarts? Recommendation: no;
   the protocol says in-flight state is lost on restart and clients retry.
