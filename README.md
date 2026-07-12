# iTunes/Apple Music CLI & MCP Server

> High-performance command-line tool and MCP server for iTunes/Apple Music integration on macOS

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Platform](https://img.shields.io/badge/Platform-macOS-blue?style=flat&logo=apple)](https://www.apple.com/macos/)
[![SQLite](https://img.shields.io/badge/Database-SQLite%20FTS5-003B57?style=flat&logo=sqlite)](https://www.sqlite.org/)
[![MCP](https://img.shields.io/badge/MCP-Server-green?style=flat)](https://github.com/mark3labs/mcp-go)

## Overview

A Go-based tool that bridges command-line interfaces and AI applications with your Apple Music library. Features ultra-fast search (<7ms via SQLite FTS5), smart playback control, and integration with Large Language Models through the Model Context Protocol (MCP).

### Key Features

- **Ultra-Fast Search**: SQLite FTS5 database with <7ms query performance
- **Smart Playback**: ID-based track lookup with playlist context support
- **Audio Control**: EQ presets and output device management (local/AirPlay)
- **MCP Integration**: 14 specialized tools for AI/LLM applications
- **Database-First**: Normalized SQLite schema keyed by persistent Apple Music IDs
- **Context-Aware**: `context.Context` flows from MCP requests down to SQLite queries and JXA processes, so cancellation propagates end to end
- **Radio Stations**: Apple Music stations with database-backed CRUD and search

## Project History

This project evolved from a legacy VIM plugin for iTunes integration. The original plugin remains untouched in the `master` branch as a reference. The current implementation is a complete rewrite focused on modern Apple Music integration, database performance, and AI/LLM compatibility through the MCP protocol.

## Architecture

```
+-------------------+    +--------------------+    +-------------------+
|   CLI Tool        |    |   MCP Server       |    |  Apple Music      |
|   (itunes)        |    |  (mcp-itunes)      |    |  Desktop App      |
+---------+---------+    +---------+----------+    +---------+---------+
          |                        |                         |
          +----------+-------------+                         |
                     |                                       |
          +----------v------------+              +----------v--------+
          |   itunes.Client       |              |   JXA Scripts     |
          |   (core library)      |<-------------+   (via osascript  |
          +----------+------------+              |    stdin)         |
                     |                           +-------------------+
          +----------v------------+
          |   SQLite Database     |
          |   FTS5 Search         |
          +-----------------------+
```

- **Search path**: queries hit the local SQLite database directly — the Apple Music app is not involved.
- **Playback path**: Go pipes embedded JXA scripts to `osascript` via stdin (no temp files) to drive the Apple Music app.
- **Library refresh**: Apple Music → JXA bulk property fetch → SQLite (~15–30 seconds even for 10k+ track libraries).

## Installation

### Prerequisites

- **macOS** (required for Apple Music integration)
- **Go 1.25+**
- **Apple Music app** (installed and configured)
- **Terminal access** with Automation permissions for AppleScript/JXA

### Build from Source

```bash
git clone <repository-url>
cd itunes

go build -o bin/itunes .                 # CLI tool
go build -o bin/mcp-itunes ./mcp-server  # MCP server
go build -o bin/itunes-migrate ./cmd/migrate  # Migration tool
```

## Quick Start

### Database Setup (first)

```bash
# First-time setup or library refresh (~15-30 seconds)
./bin/itunes-migrate -from-script

# Validate existing database
./bin/itunes-migrate -validate
```

**When to refresh**: first install, library changes, metadata updates, or search issues.

### CLI Usage

```bash
# Search your music library
./bin/itunes search "jazz"
./bin/itunes search "Miles Davis"

# Play tracks: play <playlist> [album] [track] [trackID]
./bin/itunes play "My Playlist" "" "" "B258396D58E2ECC9"   # ID with playlist context
./bin/itunes play "" "Kind of Blue" "So What"              # album + track name

# Current playback status
./bin/itunes now-playing   # or: status

# Radio stations
./bin/itunes search-stations "jazz"
./bin/itunes list-stations
./bin/itunes add-station --name "Name" --url "itmss://..." [--genre g --description d --homepage url]
./bin/itunes update-station <id> [--name/--url/--genre/--description/--homepage ...]
./bin/itunes delete-station <id>
./bin/itunes import-stations stations.json
./bin/itunes export-stations stations.json
```

### MCP Server

```bash
# Start MCP server (stdio transport)
./bin/mcp-itunes
```

Claude Desktop / Claude Code configuration:

```json
{
  "mcpServers": {
    "itunes": {
      "command": "/path/to/bin/mcp-itunes"
    }
  }
}
```

## MCP Tools (14)

**Music Library:**
- `search_itunes` — search library (<7ms FTS5 performance)
- `search_advanced` — filtered search (genre, artist, album, rating, starred, streaming)
- `play_track` — play with ID-based lookup and optional playlist context
- `now_playing` — current playback status and track info
- `refresh_library` — rebuild database from Apple Music (~15–30 s, resource-intensive)

**Playlists:**
- `list_playlists` — all playlists with metadata
- `get_playlist_tracks` — tracks in a specific playlist

**Radio & Streams:**
- `search_stations` — find Apple Music radio stations by genre/name
- `play_stream` — play streaming URLs (`itmss://`, `https://`)

**EQ & Audio Output:**
- `check_eq` — current EQ status and available presets
- `set_eq` — apply EQ presets (Rock, Jazz, Classical, …) or enable/disable
- `get_output_device` — current audio output (local/AirPlay)
- `list_output_devices` — available output devices
- `set_output_device` — switch to local output (AirPlay selection must be manual)

**MCP Resources:** `itunes://database/stats`, `itunes://database/playlists`

### Usage Pattern

Best practice is ID-based playback with playlist context for continuous play:

```json
{"track_id": "B258396D58E2ECC9", "playlist": "My Jazz Collection"}
```

Track IDs come from search results and are Apple Music persistent IDs, which avoids name-matching and encoding issues.

## Performance

- **Search**: <7ms queries, <5µs cache hits
- **Database**: ~760 bytes per track, ~800 tracks/sec migration
- **Storage**: ~1MB per 1,000 tracks including FTS5 indexes
- **Driver**: pure Go SQLite (`modernc.org/sqlite`, no CGO)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ITUNES_DB_PATH` | `~/Music/iTunes/itunes_library.db` | Primary database path |
| `ITUNES_BACKUP_DB_PATH` | `~/Music/iTunes/itunes_library_backup.db` | Backup database path |
| `ITUNES_SEARCH_LIMIT` | `15` | Maximum search results |

## Documentation

Developer docs live in [`docs/`](docs/): [architecture](docs/ARCHITECTURE.md),
[database & FTS5 internals](docs/DATABASE.md), and
[JXA automation](docs/JXA-AUTOMATION.md) (including the bulk-fetch rule —
required reading before touching any JXA script).

## Development

### Project Structure

```
├── itunes.go              # CLI application (command dispatch + handlers)
├── mcp-server/            # MCP server (14 tools, grouped registration)
├── itunes/                # Core library: Client type, playback, EQ, JXA scripts
│   └── scripts/           # Embedded JXA automation scripts
├── database/              # SQLite + FTS5: schema, search, migrations
├── docs/                  # Developer documentation
├── logging/               # zap logger setup
└── cmd/migrate/           # Database migration/validation tool
```

The core library exposes `itunes.NewClient(logger)` owning the database connection and search manager; playback and audio-control functions are package-level and take a `context.Context`.

### Build, Test, Lint

```bash
./run_test.sh        # go test -v ./...
./run_lint.sh        # golangci-lint (zero findings expected)
./run_format.sh      # gofmt -w .

# Or directly:
go test ./...
go test ./database -bench=BenchmarkSearch
```

## Troubleshooting

**Database issues:**
```bash
./bin/itunes-migrate -from-script    # Initialize/refresh
./bin/itunes-migrate -validate       # Check status
```

**Playback issues:**
- Use `track_id` from search results (most reliable)
- Ensure the Apple Music app is running
- Grant Automation permissions: System Settings → Privacy & Security → Automation

**EQ/Audio issues:**
- EQ status is unavailable while using AirPlay (macOS limitation)
- Specific AirPlay device selection must be done manually in the Music app

**Radio station playback:**
- Stations must use `itmss://` protocol URLs for playback; `https://` URLs are homepage links only

## License & Acknowledgments

**MIT License** — see LICENSE file for details.

Thanks to Apple Music, SQLite FTS5, the MCP Protocol, and `modernc.org/sqlite`.
