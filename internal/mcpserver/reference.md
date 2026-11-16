# nesgoemu MCP server

This server drives a headless NES emulator. Emulation only advances inside the
tools that run time: `step`, `run_frames`, `set_buttons` with `hold_frames`,
`run_until`, `restore`, `fork_machine`, `record_audio`,
and the analysis tools that take a `frames` count. Every other tool reads or
changes state without advancing game time.

## Machines

An emulator instance is a machine. `create_machine` loads a ROM and returns a
`machine_id`. Every other tool takes that `machine_id`.

A machine expires after the idle limit (`-machine-ttl`, 30 minutes by default)
without a call. The server also limits the
number of live machines. When a call reports an unknown `machine_id`, call
`create_machine` again.

`list_machines` shows the live machines with their frame counters and idle
time.

## Addresses

An address accepts an integer, `"$C000"`, or `"0xC000"`. The `prg` and `chr`
spaces of `read_memory` are physical ROM data. They use the `offset` argument,
because a ROM can be larger than 64 KiB.

A relative file path starts at the first `-root` directory. A path that leaves
the roots, also through a symbolic link, is refused.

Every `timeout` argument takes a duration such as `"10s"`. The default is 10
seconds and the limit is 2 minutes.

The deadline includes the time in the machine queue. A timeout before execution
returns a tool error without a state change. `record_audio` defaults to 2 minutes.

## Machine and display tools

| Tool | Hints | Purpose |
| --- | --- | --- |
| `create_machine` | write | Load a `rom`, optional `entrypoint` and `battery`. Returns `machine_id`, the ROM summary, and the initial CPU state. |
| `destroy_machine` | destructive | Free a machine. |
| `list_machines` | read | Machine identifiers, ROM names, frame counters, and idle time. |
| `reset` | write | Reset the machine. `hard` recreates the system. |
| `step` | write | Required `mode`: `instructions`, `over`, or `out`. Returns `before`, `after`, `reason`, `instructions`, `cycles`, `frames`, breakpoint details, and the trace tail. Only mode `instructions` accepts an instruction count (default 1, range 1 to 1000000). |
| `run_frames` | write | Run `frames` with optional `inputs`, ignoring breakpoints. Optional `capture` returns memory reads in the structured result and a PNG as image content. Capture does not advance time. |
| `inspect_machine` | read | Return selected sections from one stopped state: `cpu`, `ppu`, `apu`, `cartridge`, `banks`, `palette`, `oam`. Default `cpu`. Omitted sections have no result field. |
| `read_memory` | read | A side-effect-free range for the space `cpu`, `ppu`, `oam`, `prg`, `chr`, or `ram`. The result lists `-1` for a byte that has no side-effect-free read path. |
| `write_memory` | write | Write from 1 to 4096 bytes to CPU RAM or `$6000-$7FFF`. The call checks the whole range first and refuses hardware register and ROM ranges. A write to `$6000-$7FFF` goes through the cartridge bus, so a mapper that decodes registers there sees a register write. The write is part of the event log. |
| `set_buttons` | write | Set the buttons of one controller. `hold_frames` runs frames with the buttons held. |
| `screenshot` | read | The current frame as a PNG, with the frame number and CPU state. |

## Inspection and execution

`inspect_machine` accepts a `sections` array with one to seven unique entries.
It returns only the selected named fields and a common `frame` count:

| Section | Contents |
| --- | --- |
| `cpu` | Registers, flags, cycles, interrupts, and the PRG location of PC. |
| `ppu` | Registers, render position, and VRAM addresses. |
| `apu` | Available DMC request and IRQ state, and expansion audio. |
| `cartridge` | Mapper identity, ROM path, RAM sizes, feature usage, and battery status. |
| `banks` | PRG and CHR windows, mirroring, and the last 256 writes to `$8000-$FFFF`. |
| `palette` | The 32 palette entries as RGB colors, including backdrop mirrors. |
| `oam` | The 64 sprite entries with position, tile, and attributes. |

