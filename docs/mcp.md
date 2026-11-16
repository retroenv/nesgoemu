# MCP server

nesgoemu provides a headless NES emulator through the Model Context Protocol
(MCP). The server supports protocol revision `2026-07-28`, uses stdio by
default, and starts with `server/discover`. It does not accept the legacy
`initialize` handshake. The client must support this revision.

## Start the server

Configure the MCP client to run:

```text
nesgoemu mcp -root /path/to/roms
```

Use `nesgoemu` as the command and `mcp -root /path/to/roms` as its arguments
when the client has separate command and argument fields.

The `-root` flag limits ROM, save, log, and output paths to the given
directories. Repeat it to allow more directories. A relative path starts in
the first root. With no root, the server uses the current working directory.
Symbolic links cannot lead outside the roots.

## Configuration and safety

```text
nesgoemu mcp [-transport stdio|http] [-addr 127.0.0.1:0] [-root DIR]
             [-max-machines N] [-machine-ttl 30m] [-debug]
```

The server logs to standard error. Standard output carries MCP messages only.
HTTP has no authentication. Keep its address on loopback unless access from
other hosts is intended. Any process that can reach the address can call the
tools.

The server accepts only regular input files. It rejects named pipes, devices,
and paths that escape the roots. File-producing tools write a temporary file
and rename it on success. They replace only regular files. The server does not
write battery saves; `create_machine` can load `<rom>.sav` when battery loading
is enabled.

## Work with a machine

Create a machine from a ROM. Use the returned `machine_id` for later calls.
Run frames with controller inputs, then inspect the machine or capture a
screenshot and memory ranges:

```json
{
  "machine_id": "...",
  "frames": 60,
  "inputs": [{"frame": 0, "controller": 1, "buttons": ["A", "Right"]}],
  "capture": {
    "screenshot": true,
    "memory": [{"space": "ram", "address": "$0010", "length": 16}]
  }
}
```

`run_frames` returns the run result, captured memory, and a PNG image. Capture
does not advance the machine. `inspect_machine`, `read_memory`, and
`screenshot` also inspect the current state without advancing it.

The tool groups cover machine control and execution, inspection, debugging,
analysis, graphics, and audio. Debugging includes breakpoints, tracing,
disassembly, call stacks, and event-log checkpoints. Analysis includes code
and data logging, coverage, memory searches, and bus access tools. Graphics
tools return nametables and pattern tables. Audio recording writes a WAV file.

`memory_search` uses independent searches. Keep the `search_id` returned by
`start` and pass it to each `filter`, `list`, and `release` call. The reference
describes when a search handle expires and how to page results.

## Tool reference

The [`MCP tool reference`](../internal/mcpserver/reference.md) describes
arguments, results, limits, and effects. The server also provides that reference
as the `nesgoemu://docs/mcp` resource. The tool definitions publish argument
schemas and structured output schemas.
