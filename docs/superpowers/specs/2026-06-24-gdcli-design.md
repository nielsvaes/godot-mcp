# Design: `gdcli` — a Go CLI front-end for Godot MCP

- **Date:** 2026-06-24
- **Status:** Approved (pending spec review)
- **Author:** Niels Vaes (with Claude)

## Context & problem

This repo ships an MCP server (`mcp-server/`, Node/TypeScript) that lets AI clients
drive a live Godot 4.x editor. We want an alternative front-end: a **Go CLI** that
exposes the same capabilities, so the tooling is scriptable from a shell and
"feels cleaner" than an MCP server for non-agent use.

### The key insight that shapes everything

The Node MCP server is **thin**. It defines tool *schemas* and routes calls; it
does **not** contain the tool logic. Every tool's real work (`add_node`,
`create_scene`, `take_screenshot`, …) executes as **GDScript inside the Godot
editor process**, because it needs live access to `EditorInterface`, the scene
tree, and the running game. The Node server reaches that logic only by sending
JSON over a WebSocket to the Godot plugin.

Therefore a CLI **cannot "port the functionality"** — the functionality lives in
Godot and stays there. A CLI is a *new front-end* that talks to the same Godot
bridge, exactly as the MCP server is. This makes the project far smaller than a
rewrite, but forces one architectural decision: **who hosts the listener Godot
dials into.**

### Why a listener is unavoidable

Godot's plugin (`addons/godot_mcp/mcp_client.gd`) is a WebSocket **client** — it
dials *outward* to `ws://127.0.0.1:6505` and waits. Godot itself listens on
nothing. So "talk to Godot directly" is not possible with the current plugin;
something must listen on `:6505` for Godot to connect to. (Godot's
`--headless --script` is not an escape hatch: it runs a *separate* instance and
can only touch project files, not the live editor session or running game.)

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Who is the listener | **Go daemon**, plugin untouched | Holds the Godot connection warm → fast commands; zero risk to the working addon; no Node dependency |
| Node dependency | **None** — pure Go binary | The "cleaner single binary" goal |
| Command surface | **Schema-driven dynamic** | Typed flags + `--help` + validation auto-generated from a shared schema; new tools need no Go changes; single source of truth |
| Schema source of truth | **`schemas/tools.json`** at repo root | Consumed by both Go (embed) and the existing Node server (import) → zero drift |
| Binary name | **`gdcli`** | `godot` collides with the engine on PATH |
| Daemon idle-shutdown | **Yes** | Auto-cleanup; daemon need not outlive use |
| Browser visualizer in v1 | **No** | Browser/interactive, not CLI-shaped; large port; Node still serves it |
| MCP-over-stdio in Go in v1 | **No** | Biggest scope item; deferred. Node still serves AI clients |

## Architecture

A single Go binary, `gdcli`, with two modes:

- **`gdcli serve`** — long-lived daemon that *is* the bridge: hosts the WebSocket
  server on `:6505` that Godot dials into and holds the connection open.
- **`gdcli <tool> [flags]`** — thin, fast invocation that sends one tool call and
  exits.

The daemon speaks the **exact WebSocket wire protocol the Node server uses today**
(`godot_ready` / `tool_invoke` / `tool_result` / `ping` / `client_status`), so the
**Godot addon needs zero changes**.

### Control channel & coexistence with Node

The CLI reaches the bridge over the **same `:6506` HTTP contract the Node primary
already exposes**, which yields free coexistence:

```
gdcli add-node ...
   │
   ├─ probe GET 127.0.0.1:6506/health
   │     ├─ healthy bridge present (Go daemon OR Node primary)? → use it
   │     └─ nothing there? → spawn `gdcli serve` (detached), await health, use it
   │
   └─ POST :6506/tool {name, args} ──▶ bridge ──WS :6505──▶ Godot ──▶ result
```

- If an AI client already has a Node primary running, it owns `:6505/:6506`; the
  CLI detects it via `/health` and routes through it instead of spawning a daemon.
- If nothing is running, `gdcli serve` becomes the bridge.
- `gdcli` and the Node server are therefore **mutually exclusive as bridges** —
  exactly one owns the Godot connection at a time, matching today's behavior.
- Control channel is **localhost TCP** (not a unix socket): portable to Windows
  and wire-compatible with the Node primary.
- The user never starts anything by hand.

### Daemon internals (the essential parts of the Node bridge, ported to Go)

Ports only the essentials of `godot-bridge.ts` + the primary half of `index.ts`:

- WebSocket server on `:6505` (host `127.0.0.1`); one `editor` slot + one
  `runtime` slot, distinguished by the `role` field in the `godot_ready` hello.