For `step`, mode `instructions` ignores breakpoints. Mode `over` runs the
current JSR to its return. Mode `out` runs until the current call frame returns.
The run to a return honors machine breakpoints and checks the stack depth.
Without a JSR or a call frame, `over` or `out` executes one instruction and
ignores breakpoints. The reason `returned` reports a completed return.

`run_frames.capture` accepts `screenshot`, optional `scale` (1 to 4, default 1),
and `memory`. Scale requires `screenshot: true`. Memory accepts at most eight
ranges. Each range uses `space`, optional `address` or physical `offset`, and
optional `length` (1 to 4096, default 256), as `read_memory` does.
All capture arguments are checked before emulation. The result identifies the
stop frame. If image capture fails after execution, the tool error states how
many frames ran and the frame where the machine stopped.

## Debugging tools

| Tool | Purpose |
| --- | --- |
| `add_breakpoint` | Add an `exec`, `read`, `write`, or `access` breakpoint on an address range. Optional `bank`, write `value`, and `once`. Returns `id`. |
| `remove_breakpoint`, `list_breakpoints` | Manage breakpoints. The list includes hit counts. |
| `run_until` | Run until a breakpoint or a frame, instruction, cycle, or timeout limit. Returns the reason, the breakpoint id, `hit_pc` (the instruction), `hit_address`, `hit_value`, the CPU state, and the trace tail. A read-modify-write instruction such as `INC` writes the old value before the new value, so a write breakpoint hits with the old value first. |
| `disassemble` | Decode instructions from a CPU address with location objects. |
| `trace_start`, `trace_stop`, `get_trace` | Control and page the side-effect-free trace ring. Tracing is off until `trace_start`, because it slows emulation. `trace_start` clears the ring. |
| `call_stack` | Reconstructed JSR and interrupt frames from the CPU stack events. A frame has the `return_address` where execution continues and the location of the calling instruction. |
| `checkpoint` | Name the current point of the event log, and write `<machine>.events.json` inside the first root. Returns the `checkpoint_id`. The point includes the CPU cycle, so a checkpoint between two instructions of a frame restores exactly. |
| `restore` | Replay the event log to a `checkpoint_id` on the same machine. The `frame` argument selects a frame instead, and includes every event of that frame. Events after the target are dropped. |
| `fork_machine` | Create a new machine from a `checkpoint_id` or a `frame` of another machine. Without both, it copies the current state, also between two instructions of a frame. A `frame` target ends at the frame boundary. `restore`, `fork_machine`, and a hard `reset` keep the call stack but start a new CDL recorder that is stopped. |
| `restore_machine` | Recreate a machine from a ROM and an event log file after a server restart. The file stores the ROM checksums, the entrypoint, and the frame. A different ROM is refused. |
| `event_log` | Page the recorded inputs, releases, pokes, and resets. |

A replay re-runs the machine from power-on and applies each event at the CPU
cycle where it happened. `step` and `run_until` calls do not need events,
because the replay reaches the same cycles. A hard reset clears the log.

The machine stores a SHA-256 hash of the ROM, including its header. A replay
or hard reset refuses a changed ROM. New event logs store the hash for
`restore_machine`. Older event logs use the PRG and CHR checksums.

New event logs also store the initial battery data, including an absent save.
Replays and hard resets use this data instead of the current `.sav` file.
Older event logs with `battery: true` are refused because they lack this data.

A machine retains at most 65,536 replay events and 1,024 checkpoints. The
history budget is 16 MiB. Each event uses 256 budget bytes, plus four bytes for
each recorded memory-write value. A call that cannot fit all its inputs or
writes is refused before it changes the machine. A restore releases later
history; a hard reset clears all history and checkpoints. A checkpoint file
must fit the 64 MiB file limit. A larger snapshot is refused before a handle
is created or a file is replaced.

## Analysis tools

