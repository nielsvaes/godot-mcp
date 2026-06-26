# gdcli — a Go CLI for Godot MCP

`gdcli` drives a live Godot editor from the shell. It exposes every Godot MCP
tool as a subcommand, backed by an auto-managed daemon that hosts the WebSocket
bridge the Godot addon connects to. No Node required; the Godot addon is
unchanged.

## Build

```bash
go build -o gdcli ./cli
```

## Use

```bash
gdcli tools                       # list all tools
gdcli describe add-node           # show a tool's flags
gdcli status                      # is Godot connected?
gdcli add-node --name Player --type CharacterBody2D --parent /root/Main
gdcli read-file --path res://player.gd --raw | jq .content
gdcli call send_input --json '{"event":{"type":"key","key":"Space","pressed":true}}'
gdcli stop                        # stop the background daemon
```

The first tool call auto-starts the daemon (`gdcli serve`). If a Node
`godot-mcp-server` primary is already running (e.g. launched by an AI client),
`gdcli` routes through it instead of starting its own daemon.

## Output and exit codes

- Tool results print as JSON to stdout (pretty by default; `--raw`/`-r` for
  compact, pipeable JSON).
- Errors print to stderr with a non-zero exit code: `2` tool error, `3` Godot
  not connected, `4` daemon unreachable, `5` timeout.

## Environment

`GODOT_MCP_PORT` (6505), `GODOT_MCP_HTTP_PORT` (6506),
`GODOT_MCP_TIMEOUT_MS` (30000), `GODOT_MCP_IDLE_TIMEOUT_MS` (30000).

## Tool contract

All tool schemas live in `/schemas/tools.json`, the single source of truth
shared with the Node server. `gdcli` embeds it at build time.