- UUID request/response correlation with a 30s timeout (`GODOT_MCP_TIMEOUT_MS`).
- **Role routing** mirroring `RUNTIME_ONLY_TOOLS`
  (`take_screenshot`, `send_input`, `query_runtime_node`, `get_runtime_log`) plus
  the conditional `list_signal_connections` rule, so runtime/game tools reach the
  running game and editor tools reach the editor.
- 10s keepalive ping; idle-shutdown after a no-activity window (default 30s,
  `GODOT_MCP_IDLE_TIMEOUT_MS`) once no Godot and no recent HTTP activity remain.
- `:6506` HTTP API: `GET /health` → `{server, version, toolCount}`,
  `POST /tool {name, args}`.
- **Dropped from the port:** multi-AI-client proxy fan-out, MCP-over-stdio, the
  `:6510` browser visualizer.

## Schema contract

- Extract the tool schemas currently inline in `mcp-server/src/tools/*.ts` into a
  language-neutral **`schemas/tools.json`** at repo root: an array of
  `{ name, category, target: "editor"|"runtime", description, inputSchema }`.
- **TS side:** `tools/*.ts` import that JSON and re-export `ToolDefinition[]`.
  Node's behavior must be byte-for-byte unchanged — **the existing vitest suite
  passing is the acceptance bar for the refactor.**
- **Go side:** `go:embed schemas/tools.json`, then build a cobra subcommand per
  tool at startup.
- **Drift guard:** a test asserting `tools.json` names == the addon's `_tool_map`
  keys, so the schema layer, the Node server, and the GDScript implementation
  cannot silently diverge.

## CLI UX

- **Flag generation:** each `inputSchema` property → a flag. `string`/`number`/
  `boolean` map directly; `required` is enforced; the property `description`
  becomes the flag's help text. Nested objects/arrays (e.g. `send_input`'s event
  object) take a JSON value: `--event '{...}'`.
- **Raw escape hatch:** `gdcli call <tool> --json '{...}'` invokes any tool with a
  raw JSON args blob.
- **Built-ins:** `gdcli tools` (list), `gdcli describe <tool>` (schema + flags),
  `gdcli status` (→ `get_godot_status`), `gdcli serve`, `gdcli stop`.

### Output & exit codes (the "scriptable" part)

- Godot's JSON result → **stdout**, pretty by default; `-r/--raw` for compact JSON
  to pipe into `jq`.
- A tool error (`isError`) → structured error to **stderr** + a **non-zero exit
  code**, so chains like `gdcli run-scene && gdcli take-screenshot` behave.
- Distinct, documented exit codes for: tool error, Godot-not-connected,
  daemon-unreachable, timeout.

## Repo layout

```
schemas/tools.json            # shared contract (new, repo root)
cli/                          # new Go module
  main.go
  cmd/                        # cobra commands + dynamic generation
  internal/
    bridge/                   # WebSocket server, role routing, request correlation
    daemon/                   # serve mode, lifecycle, idle-shutdown, :6506 HTTP API
    client/                   # probe/spawn, POST /tool, output formatting, exit codes
    schema/                   # load embedded JSON → cobra commands + flag parsing
mcp-server/src/tools/*.ts     # refactored to import ../../schemas/tools.json
```

## Testing

- **Go bridge:** lifecycle, editor/runtime role routing, request correlation and
  timeout — tested against a fake Godot WS client on a high port. Mirrors the
  existing vitest philosophy (real servers, no network mocks).
- **Go schema→flag generation:** scalar/required/nested mapping, raw `--json`.
- **Drift parity test:** `schemas/tools.json` names == addon `_tool_map` keys.
- **TS regression:** prove the schema extraction left `allTools` identical and the
  existing suite green.

## Scope

**In v1:** the `gdcli` binary; `serve` daemon (WebSocket bridge + `:6506` HTTP +
role routing + idle-shutdown); schema-driven CLI over **all** tool calls including
runtime/game tools; the shared `schemas/tools.json` contract and the TS refactor
to consume it; coexistence with a running Node primary.

**Out of v1 (remain the Node server's job):** the `:6510` browser visualizer
(`gdcli map-project` emits JSON only); MCP-over-stdio in Go; multi-AI-client proxy
fan-out.

## Future phases (not committed)

1. `gdcli mcp` — MCP-over-stdio in Go, the step that fully replaces the Node
   server with one binary for both CLI and AI clients.
2. Port the `:6510` browser visualizer to the Go daemon.
3. Shell completions generated from the schema.
