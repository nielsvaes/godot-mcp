# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Godot MCP gives MCP-compatible AI clients (Claude Desktop, Cursor, Claude Code, etc.) — and a shell — direct control over a running Godot 4.x editor. The pieces that talk over a WebSocket:

- **`addons/godot_mcp/`** — a GDScript editor plugin that runs inside Godot. Connects to a server as a WebSocket *client* and *executes* the tools. Godot itself listens on nothing; it always dials out to whatever hosts the bridge.
- **`mcp-server/`** — a Node.js/TypeScript MCP server (published to npm as `godot-mcp-server`). Speaks MCP over stdio to the AI client; hosts the WebSocket bridge Godot dials into.
- **`cli/`** — `gdcli`, a pure-Go CLI front-end (alternative to the MCP server). Same job, scriptable from a shell. Its `serve` mode hosts the *same* bridge; its tool commands are a thin client to it. Coexists with the Node server (see below).
- **`schemas/tools.json`** — the single source of truth for tool *schemas*, shared by the Node server (build-time codegen) and `gdcli` (`go:embed`).

Everything else (CHANGELOG, release-notes, screenshots, icon, configs) is supporting material.

## Commands

**MCP server** (run from `mcp-server/`):

```bash
npm install          # install deps
npm run build        # sync-schemas + tsc + bundle the visualizer into dist/ (also runs on `npm install` via prepare)
npm run watch        # sync-schemas + tsc --watch (does NOT rebuild the visualizer)
npm run build:visualizer  # rebuild only dist/visualizer.html
npm run dev          # build + run the server
npm test             # sync-schemas + vitest run (all tests once)
npm run test:watch   # vitest watch mode
npx vitest run src/tests/godot-bridge.test.ts          # run a single test file
npx vitest run -t "rejects a second simultaneous"      # run tests matching a name
```

Tests use **Vitest against real servers on high ports (16505+)** — networking code is not mocked. See `TESTING.md` for the full automated + manual test checklist (manual tests need a live Godot editor and are spot-checked per release).

**Go CLI** (run from the repo root — the Go module lives there):

```bash
go build -o gdcli ./cli   # build the binary (gitignored)
go test ./...             # all Go tests (real servers on ephemeral ports, no mocks)
go test ./cli/internal/bridge/ -race   # the concurrency-sensitive package
go vet ./cli/... ./schemas/
./gdcli tools             # list tools (offline)
./gdcli describe add-node # show a tool's flags (offline)
```

There is no build/lint step for the GDScript addon — it is loaded directly by Godot. The plugin targets Godot **4.2+**.

## Architecture

### Connect-or-spawn (primary / proxy)

Each AI client launches its *own* `godot-mcp-server` process, but only one can own the Godot connection. On startup the server probes `HTTP_PORT` (6506) for an existing primary:

- **No primary found → PRIMARY mode.** Owns three listeners: WebSocket bridge on **6505** (for Godot), HTTP API on **6506** (for proxies), and the visualizer HTTP server on **6510** (for the browser). Stays alive after its own client disconnects; only an idle timeout with no Godot + no proxy activity shuts it down.
- **Primary found → PROXY mode.** Holds no Godot connection; forwards every tool call to the primary over HTTP (`POST /tool`) and exits when its stdin closes.
- A primary whose **version or tool count** differs from the new process is treated as stale and replaced (kill + become primary). This is why version numbers must stay in sync (see below).

Entry point and all of this orchestration live in `mcp-server/src/index.ts`. `executeToolCall()` there is the shared path used by both the primary's MCP handler and its HTTP API.

### gdcli (the Go CLI) and coexistence with the Node server

`gdcli` is a second front-end onto the same bridge, not a port of the tool logic (which lives in GDScript and stays there). It has two modes (`cli/cmd/`):

- **`gdcli serve`** — a daemon that hosts the WebSocket bridge on **6505** plus an HTTP control API on **6506** (`GET /health`, `POST /tool`, `POST /shutdown`), with the same 10s ping and idle-shutdown behavior as the Node server. The bridge itself is `cli/internal/bridge/` (a Go re-implementation of `GodotBridge`, byte-compatible on the wire); the daemon is `cli/internal/daemon/`.
- **`gdcli <tool> [flags]`** — a thin client (`cli/internal/client/`) that probes `:6506/health`; if a healthy bridge is up (Go daemon **or** a Node primary) it reuses it, otherwise it spawns `gdcli serve` detached. Then it `POST`s the call and prints the result.

Coexistence is **one-directional**: `gdcli`'s probe is lenient (any 200 + non-empty `version` counts) and it never evicts a primary, so it routes cleanly through a running Node server. The reverse is not true — the Node server's startup *does* evict a primary whose version/tool-count differs, so launching the Node server while a `gdcli` daemon is up will replace it. `gdcli` answers `get_godot_status` locally and does **not** implement `get_guide` or the visualizer.

### Tool call flow

```
AI client --MCP/stdio--> server (proxy? --HTTP--> primary) --WS 6505--> mcp_client.gd ...
gdcli <tool> --HTTP :6506--> bridge (gdcli daemon OR Node primary) --WS 6505--> mcp_client.gd ...
  --> tool_executor.gd (routes by name via _tool_map) --> tools/<category>.gd --> result back up
  (runtime tools instead --> mcp_runtime.gd `_dispatch` inside the launched game)
```

### Editor vs Runtime connections

The WebSocket bridge (`src/godot-bridge.ts` / `GodotBridge`) accepts **two** Godot connections distinguished by a `role` field in their `godot_ready` hello:

- `editor` — the editor plugin (`addons/godot_mcp/mcp_client.gd`). Handles most tools.
- `runtime` — the `MCPRuntime` autoload (`addons/godot_mcp/runtime/mcp_runtime.gd`) that lives inside the *user's launched game*. The plugin auto-registers this autoload on `_enable_plugin()` and removes it on `_disable_plugin()`.

`GodotBridge.routeIsRuntime()` decides where each tool goes. `RUNTIME_ONLY_TOOLS` (`take_screenshot`, `send_input`, `query_runtime_node`, `get_runtime_log`) and `list_signal_connections` with `source=runtime` go to the runtime; everything else goes to the editor.

### Tools: schemas/tools.json is the source of truth; the GDScript handlers are the implementation

A tool has a *schema* (one entry in `schemas/tools.json`: `{ name, category, target, description, inputSchema }`) and an *implementation* (a GDScript handler). Adding or changing a tool means editing **both**:

1. **Schema (what callers see):** edit `schemas/tools.json` directly — it is hand-edited source, not generated. Two consumers read it:
   - **Node server:** `mcp-server/scripts/sync-schemas.mjs` runs before every build/test and generates `src/tools/generated-tools.ts` (gitignored) — that's what `src/tools/index.ts` re-exports as `allTools`. The old per-category `src/tools/*-tools.ts` files are gone.
   - **gdcli:** `schemas/embed.go` bakes the JSON into the binary via `go:embed`; `cli/internal/schema` parses it and builds one cobra subcommand + typed flags per tool at startup.
2. **Implementation (what runs):** `addons/godot_mcp/tools/*.gd`, registered in the `_tool_map` dictionary in `addons/godot_mcp/tool_executor.gd` (editor tools), or — for the 4 runtime tools — dispatched in `addons/godot_mcp/runtime/mcp_runtime.gd`'s `_dispatch`.

A schema entry with no handler (or vice-versa) silently fails to route. `cli/internal/schema/drift_test.go` guards this: it asserts every `editor`-target tool has a `_tool_map` entry and every `runtime`-target tool has an `mcp_runtime.gd` dispatch entry. `get_godot_status` and `get_guide` are special-cased in `index.ts` (not in `allTools`), which is why the Node server's advertised tool count is `allTools.length + 2`.

### GDScript handler return convention

Handlers return a `Dictionary`. `ok: true` means success; on failure include an `error` string. `plugin.gd` ships the *entire* dict back (minus `ok`) so structured failure details (`open_in_editor`, `where`, `clamped`, `requested_ms`, …) survive the round-trip — the TS side spreads these into the agent-visible response in `index.ts`.

### Visualizer

`map_project` returns a project graph that `serveVisualization()` (in `src/visualizer-server.ts`) serves as a single self-contained HTML page at `localhost:6510`. The page is **built, not served from source**: `scripts/build-visualizer.js` uses esbuild to bundle `src/visualizer/*.js` + `visualizer.css` + `template.html` into `dist/visualizer.html`. After editing anything under `src/visualizer/`, run `npm run build:visualizer` (plain `npm run watch` will not pick it up).

### Guides / resources

`src/resources.ts` defines short markdown `GUIDES` exposed two ways: as standard MCP resources (`resources/list` + `resources/read`) and as a `get_guide` tool — the tool exists because some clients (Claude Desktop, Cursor chat) don't support MCP resources.

## Gotchas

- **Killing processes on port 6505:** always filter with `lsof -ti :6505 -sTCP:LISTEN`. Godot connects to 6505 as a *client*, so a plain `lsof -ti :6505` returns Godot's PID too and `kill` would crash the editor. See `killProcessOnPort()` in `index.ts`.
- **Version must stay in sync across four spots:** `mcp-server/package.json`, `mcp-server/server.json` (two version fields), and `addons/godot_mcp/plugin.cfg`. The server reads its version from `package.json` at runtime; a mismatch with a running primary triggers the stale-replacement path.
- **Tunables via env:** `GODOT_MCP_PORT` (6505), `GODOT_MCP_HTTP_PORT` (6506), `GODOT_MCP_TIMEOUT_MS` (30000), `GODOT_MCP_IDLE_TIMEOUT_MS` (30000). The `--no-force` flag stops the server from killing existing processes on its ports.
- **`dist/` is gitignored** and rebuilt by `prepare`. The published npm package ships `dist` + `scripts`; clients can also point directly at `mcp-server/dist/index.js` instead of `npx godot-mcp-server`. The published tarball must be self-contained (it does **not** ship the repo-root `schemas/`), which is why the build *generates* `src/tools/generated-tools.ts` into `dist` rather than reading `schemas/tools.json` at runtime. **Before `npm publish`, do a clean `rm -rf mcp-server/dist && npm run build`** so the tarball doesn't carry stale pre-refactor `*-tools.js` from an old build.
- **Schema edits go in `schemas/tools.json` only.** Don't hand-edit `mcp-server/src/tools/generated-tools.ts` (gitignored, regenerated each build) or expect per-category `*-tools.ts` to exist. After editing the JSON, both the Node server (`npm run build`) and `gdcli` (`go build`) pick it up.
- **gdcli on ports 6505/6506:** the Go module is at the **repo root** (`go.mod`, module `github.com/tomyud1/godot-mcp`) because `go:embed` can't reach a parent dir — the embed shim (`schemas/embed.go`) must sit beside `schemas/tools.json`. `gdcli` reads the same `GODOT_MCP_*` env tunables as the Node server.
