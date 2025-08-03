# iTunes/Apple Music CLI & MCP Server

> High-performance command-line tool and MCP server for iTunes/Apple Music integration on macOS

[![Go Version](https://img.shields.io/badge/Go-1.24.4+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Platform](https://img.shields.io/badge/Platform-macOS-blue?style=flat&logo=apple)](https://www.apple.com/macos/)
[![SQLite](https://img.shields.io/badge/Database-SQLite%20FTS5-003B57?style=flat&logo=sqlite)](https://www.sqlite.org/)
[![MCP](https://img.shields.io/badge/MCP-Server-green?style=flat)](https://github.com/mark3labs/mcp-go)

## Overview

A comprehensive Go-based tool that bridges command-line interfaces and AI applications with your Apple Music library. Features ultra-fast search capabilities (<7ms), smart playback control, and seamless integration with Large Language Models through the Model Context Protocol (MCP).

### Key Features

- **Ultra-Fast Search**: SQLite FTS5 database with <7ms query performance
- **Smart Playback**: ID-based track lookup with playlist context support
- **Audio Control**: EQ presets and output device management (local/AirPlay)
- **MCP Integration**: 14 specialized tools for AI/LLM applications
- **Database-First**: Normalized SQLite schema with persistent Apple Music IDs
- **Real-Time Sync**: JXA automation bridge for live Apple Music control
- **Radio Stations**: 25+ Apple Music stations with database search

## Project History

This project evolved from a legacy VIM plugin for iTunes integration originally developed 8 years ago. The original VIM plugin remains untouched in the `master` branch as a reference for the earlier iTunes integration approach. The current implementation represents a complete rewrite focused on modern Apple Music integration, database performance, and AI/LLM compatibility through the MCP protocol.

## Architecture

```
+-------------------+    +--------------------+    +-------------------+
|   CLI Tool        |    |   MCP Server       |    |  Apple Music      |
|   (itunes)        |    |  (mcp-itunes)      |    |  Desktop App      |
+---------+---------+    +---------+----------+    +---------+---------+
          |                        |                         |
          +----------+-------------+                         |
                     |                                       |
          +----------v------------+              +---------v---------+
          |   Go Application      |              |   JXA Scripts     |
          |   Core Logic          |<-------------+   Automation      |
          +----------+------------+              +-------------------+
                     |
          +----------v------------+
          |   SQLite Database     |
          |   FTS5 Search         |
          +-----------------------+
```

## Installation

### Prerequisites

- **macOS** (required for Apple Music integration)
- **Go 1.24.4+** 
- **Apple Music app** (installed and configured)
- **Terminal access** with appropriate permissions for AppleScript

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd itunes

# Build CLI tool
go build -o bin/itunes itunes.go

# Build MCP server
go build -o bin/mcp-itunes ./mcp-server

# Build migration tool (if needed)
go build -o bin/itunes-migrate ./cmd/migrate
```

### Binary Locations

After building, binaries are available in the `bin/` directory:
- `bin/itunes` - CLI tool
- `bin/mcp-itunes` - MCP server
- `bin/itunes-migrate` - Database migration tool

## Quick Start

### CLI Usage

```bash
# Search your music library
./bin/itunes search "jazz"
./bin/itunes search "Miles Davis"

# Play tracks with context
./bin/itunes play "My Playlist" "" "" "TRACK_ID_FROM_SEARCH"
./bin/itunes play "" "Album Name" "Track Name"

# Check current playback
./bin/itunes now-playing
./bin/itunes status  # alias for now-playing
```

### MCP Server

```bash
# Start MCP server (stdio transport)
./bin/mcp-itunes

# The server provides 14 tools including:
# - search_itunes, play_track, now_playing
# - check_eq, set_eq (EQ control)
# - get_output_device, set_output_device (audio output)
# - search_stations, play_stream (radio stations)
# - refresh_library, list_playlists, get_playlist_tracks
# - search_advanced
```

### Database Setup

```bash
# First-time setup or library refresh (1-3 minutes)
./bin/itunes-migrate -from-script

# Validate existing database
./bin/itunes-migrate -validate
```

**When to refresh**: First install, library changes, metadata updates, or search issues.

**Process**: Apple Music → JXA → SQLite with persistent IDs and FTS5 index.

## Usage Examples

### Search Operations

```bash
# Basic search
./bin/itunes search "blue note"

# Set custom search limit
ITUNES_SEARCH_LIMIT=25 ./bin/itunes search "jazz"

# Search with custom database path
ITUNES_DB_PATH=/custom/path/library.db ./bin/itunes search "classical"
```

### Playback Control

```bash
# ID-based playback (most reliable)
./bin/itunes play "" "" "" "B258396D58E2ECC9"

# Playlist context (enables continuous playback)
./bin/itunes play "Jazz Collection" "" "" "B258396D58E2ECC9"

# Album context (helps locate track)
./bin/itunes play "" "Kind of Blue" "So What"

# Name-based fallback
./bin/itunes play "" "" "Take Five"
```


## MCP Integration

The MCP server provides 14 specialized tools for AI applications:

### LLM Integration

For intelligent music curation with AI applications, see the comprehensive system prompt in [`Phase2/system_prompt.md`](Phase2/system_prompt.md) which includes:

- Complete tool descriptions and usage guidelines
- Music curation best practices  
- EQ and audio output control instructions
- Radio station discovery and playback
- Proper restrictions and safety measures

### Core Tools

**Music Library:**
- `search_itunes` - Search library (<7ms FTS5 performance)
- `search_advanced` - Advanced search with filters (genre, rating, starred)
- `play_track` - Play with ID-based lookup and playlist context
- `now_playing` - Current playback status and track info

**Audio Control (New):**
- `check_eq` - Get current EQ status and available presets
- `set_eq` - Apply EQ presets (Rock, Jazz, Classical, etc.) or enable/disable
- `get_output_device` - Check current audio output (local/AirPlay)
- `set_output_device` - Switch to local output (AirPlay selection manual)

**Radio Stations:**
- `search_stations` - Find Apple Music radio stations by genre/name
- `play_stream` - Play streaming URLs (itmss://, https://)

**Playlists & Library:**
- `list_playlists` - All playlists with metadata
- `get_playlist_tracks` - Tracks in specific playlist
- `refresh_library` - Rebuild database from Apple Music (1-3 min)

**Stream Integration:**
- `play_stream` - Play any streaming URL (Apple Music stations, web streams)

## Performance

- **Search**: <7ms queries, <5µs cache hits
- **Database**: ~760 bytes per track, ~800 tracks/sec migration
- **Memory**: ~64MB cache, pure Go SQLite (no CGO)
- **Storage**: ~1MB per 1,000 tracks including FTS5 indexes

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ITUNES_DB_PATH` | `~/Music/iTunes/itunes_library.db` | Primary database path |
| `ITUNES_BACKUP_DB_PATH` | `~/Music/iTunes/itunes_library_backup.db` | Backup database path |
| `ITUNES_SEARCH_LIMIT` | `15` | Maximum search results |

## Development

### Build & Test

```bash
# Build all binaries
go build -o bin/itunes itunes.go
go build -o bin/mcp-itunes ./mcp-server
go build -o bin/itunes-migrate ./cmd/migrate

# Test & validate
go test ./...                           # All tests
go test ./database -bench=BenchmarkSearch  # Performance
go run ./mcp-server                     # MCP server test
./bin/itunes search "jazz"              # CLI test
```

### Project Structure

```
├── itunes.go              # CLI application
├── mcp-server/            # MCP server (14 tools)
├── itunes/                # Core library + JXA scripts
├── database/              # SQLite + FTS5 search
└── cmd/migrate/           # Database migration tool
```

## Troubleshooting

**Database Issues:**
```bash
./bin/itunes-migrate -from-script    # Initialize/refresh
./bin/itunes-migrate -validate       # Check status
```

**Playback Issues:**
- Use `track_id` from search results (most reliable)
- Ensure Apple Music app is running
- Grant AppleScript permissions: System Preferences → Security & Privacy → Automation

**EQ/Audio Issues:**
- EQ unavailable during AirPlay (macOS limitation)
- AirPlay device selection must be done manually in Music app

## Contributing

1. Fork and clone the repository
2. Install Go 1.24.4+ 
3. Build and test the project
4. Follow Go conventions and run `golangci-lint`
5. Add tests for new functionality

### Testing & Quality

```bash
# Code quality
golangci-lint run    # Linting
go fmt ./...         # Formatting
go test ./...        # All tests
```

## License & Acknowledgments

**MIT License** - See LICENSE file for details.

Thanks to Apple Music, SQLite FTS5, MCP Protocol, and modernc.org/sqlite.