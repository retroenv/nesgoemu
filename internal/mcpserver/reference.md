# nesgoemu MCP server

This server drives a headless NES emulator. Emulation only advances inside the
tools that run time: `step`, `run_frames`, `set_buttons` with `hold_frames`,
`run_until`, `step_over`, `step_out`, `restore`, `fork_machine`, `record_audio`,
and the analysis tools that take a `frames` count. Every other tool reads or
changes state without advancing game time.

## Machines

An emulator instance is a machine. `create_machine` loads a ROM and returns a
`machine_id`. Every other tool takes that `machine_id`.

A machine expires after 30 minutes without a call. The server also limits the
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

## Machine and display tools

| Tool | Hints | Purpose |
| --- | --- | --- |
| `create_machine` | write | Load a `rom`, optional `entrypoint` and `battery`. Returns `machine_id`, the ROM summary, and the initial CPU state. |
| `destroy_machine` | destructive | Free a machine. |
| `list_machines` | read | Machine identifiers, ROM names, frame counters, and idle time. |
| `reset` | write | Reset the machine. `hard` recreates the system. |
| `step` | write | Execute `instructions` CPU instructions. Returns the CPU state before and after. |
| `run_frames` | write | Run `frames` with optional `inputs`. |
| `cpu_state` | read | Registers, flags, cycles, frame, pending interrupts, and the PRG location of PC. |
| `read_memory` | read | A side-effect-free range for the space `cpu`, `ppu`, `oam`, `prg`, `chr`, or `ram`. The result lists `-1` for a byte that has no side-effect-free read path. |
| `write_memory` | write | Write bytes to CPU RAM or PRG-RAM. The call checks the whole range first and refuses register and ROM ranges. The write is part of the event log. |
| `set_buttons` | write | Set the buttons of one controller. `hold_frames` runs frames with the buttons held. |
| `screenshot` | read | The current frame as a PNG, with the frame number and CPU state. |
| `machine_info` | read | Mapper windows, mirroring, RAM sizes, feature usage, and battery status. |

## Debugging tools

| Tool | Purpose |
| --- | --- |
| `add_breakpoint` | Add an `exec`, `read`, `write`, or `access` breakpoint on an address range. Optional `bank`, write `value`, and `once`. Returns `id`. |
| `remove_breakpoint`, `list_breakpoints` | Manage breakpoints. The list includes hit counts. |
| `run_until` | Run until a breakpoint or a frame, instruction, cycle, or timeout limit. Returns the reason, the breakpoint id, `hit_pc` (the instruction), `hit_address`, `hit_value`, the CPU state, and the trace tail. A read-modify-write instruction such as `INC` writes the old value before the new value, so a write breakpoint hits with the old value first. |
| `disassemble` | Decode instructions from a CPU address with location objects. |
| `trace_start`, `trace_stop`, `get_trace` | Control and page the side-effect-free trace ring. Tracing is off until `trace_start`, because it slows emulation. `trace_start` clears the ring. |
| `call_stack` | Reconstructed JSR and interrupt frames from the CPU stack events. A frame has the `return_address` where execution continues and the location of the calling instruction. |
| `step_over`, `step_out` | Run over the current call or until the current function returns. The reason `returned` means that the return completed. A breakpoint of the machine stops the run first. A recursive call that returns to the same address on a deeper stack level does not end the run. |
| `checkpoint` | Name the current point of the event log, and write `<machine>.events.json` inside the first root. Returns the `checkpoint_id`. |
| `restore` | Replay the event log to a `checkpoint_id` on the same machine. The `frame` argument selects a frame instead, and includes every event of that frame. Events after the target are dropped. |
| `fork_machine` | Create a new machine from a `checkpoint_id` or a `frame` of another machine. Without both, it copies the current state. |
| `restore_machine` | Recreate a machine from a ROM and an event log file after a server restart. The file stores the ROM checksums, the entrypoint, and the frame. A different ROM is refused. |
| `event_log` | Page the recorded inputs, releases, pokes, and resets. |

A replay re-runs the machine from power-on and applies each event at the CPU
cycle where it happened. `step` and `run_until` calls do not need events,
because the replay reaches the same cycles. A hard reset clears the log.

## Analysis tools

| Tool | Purpose |
| --- | --- |
| `cdl_start`, `cdl_stop`, `cdl_reset` | Control code and data logging. |
| `cdl_export` | Write a FCEUX-format `.cdl` file inside the roots for retrodisasm. |
| `coverage` | Executed and read bytes per PRG bank. |
| `bank_state` | Current PRG and CHR windows and the bank switches since the last call. |
| `find_accessors` | Run frames with a bus observer on a range. Each result is one instruction with its read count, write count, and the first distinct values it wrote. |
| `memory_search` | Cheat search with `start`, `filter`, and `list`. Filters: `equal`, `changed`, `unchanged`, `increased`, `decreased`, `value`. |
| `ram_diff` | Snapshot RAM, run frames, and return the changed bytes. |
| `watch_writes` | Write log of a range with frame and scanline. |

## PPU and APU tools

| Tool | Purpose |
| --- | --- |
| `ppu_state` | PPU registers, render position, and VRAM addresses. |
| `nametables` | The four nametables as tile grids, and a rendered PNG on request. |
| `palette`, `oam` | Decoded palette entries and sprites. |
| `pattern_tables` | Render the CHR pattern tables as a PNG. |
| `apu_state` | DMC request and IRQ, and expansion audio. |
| `record_audio` | Run the machine and write `samples` mono 16-bit samples at 44100 Hz (up to 60 seconds) to a WAV file inside the roots. |

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
2. `run_frames` with the inputs for the current level.
3. `screenshot` and `read_memory` to inspect the result.
4. Repeat until the goal is reached.
