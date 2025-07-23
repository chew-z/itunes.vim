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
- **MCP Integration**: 7 specialized tools for AI/LLM applications
- **Database-First**: Normalized SQLite schema with persistent Apple Music IDs
- **Real-Time Sync**: JXA automation bridge for live Apple Music control
- **Reliable**: Handles complex track names and encoding issues gracefully

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

# The server provides 7 tools for AI integration:
# - search_itunes
# - play_track  
# - now_playing
# - refresh_library
# - list_playlists
# - get_playlist_tracks
# - search_advanced
```

### First-Time Setup & Database Refresh

```bash
# Initialize and populate database (first run or when library changes)
./bin/itunes-migrate -from-script

# Or migrate from existing JSON cache
./bin/itunes-migrate -cache-dir ~/Music/iTunes/cache
```

#### When to Refresh Your Database

Your iTunes/Apple Music database should be refreshed when:
- **First installation** - Initial database population
- **Library changes** - After adding/removing songs or playlists  
- **Metadata updates** - After editing track information or ratings
- **Search issues** - When search results seem outdated or incomplete

#### Refresh Process (1-3 minutes)

The refresh process completely rebuilds your music database:

1. **Extracts all tracks and playlists** from Apple Music app using embedded JavaScript
2. **Creates normalized SQLite database** with persistent Apple Music IDs
3. **Builds FTS5 search index** for ultra-fast search performance (<7ms)
4. **Validates data integrity** and reports statistics

**Via MCP Server:**
```bash
# Use the refresh_library tool (requires user approval due to time cost)
```

**Via CLI:**
```bash
# Direct refresh with progress reporting
./bin/itunes-migrate -from-script -verbose

# Validate existing database
./bin/itunes-migrate -validate
```

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

### Database Management

```bash
# Refresh library from Apple Music
./bin/itunes-migrate -from-script

# Validate existing database
./bin/itunes-migrate -validate

# Verbose migration output
./bin/itunes-migrate -from-script -verbose
```

## MCP Integration

The MCP server provides 7 specialized tools for AI applications:

### Suggested System Prompt for LLM Integration

When integrating with LLM applications, use this system prompt to enable intelligent music curation:

```
Please act as DJ and curator of my Music library. You have access to the following iTunes/Apple Music tools:

**Core Tools:**
- `search_itunes` - Basic search across library for tracks, artists, albums
- `search_advanced` - Advanced search with filters (genre, artist, album, playlist, rating, starred status)
- `play_track` - Play tracks using track_id (recommended), playlist context, album, or track name
- `now_playing` - Check current playback status and track information

**Library Exploration:**
- `list_playlists` - Browse all playlists with metadata (track counts, genres)
- `get_playlist_tracks` - Get all tracks from specific playlists (by name or persistent ID)

**Usage Guidelines:**
- Always prefer `track_id` parameter in `play_track` for reliability
- Use playlist context in `play_track` for continuous playback within playlists
- Use `search_advanced` for filtered searches (by genre, rating, starred tracks, etc.)
- Explore playlists with `list_playlists` and `get_playlist_tracks` to understand the collection
- Check `now_playing` regularly to stay aware of current music state

**Restrictions:**
- NEVER use `refresh_library` without explicit user approval - this is a resource-intensive 1-3 minute operation that rebuilds the entire music database

