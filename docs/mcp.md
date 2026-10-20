# MCP server

nesgoemu can run as a Model Context Protocol (MCP) server. An AI agent then
loads a ROM, runs frames, reads memory, and takes screenshots without a
graphical interface.

The server implements protocol revision `2026-07-28` and uses the stdio
transport by default.

## Client support

Only Claude Code 2.1.292 or later connects with default settings. The server
uses the modern `server/discover` opening and does not implement the legacy
`initialize` handshake. Codex CLI, OpenCode, Gemini CLI, and Cline still open
with `initialize` and cannot connect. A user who sets
`MCP_PROTOCOL_NEGOTIATION=legacy` in Claude Code also cannot connect.

## Claude Code setup

Add the server with the CLI and allow the directory that holds the ROMs:

```bash
claude mcp add --transport stdio nesgoemu -- nesgoemu mcp -root /path/to/roms
```

Or add an entry to `.mcp.json` in the project:

```json
{
  "mcpServers": {
    "nesgoemu": {
      "command": "nesgoemu",
      "args": ["mcp", "-root", "/path/to/roms"]
    }
  }
}
```

The `-root` flag restricts every ROM, save, log, and output path to that
directory, also through symbolic links. A relative path starts at the first
root. Repeat the flag to allow more than one directory. With no `-root` flag,
the server allows the current working directory.

## Command line

```
nesgoemu mcp [-transport stdio|http] [-addr 127.0.0.1:0] [-root DIR]
             [-max-machines N] [-machine-ttl 30m] [-debug]
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `-transport` | `stdio` | Transport: `stdio` or `http`. |
| `-addr` | `127.0.0.1:0` | Listen address for the HTTP transport. |
| `-root` | working directory | Directory that holds ROMs. Repeatable. |
| `-max-machines` | `4` | Maximum number of live machines. |
| `-machine-ttl` | `30m` | Idle limit for a machine. |
| `-debug` | `false` | Enable debug logging. |

The server logs to standard error. Standard output carries MCP messages only.

## Tools

The server exposes five tool groups:

- **Machine and display**: `create_machine`, `destroy_machine`,
  `list_machines`, `reset`, `step`, `run_frames`, `cpu_state`, `read_memory`,
  `write_memory`, `set_buttons`, `screenshot`, `machine_info`.
- **Debugging**: breakpoints, `run_until`, `disassemble`, trace ring control,
  `call_stack`, `step_over`, `step_out`, and deterministic checkpoints by
  event-log replay.
- **Analysis**: code and data logging, coverage, bank state, accessor search,
  cheat search, RAM diff, and write watch.
- **PPU and APU**: register snapshots, nametables, palette, OAM, pattern
  tables, APU state, and WAV recording.
- **Resources**: `nesgoemu://docs/mcp` and
  `nesgoemu://machines/{machine_id}/screen`.

The `nesgoemu://docs/mcp` resource carries the full tool and argument
reference.

## Usage

A typical session:

1. `create_machine` with a ROM path. The result contains a `machine_id`.
2. `run_frames` with the controller inputs for the current level.
3. `screenshot` and `read_memory` to inspect the result.
4. Repeat until the goal is reached.

Emulation only advances inside the tools that run time: `step`, `run_frames`,
`set_buttons` with a nonzero `hold_frames`, `run_until`, `step_over`,
`step_out`, `restore`, `fork_machine`, `record_audio`, and the analysis tools
that take a frame count. Every other tool reads or changes state without
advancing time, so runs are deterministic.

Tracing is off until `trace_start`, because it slows emulation several times.
`run_until` returns a trace tail only while tracing is on.

`checkpoint` writes the event log of inputs, pokes, and resets to a file.
`restore` and `fork_machine` replay it to a `checkpoint_id` and give a machine
with the same state. `restore_machine` rebuilds a machine from that file after
a server restart.

The server sends a full tool and resource reference as the
`nesgoemu://docs/mcp` resource, and the tool descriptions carry the same
information.

## Display palette

`create_machine` has an optional `palette` argument. It is the path of an RGB
`.pal` file with 192 or 1536 bytes, inside the allowed roots. Without it, the
machine uses the default palette that [usage](usage.md#display-palette)
describes.

The palette applies to screenshots and image resources. It stays after resets,
forks, and event-log replay. Event logs contain the palette bytes, so a restore
does not need the original file.

A screenshot shows the last completed frame. Each pixel uses the PPUMASK
emphasis bits that were active when the PPU drew it. A screenshot does not
advance emulation.
