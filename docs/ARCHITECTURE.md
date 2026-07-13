# Architecture

Go-based iTunes/Apple Music controller for macOS with three entry points sharing
one core library. This document describes how the pieces fit together; see
[DATABASE.md](DATABASE.md) for the storage layer and
[JXA-AUTOMATION.md](JXA-AUTOMATION.md) for the Apple Music automation layer.

## Components

```
+-------------------+    +--------------------+    +---------------------+
|   CLI Tool        |    |   MCP Server       |    |   Migration Tool    |
|   (itunes.go)     |    | (mcp-server/)      |    |   (cmd/migrate/)    |
+---------+---------+    +---------+----------+    +----------+----------+
          |                        |                          |
          +------------------------+--------------------------+
                                   |
                     +-------------v--------------+
                     |    itunes (core library)   |
                     |  Client + package funcs    |
                     +------+--------------+------+
                            |              |
              +-------------v---+     +----v----------------+
              |  database pkg   |     |  JXA scripts        |
              |  SQLite + FTS5  |     |  (osascript stdin)  |
              +-----------------+     +----------+----------+
                                                 |
                                      +----------v----------+
                                      |  Apple Music app    |
                                      +---------------------+
```

| Binary | Source | Purpose |
|---|---|---|
| `bin/itunes` | `itunes.go` (repo root) | CLI: search, play, status, station CRUD |
| `bin/mcp-itunes` | `mcp-server/main.go` | MCP stdio server (14 tools + 2 resources) |
| `bin/itunes-migrate` | `cmd/migrate/main.go` | Library refresh, DB migration, validation |

Supporting packages: `itunes/` (core library), `database/` (SQLite layer),
`logging/` (zap logger setup).

A fourth entry point lives outside the Go module: the **Vim plugin**
(`vim-plugin/`) shells out to `bin/itunes` (`search --json`, `play`,
`now-playing`) and presents results through fzf — it has no direct access to
the database or JXA layer. See [VIM-PLUGIN.md](VIM-PLUGIN.md).

## The two data paths

**Search path — database only.** Queries never touch the Apple Music app. They
hit the local SQLite database through FTS5 with an in-memory result cache on
top. Typical latency <7ms cold, <5µs cached. This is why search works whether
or not Music is running.

**Control path — JXA only.** Playback, now-playing status, EQ, and audio-output
operations pipe embedded JavaScript-for-Automation scripts to `osascript` and
drive the Music app directly. The database is not consulted (except to resolve
a persistent ID before playing).

Library refresh bridges the two: Apple Music → JXA bulk extraction → JSON →
SQLite repopulation.

## The core library (`itunes/`)

### Client vs package functions

The API splits along the two data paths:

- **`itunes.Client`** owns database resources — created with
  `NewClient(ctx, logger)`, closed with `Close()`. All search, playlist, stats,
  and radio-station methods hang off it (`SearchTracks`,
  `GetTrackByPersistentID`, `GetPlaylistTracks`, `ListPlaylists`,
  `SearchStations`, `AddRadioStation`, `MigrateFromJSON`, …). One Client per
  process; both the CLI and the MCP server construct it at startup.
- **Package-level functions** wrap JXA operations and hold no state:
  `PlayPlaylistTrackWithStatus`, `GetNowPlaying`, `PlayStreamURL`,
  `RefreshLibraryCache`, `GetEQStatus`/`SetEQStatus`,
  `GetAudioOutput`/`ListAirPlayDevices`/`SetAirPlayDevice`.

There is no package-level mutable state; the Client replaced earlier globals
(`DBManager`, `SearchManager`, `SearchLimit`) in the 2026 revival.

### Context propagation

Every I/O function takes `ctx context.Context` first. Contexts flow unbroken
from the MCP request (or CLI signal context) down to `QueryContext`/`ExecContext`
on SQLite and `exec.CommandContext` on osascript, so cancellation kills the
actual work. Functions apply their own deadline on top of the caller's:
10 s for most JXA calls, 30 s for stream playback, 10 min for library refresh.

### Error model

- `ITunesError{Op, Kind, Err}` with `Unwrap()`, so `errors.As`/`errors.Is`
  work through it. Kinds include `ErrNoTracksFound`, `ErrJXAScript`,
  `ErrDatabase`.
- The database layer returns the sentinel `database.ErrTrackNotFound` instead
  of `(nil, nil)`; the Client maps it to an `ErrNoTracksFound`-kind error.
- JXA scripts signal via exit codes: 1 = no results (not an error condition),
  2 = script error. `runScript` preserves stdout even on failure so callers
  can read structured error JSON.

## MCP server

`mcp-server/main.go` builds on `mark3labs/mcp-go` (stdio transport). Tool
registration is grouped by concern (`registerSearchTools`,
`registerPlaybackTools`, `registerLibraryStateTools`, `registerPlaylistTools`,
`registerEQAudioTools`, `registerStationTools`, `registerResources`). Handlers
receive the request context and call the shared Client / package functions —
they contain no logic beyond argument parsing and JSON marshalling.

The 14 tools and their parameters are listed in the [README](../README.md#mcp-tools-14);
the authoritative definitions are the `mcp.NewTool` calls in `mcp-server/main.go`.

## CLI

`itunes.go` is a dispatch table: `commandHandlers()` maps command names to
handler closures (`handleSearch`, `handlePlay`, `handleStationAction`, …).
Adding a command means adding one entry and one handler.

## Design invariants

Worth preserving when changing code:

1. **Search never launches or requires the Music app.**
2. **Playback is ID-first.** Persistent IDs from search results are the
   reliable currency; name-based lookup is fallback only.
3. **JXA scripts are embedded** (`go:embed`) and piped via stdin — binaries
   are self-contained, no temp-file lifecycle.
4. **Bulk-fetch in JXA.** Never loop per-object property reads over large
   collections (see JXA-AUTOMATION.md — this once made refresh unusable).
5. **Zero lint findings** is the baseline: `./run_lint.sh` must stay clean.