Act as an intelligent music curator who understands the user's taste, suggests appropriate tracks/playlists, and creates seamless listening experiences.
```

### Available Tools

### 1. `search_itunes`
Search your music library with SQLite FTS5 performance.

**Parameters:**
- `query` (string, required): Search query for tracks, artists, or albums

**Returns:** JSON array of matching tracks with metadata

### 2. `play_track`
Play tracks with optional playlist context for continuous playback.

**Parameters:**
- `track_id` (string, optional): **Recommended** - Use exact `id` from search results
- `playlist` (string, optional): For continuous playback within playlist
- `album` (string, optional): For track location assistance  
- `track` (string, optional): Fallback track name matching

**Returns:** Enhanced JSON with playback result and current track info

### 3. `now_playing`
Get current playback status and track information.

**Returns:** JSON with track details, playback position, and player status

### 4. `refresh_library`
Refresh the library database from Apple Music (resource-intensive).

**Returns:** Database population statistics and refresh status

### 5. `list_playlists`  
List all user playlists with metadata.

**Returns:** JSON array of playlists with track counts and genres

### 6. `get_playlist_tracks`
Get all tracks in a specific playlist.

**Parameters:**
- `playlist` (string, required): Playlist name or persistent ID
- `use_id` (boolean, optional): Set true if using persistent ID

### 7. `search_advanced`
Advanced search with filters for genre, artist, rating, etc.

**Parameters:**
- `query` (string, required): Search query
- `genre`, `artist`, `album`, `playlist` (optional): Filter criteria
- `min_rating` (number, optional): Minimum rating (0-100)
- `starred` (boolean, optional): Filter by starred/loved tracks
- `limit` (number, optional): Results limit (default: 15)

## Performance Characteristics

### Search Performance
- **Average query time**: <7ms (target <10ms achieved)
- **Cached searches**: <5µs for repeated queries
- **Database size**: ~760 bytes per track including indexes
- **Insert performance**: ~800 tracks/second during migration

### System Requirements
- **Memory**: ~64MB cache for optimal performance
- **Storage**: ~1MB per 1,000 tracks (including FTS5 indexes)
- **Dependencies**: Pure Go SQLite driver (no CGO required)

### Benchmarks
```
Search Operations (9,000+ track library):
- Simple queries:    3-5ms
- Complex queries:   5-7ms
- FTS5 phrase search: 6-8ms
- Cache hits:        <5µs
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ITUNES_DB_PATH` | `~/Music/iTunes/itunes_library.db` | Primary database path |
| `ITUNES_BACKUP_DB_PATH` | `~/Music/iTunes/itunes_library_backup.db` | Backup database path |
| `ITUNES_SEARCH_LIMIT` | `15` | Maximum search results |

## Development

### Build Commands

```bash
# Build all binaries
go build -o bin/itunes itunes.go
go build -o bin/mcp-itunes ./mcp-server
go build -o bin/itunes-migrate ./cmd/migrate

# Run tests
go test ./...                    # All tests
go test ./database -v           # Database tests
go test ./database -bench=.     # Performance benchmarks

# Database validation
go run database_validate.go
```

### Testing

```bash
# Run CLI tests
go run itunes.go search "jazz"
go run itunes.go now-playing

# Test MCP server startup  
go run ./mcp-server

# Database performance testing
go test ./database -bench=BenchmarkSearch
```

### Code Structure

```
├── itunes.go              # Main CLI application
├── mcp-server/            # MCP server implementation
├── itunes/                # Core iTunes integration library
│   ├── itunes.go         # Main iTunes functions
│   └── scripts/          # JXA automation scripts
├── database/              # SQLite database layer
│   ├── database.go       # Core database operations
│   ├── search.go         # FTS5 search implementation
│   ├── migrate.go        # Migration utilities
│   └── schema.go         # Database schema
└── cmd/migrate/           # Migration tool
```

## Troubleshooting

### Common Issues

**Database Not Found**
```bash
# Initialize database
./bin/itunes-migrate -from-script
```

**Search Returns No Results**
```bash
# Check database status
./bin/itunes-migrate -validate

# Refresh library
./bin/itunes-migrate -from-script -verbose
```

**Playback Fails**
- Use `track_id` from search results instead of track names
- Ensure Apple Music app is running and accessible
- Check AppleScript permissions in System Preferences

**Migration Errors**
```bash
# Validate existing data
./bin/itunes-migrate -validate

# Force fresh migration
rm ~/Music/iTunes/itunes_library.db
./bin/itunes-migrate -from-script
```

### AppleScript Permissions

Grant Terminal/iTerm access to:
- **System Preferences → Security & Privacy → Privacy → Automation**
- Enable access to "Music" for your terminal application

### Performance Issues

```bash
# Check database statistics
./bin/itunes-migrate -validate

# Rebuild FTS5 index
./bin/itunes-migrate -from-script
```

## Contributing

### Development Setup

1. **Fork and clone** the repository
2. **Install Go 1.24.4+** and ensure it's in your PATH
3. **Build the project** using the commands above
4. **Run tests** to ensure everything works
5. **Follow Go conventions** for code style and structure

### Code Style

- Follow standard Go conventions
- Use `golangci-lint` for linting  
- Run `go fmt` before committing
- Add tests for new functionality
- Update documentation for API changes

### Testing

```bash
# Linting and formatting
./run_lint.sh        # If available
./run_format.sh      # If available  
./run_test.sh        # If available

# Manual testing
golangci-lint run
go fmt ./...
go test ./...
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- **Apple Music** for the rich metadata and JXA automation capabilities
- **SQLite FTS5** for exceptional full-text search performance  
- **MCP Protocol** for seamless AI/LLM integration standards
- **modernc.org/sqlite** for the pure Go SQLite implementation