| Tool | Purpose |
| --- | --- |
| `cdl_set_enabled` | Required boolean `enabled`. Enable or stop code and data logging without clearing flags. Returns `enabled` and the count of fixed 8 KiB PRG `segments`. Repeated calls with the same value have no additional effect. |
| `cdl_reset` | Clear the recorded code and data flags. The recording state stays unchanged. |
| `cdl_read` | Read raw and decoded flags from `prg` or `chr` with a physical `offset` (default 0) and `length` (default 256, maximum 4096). The complete range must fit in the ROM. This tool does not advance emulation. |
| `cdl_export` | Write a FCEUX-format `.cdl` file inside the roots for retrodisasm. The path must end in `.cdl`. The file is replaced only when it is a regular file. |
| `coverage` | Executed and read bytes in fixed 8 KiB PRG segments. Returns `segments`, with a zero-based `segment` index and byte `size` in each entry. |
| `find_accessors` | Run frames with a bus observer on a range. Each result identifies the instruction by CPU address, PRG bank, and physical offset at execution. It reports the read count, write count, and first distinct values written. The `run` object tells whether the run finished or timed out. |
| `memory_search` | `start` returns `search_id`. `filter`, `list`, and `release` require that handle. Filters: `equal`, `changed`, `unchanged`, `increased`, `decreased`, `value`. Only the `value` filter accepts and requires `value`. |
| `ram_diff` | Snapshot RAM, run frames, and return the changed bytes and a `run` object. |
| `watch_writes` | Write log of a range with frame and scanline. At most `limit` events (default 1000, maximum 10000). The run stops after the frame that reaches the limit and the result has `truncated: true`. The `run` object tells why the run ended. |

Each machine permits 16 independent memory searches. A search handle lasts
until release, restore, hard reset, or machine expiry or destruction. A fork
starts with no searches. A soft reset keeps the searches. Unknown or released
handles are refused before a run. `list` accepts `limit` (default 50, maximum
10000) and `offset` (default 0). `filter` accepts optional `frames` and `inputs`
to run the machine before it compares values with that search's baseline.

CHR logging records program reads through `$2007` and background and sprite
pattern fetches when the mapper reports a physical CHR-ROM offset. The
`drawn_on_screen` flag includes prefetches and empty sprite slots. It does not
prove that a pixel was displayed. CHR-RAM has no ROM flags.

PRG flags retain both code and data when a byte has both uses. Mapping bits
identify the CPU window of the last access. Dummy CPU reads do not add data
flags. Indirect data flags apply to `(dd,X)` and `(dd),Y` accesses.

Exports use raw FCEUX CDL: PRG flags, then CHR flags, with no header. Reserved
bits are cleared. The legacy Mesen `sub_entry_point` annotation remains in
`cdl_read` results but is omitted from exports. Mesen 2 `CDLv2` files use a
different format and are not supported by the shared loader.

## PPU and APU tools

| Tool | Purpose |
| --- | --- |
| `nametables` | The four nametables as tile grids, and a rendered PNG on request. |
| `pattern_tables` | Render the CHR pattern tables as a PNG. |
| `record_audio` | Run the machine and write `samples` mono 16-bit samples at 44100 Hz (up to 60 seconds) to a WAV file inside the roots. The path must end in `.wav`. Audio that earlier runs queued is dropped first, so the file holds only new audio. The `timeout` argument defaults to 2 minutes. |

## Resources

| URI | Contents |
| --- | --- |
| `nesgoemu://docs/mcp` | This reference. |
| `nesgoemu://machines/{machine_id}/screen` | The current frame of a machine as a PNG. |

## Inputs

`run_frames`, `run_until`, `set_buttons`, `memory_search`, and `ram_diff` accept
button names: `A`, `B`, `Select`, `Start`, `Up`, `Down`, `Left`, `Right`. An
input names the controller (1 or 2) and the buttons to press. The buttons stay
pressed until another input changes the controller.

## Typical loop

1. `create_machine` with a ROM path.
2. `run_frames` with the inputs and a `capture` request for a screenshot or memory ranges.
3. Use the returned observations to select the next inputs.
4. Use `inspect_machine`, `screenshot`, or `read_memory` for additional inspection.
