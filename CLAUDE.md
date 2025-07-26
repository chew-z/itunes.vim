# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@/Users/rrj/Projekty/CodeAssist/Prompts/GOLANG.md

## Build and Development Commands

```bash
# Build the CLI binary
go build -o bin/itunes itunes.go

# Build the MCP server binary
go build -o bin/mcp-itunes ./mcp-server

# Build the migration tool binary (Phase 2)
go build -o bin/itunes-migrate ./cmd/migrate

# Run the CLI program
./bin/itunes search <query>                           # Search iTunes library for tracks
./bin/itunes play <playlist> [album] [track] [trackID] # Play with context support
./bin/itunes now-playing                              # Get current playback status
./bin/itunes status                                   # Alias for now-playing

# Run the MCP server (for use with Claude Code and other LLM applications)
./bin/mcp-itunes                     # Starts MCP server via stdio transport

# Test the programs
go run itunes.go search "jazz"
go run itunes.go play "" "Album Name" "" "TRACK_ID_FROM_SEARCH"
go run itunes.go now-playing
go run ./mcp-server              # Test MCP server startup

# Test with custom database path
ITUNES_DB_PATH=/tmp/test.db ./bin/itunes-migrate
ITUNES_DB_PATH=/tmp/test.db ./bin/itunes search "jazz"

# Phase 2: SQLite Database Testing & Migration
go run database_validate.go      # Validate SQLite schema and run performance benchmarks
go test ./database -v            # Run all database tests
go test ./database -bench=.      # Run performance benchmarks

# Migration commands
./bin/itunes-migrate             # Migrate from JSON cache to SQLite
./bin/itunes-migrate -validate   # Validate existing SQLite database
./bin/itunes-migrate -from-script # Refresh library and migrate in one step
./bin/itunes-migrate -verbose    # Show detailed migration progress
```

## Project Architecture

This is a Go-based iTunes/Apple Music integration tool that bridges between command-line interface and macOS Apple Music app through AppleScript/JavaScript for Automation (JXA).

### Core Components

- **Shared Library** (`itunes/itunes.go`): Core functions for iTunes integration, including native Go search and status retrieval
- **CLI Application** (`itunes.go`): Command-line interface with search, play, and now-playing commands
- **MCP Server** (`mcp-server/main.go`): Model Context Protocol server for LLM integration
- **JXA Scripts** (`itunes/scripts/`): JavaScript automation scripts for Apple Music control (library refresh, playback, and status)
- **Database Layer** (`itunes/database/`): SQLite database with FTS5 for high-performance persistent storage and search

### Data Flow

**Search Operations (Database-backed, no scripts needed):**
- **CLI**: Go CLI → SQLite FTS5 database → Native Go search → Results
- **MCP Server**: LLM client → MCP server → SQLite FTS5 database → Native Go search → Results

**Library Refresh, Playback & Status (AppleScript/JXA bridge to Apple Music):**
- **Library Refresh**: Go → JXA script → Apple Music app → SQLite database population
- **Playback**: Go → JXA script → Apple Music app → Playback control → Current track status
- **Now Playing**: Go → JXA script → Apple Music app → Current playback status

### Key Dependencies

- macOS with Apple Music app
- Go 1.24.4+ (as specified in go.mod)
- For MCP server: `github.com/mark3labs/mcp-go v0.34.0`
- For database: `modernc.org/sqlite` (pure Go SQLite driver)
- For Vim integration: Vim 8+, fzf

### Track Data Structure

```go
type Track struct {
    ID           string   `json:"id"`
    PersistentID string   `json:"persistent_id,omitempty"` // Apple Music persistent ID (Phase 2)
    Name         string   `json:"name"`
    Album        string   `json:"album"`
    Collection   string   `json:"collection"` // Primary playlist name or album if not in a playlist
    Artist       string   `json:"artist"`
    Playlists    []string `json:"playlists"`  // All playlists containing this track
    Genre        string   `json:"genre,omitempty"`      // Phase 2: Track genre
    Rating       int      `json:"rating,omitempty"`     // Phase 2: Track rating (0-100)
    Starred      bool     `json:"starred,omitempty"`    // Phase 2: Loved/starred status
}

// Phase 2: Playlist metadata with persistent ID
type PlaylistData struct {
    ID          string `json:"id"`          // Persistent ID
    Name        string `json:"name"`
    SpecialKind string `json:"special_kind"` // "none" for user playlists
    TrackCount  int    `json:"track_count"`
    Genre       string `json:"genre,omitempty"`
}
```

## Database Architecture (Phase 2 Complete)

### Overview
The system now uses SQLite as the primary and only storage backend, with Apple Music Persistent ID support for enhanced reliability and performance:

- **SQLite Database**: Primary storage with normalized schema (artists, albums, tracks, playlists)
- **FTS5 Search**: Full-text search with <10ms query performance (achieved <7ms in testing)
- **Persistent IDs**: Apple Music's stable identifiers for reliable track identification
- **Database-First**: SQLite is the default mode, no JSON fallback
- **Enhanced JXA Scripts**: Extract persistent IDs for tracks and playlists

### Database Schema
- Artists, Genres, Albums tables with proper normalization
- Tracks table with Apple Music persistent IDs
- Playlists and playlist_tracks junction tables
- FTS5 virtual table for high-performance search
- Comprehensive indexes for query optimization

### Performance Characteristics
- **Search operations**: <7ms with real data (target <10ms achieved)
- **Cached searches**: <5µs for repeated queries
- **Insert operations**: ~800 tracks/second
- **Database initialization**: <100ms
- **Database size**: ~760 bytes per track including indexes
- **Dependencies**: Only `modernc.org/sqlite` (pure Go SQLite driver)

## MCP Tools (9 Available)

The iTunes MCP server provides 9 tools for comprehensive iTunes/Apple Music integration:

### `search_itunes`
- **Description**: Search iTunes/Apple Music library for tracks using SQLite FTS5
- **Parameters**: `query` (string, required) - Search query for tracks, artists, or albums
- **Returns**: JSON array of matching tracks with metadata
- **Performance**: <7ms average search time

### `play_track`
- **Description**: Play a track with optional playlist context for continuous playback. **IMPORTANT**: Playlist context enables seamless continuation within the playlist. Album parameter helps locate tracks but does NOT provide album playback context.
- **Parameters**:
  - `track_id` (string, optional): **RECOMMENDED** - Use the exact `id` field value from search results. Most reliable method that avoids encoding/character issues.
  - `playlist` (string, optional): **For continuous playback** - Use when playing from a user-created playlist. Use exact `collection` field value or a value from the `playlists` array. Enables playlist continuation.
  - `album` (string, optional): **For track location only** - Use the exact `album` field value from search results. Helps find tracks but does NOT provide album playback context.
  - `track` (string, optional): **FALLBACK** - Use the exact `name` field value from search results. Only use if `track_id` not available. Less reliable with complex names.
- **Returns**: **Enhanced** - JSON object with playback result and current track info after a 1-second delay

### `now_playing`
- **Description**: Get current playback status and track information from Apple Music
- **Parameters**: None
- **Returns**: JSON object with current track details, playback position, and player status ("playing", "paused", "stopped", "error")

### `refresh_library`
- **Description**: Refreshes the iTunes library database by extracting current data from Apple Music app and populating SQLite database. Takes 1-3 minutes for large libraries. Use only when library has changed significantly.
- **Parameters**: None
- **Returns**: Database population statistics and refresh status
- **Process**: JXA script extraction → SQLite database population
- **Warning**: Resource-intensive operation - only use with user approval

### `list_playlists`
- **Description**: Lists all user playlists in the iTunes/Apple Music library with metadata
- **Parameters**: None
- **Returns**: JSON array of playlists with `name`, `persistent_id`, `track_count`, `genre`, and `special_kind` fields

### `get_playlist_tracks`
- **Description**: Gets all tracks in a specific playlist
- **Parameters**:
  - `playlist` (string, required): The name or persistent ID of the playlist
  - `use_id` (boolean, optional): Set to true if providing a persistent ID instead of name. Default is false (use name)
- **Returns**: JSON array of tracks in the playlist with full metadata

### `search_advanced`
- **Description**: Advanced search with filters for genre, artist, album, rating, starred status, and streaming tracks
- **Parameters**:
  - `query` (string, required): The search query for track names, artists, or albums
  - `genre` (string, optional): Filter by genre (partial match supported)
  - `artist` (string, optional): Filter by artist name (partial match supported)
  - `album` (string, optional): Filter by album name (partial match supported)
  - `playlist` (string, optional): Filter to tracks in a specific playlist
  - `min_rating` (number, optional): Minimum rating (0-100). Only returns tracks with rating >= this value
  - `starred` (boolean, optional): If true, only return starred/loved tracks. If false, return all tracks
  - `streaming_only` (boolean, optional): If true, only return streaming tracks (e.g., radio stations). If false, return all tracks
  - `local_only` (boolean, optional): If true, only return local (non-streaming) tracks. If false, return all tracks
  - `limit` (number, optional): Maximum number of results to return. Default is 15
- **Returns**: JSON array of matching tracks with metadata including streaming indicators

### `play_stream`
- **Description**: Play streaming audio from any supported URL (itmss://, https://, http://, etc.) in Apple Music
- **Parameters**: `url` (string, required) - The streaming URL to play (supports itmss://, https://music.apple.com/, http://, https://, and other streaming formats)
- **Returns**: JSON object with playback result and current track info after streaming starts

### `search_stations`
- **Description**: Search for Apple Music radio stations by genre, name, or keywords using real-time web scraping
- **Parameters**: `query` (string, required) - Search query for stations (e.g., 'country', 'jazz', 'rock', 'classical')
- **Returns**: JSON object with matching stations including name, description, URL, genre, and keywords
- **Note**: Scrapes live data from Apple Music web interface for current station lineup

## Usage Patterns

**BEST PRACTICE: ID-based with playlist context (continuous playback):**
```json
{"track_id": "B258396D58E2ECC9", "playlist": "My Jazz Collection"}
```

**ID-based with album (track location, no continuous playback):**
```json
{"track_id": "B258396D58E2ECC9", "album": "Cul-De-Sac & Knife In The Water"}
```

**ID-only playback (single track only):**
```json
{"track_id": "B258396D58E2ECC9"}
```

**FALLBACK: Name-based with album (track location only):**
```json
{"track": "Walk On The Water", "album": "Cul-De-Sac & Knife In The Water"}
```

**FALLBACK: Name-only playbook (single track, less reliable):**
```json
{"track": "Walk On The Water"}
```

**Key Requirements:**
- **PREFERRED**: Use exact `id` field values from `search_itunes` results for maximum reliability
- **FALLBACK**: Use exact `name` field values only when ID not available
- For empty `collection` fields: ID-based lookup works universally
- Track ID lookup is immune to encoding/character issues that affect name matching

## Station Search and Play Workflow

**Complete workflow for Apple Music radio stations:**

1. **Search for stations:**
```json
{"tool": "search_stations", "arguments": {"query": "jazz"}}
```

2. **Play a station from search results:**
```json
{"tool": "play_stream", "arguments": {"url": "https://music.apple.com/us/station/jazz/ra.1000000362"}}
```

**Live station discovery:**
- Dynamically fetches current Apple Music radio stations
- Includes Apple Music 1, Apple Music Hits, Apple Music Country, Apple Music Club, Apple Music Chill
- Featured shows and artist interviews
- Genre-based stations and special programming
- Real-time availability based on current Apple Music offerings

## Database System (SQLite Only)

**SQLite with FTS5 (Single Storage Backend):**
- **Database Location**: SQLite database (configurable via `ITUNES_DB_PATH`)
- **Search Engine**: FTS5 full-text search with relevance ranking
- **Persistent IDs**: Apple Music's stable identifiers for reliable track identification
- **Database-First**: SQLite is the only storage backend (no cache files)

**Operations:**
- **Search operations**: FTS5 database search (<7ms) with advanced filtering
- **Query caching**: Database-level caching (<5µs for repeated queries)
- **Library refresh**: JXA script extraction with direct database population
- **Database size**: ~760 bytes per track including indexes
- **Search limit**: Configurable via `ITUNES_SEARCH_LIMIT` environment variable (default: 15)

### Database Refresh Process

**User Action**: Use the `refresh_library` MCP tool or run `./bin/itunes-migrate -from-script`

**Process Overview** (1-3 minutes for large libraries):
1. **JXA Script Execution**: Embedded JavaScript extracts all tracks and playlists from Apple Music app with persistent IDs
2. **JSON Cache Creation**: Script output stored in `$TMPDIR/itunes-cache/library.json`
3. **Database Population**: JSON data migrated to SQLite in atomic transaction with normalized schema
4. **FTS5 Index Rebuild**: Full-text search index updated for fast queries

**Technical Details**:
- **Complete Refresh**: Rebuilds entire database from current Apple Music state (not incremental)
- **Atomic Operation**: All changes in single transaction with rollback on failure  
- **Batch Processing**: Tracks processed in chunks of 100 for memory efficiency
- **Persistent ID Integration**: Apple Music's stable identifiers ensure reliable track identification
- **Streaming Track Support**: Detects and handles Internet audio streams with appropriate metadata

**Environment Variables:**
- `ITUNES_DB_PATH`: Override the primary database path (default: `~/Music/iTunes/itunes_library.db`)
- `ITUNES_BACKUP_DB_PATH`: Override the backup database path (default: `~/Music/iTunes/itunes_library_backup.db`)
- `ITUNES_SEARCH_LIMIT`: Set the maximum number of search results (default: 15)

## MCP Resources

- `itunes://database/stats` - Database statistics and metadata (track count, playlist count, database size, etc.)
- `itunes://database/playlists` - List of all playlists in the iTunes library with metadata

## Streaming Track Support

The system provides comprehensive support for streaming tracks (Internet audio streams like SomaFM radio stations) with different behavior from local tracks.

### Detection and Identification

**Streaming tracks are identified by:**
- `kind`: `"Internet audio stream"`
- `is_streaming`: `true`
- `stream_url`: The actual stream URL (e.g., `"http://ice6.somafm.com/insound-128-aac"`)
- `size`: `null` (no local file)
- `duration`: `null` (continuous stream)

### Different Response Structures

**For streaming tracks, `now_playing` returns:**
```json
{
  "status": "streaming",
  "stream": {
    "id": "CD48A79AC1F96E4C",
    "name": "SomaFM: The In-Sound (Special)",
    "stream_url": "http://ice6.somafm.com/insound-128-aac",
    "kind": "Internet audio stream",
    "elapsed": "2:07",
    "elapsed_seconds": 127
  },
  "display": "SomaFM: The In-Sound (Special)"
}
```

**For local tracks, `now_playing` returns:**
```json
{
  "status": "playing",
  "track": {
    "id": "4F590B5F6DF1384A",
    "name": "Humming In The Night",
    "artist": "Akira Kosemura",
    "album": "Stellar (EP) - EP",
    "position": "0:00",
    "duration": "5:08",
    "position_seconds": 0,
    "duration_seconds": 308
  },
  "display": "Humming In The Night – Akira Kosemura"
}
```

### Key Differences

- **Status field**: Streaming tracks use `"streaming"` or `"streaming_paused"` instead of `"playing"`/`"paused"`
- **Response structure**: Streaming tracks have a `stream` object instead of `track` object
- **Time fields**: Streaming tracks use `elapsed`/`elapsed_seconds` instead of `position`/`duration` fields
- **Position tracking**: Streaming tracks show elapsed time since stream started, but no total duration
- **Search results**: Include `is_streaming`, `kind`, and `stream_url` fields for streaming tracks
- **Filtering**: Use `streaming_only` or `local_only` parameters in `search_advanced` tool

### Usage with Streaming Tracks

**Search for streaming tracks only:**
```json
{"query": "soma", "streaming_only": true}
```

**Play streaming track:**
```json
{"track_id": "CD48A79AC1F96E4C"}
```

All playback operations work identically for streaming and local tracks using persistent IDs.

## Recent Critical Updates

### 1. ID-Based Playback Implementation (2025-01-21)
**Major Reliability Improvement**: Replaced fragile name-based track matching with persistent ID lookup.

**Problem**: Intermittent playback failures with complex track names like "SomaFM: Sonic Universe (#1): Transcending..." due to encoding/shell parsing issues in the automation chain.

**Solution**: Implemented ID-based track lookup using Apple Music's persistent IDs with name-based fallback.

**Reliability Impact:**
- **Before**: Fragile string matching through Go → osascript → shell → JavaScript chain (frequent failures)
- **After**: Direct persistent ID lookup + structured error messages (consistent success)
- **Result**: Eliminated intermittent playback failures, especially with complex track names

**Files Modified:**
- `itunes/scripts/iTunes_Play_Playlist_Track.js` - Added ID-based lookup as primary method, structured error responses
- `itunes/scripts/iTunes_Refresh_Library.js` - Changed from `track.id()` to `track.persistentID()` for reliable IDs
- `itunes/itunes.go` - Updated `PlayPlaylistTrack()` to accept trackID parameter, parse structured responses
- `mcp-server/main.go` - Added `track_id` parameter to MCP tool, updated descriptions

### 2. Phase 2: Enhanced JXA for Persistent IDs (2025-01-22)
**Major Enhancement**: Full persistent ID extraction for tracks and playlists with enhanced metadata.

**Achievement**: Successfully enhanced the refresh system to extract Apple Music persistent IDs and populate SQLite database directly.

**Key Improvements:**
- **Track Enhancement**: Added persistent ID, genre, rating, and starred status to track data
- **Playlist Extraction**: Complete playlist metadata with persistent IDs and special kinds
- **Structured Database Schema**: Normalized tables for artists, genres, albums, tracks, and playlists
- **100% Success Rate**: All 9,393 tracks in test library have persistent IDs extracted and migrated

**Files Modified:**
- `itunes/scripts/iTunes_Refresh_Library.js` - Enhanced to extract persistent IDs and playlist data
- `itunes/itunes.go` - Added `PlaylistData`, `RefreshStats`, enhanced `Track` struct
- `database/` - Complete SQLite schema with FTS5 search and migration tools

### 3. Database-First Search Implementation (2025-07-22)
**Major Architecture Change**: Replaced JavaScript-based search with SQLite FTS5 database backend.

**Problem**: JavaScript search scripts added unnecessary overhead and JSON file dependencies.

**Solution**: Implemented direct SQLite FTS5 search with advanced filtering capabilities.

**Performance Impact:**
- **Before**: Go → osascript → JavaScript → JSON parsing → search → JSON response → Go parsing (~50-100ms)
- **After**: Go → SQLite FTS5 query → results (<7ms)
- **Result**: ~15x faster search operations with advanced filtering support

**Files Modified:**
- `itunes/itunes.go` - Added `SearchTracks()` and `SearchTracksAdvanced()` functions with database backend
- `database/search.go` - Complete FTS5 search implementation with caching
- `mcp-server/main.go` - Updated all search handlers to use database backend
- **REMOVED**: All cache-related files (`itunes/cache.go`) and JavaScript search scripts

### 5. Script Consolidation (2025-01-21)
**Problem**: Duplicate JavaScript files in `autoload/` and `itunes/scripts/` directories.

**Solution**:
- Removed duplicates from `autoload/`
- Created symbolic links from `autoload/` → `itunes/scripts/` for remaining JXA scripts
- Maintained single source of truth in `itunes/scripts/`

**Files Affected:**
- Converted to symlinks: `iTunes_Play_Playlist_Track.js`, `iTunes_Refresh_Library.js`
- Preserved unique files: `Play.js`, `Search.js`, `Search2.js` (legacy Vim integration)

### 4. System Reliability Improvements
**Key enhancements for production stability:**

- **Empty Collection Support**: Enhanced playback to handle tracks without collection metadata
- **Universal Playability**: Optimized track lookup ensures all search results are playable
- **Script Consolidation**: Unified JXA scripts in `itunes/scripts/` with symlinks from `autoload/`
- **Error Handling**: Comprehensive error messages for database and JXA operation failures

**Result**: Reliable system with 100% track playability and graceful error handling.

### 6. Streaming Track Support Implementation (2025-01-23)
**Major Feature Addition**: Comprehensive support for streaming tracks (Internet audio streams) with differentiated behavior.

**Problem**: Streaming tracks (like SomaFM radio stations) were not properly identified and behaved identically to local tracks, confusing users with meaningless duration/position information.

**Solution**: Implemented streaming track detection with completely separate response structures.

**Key Features:**
- **Streaming Detection**: Identifies tracks by `kind: "Internet audio stream"`
- **Separate Response Structures**:
  - Streaming tracks: `status: "streaming"`, `stream` object with `elapsed`/`elapsed_seconds`
  - Local tracks: `status: "playing"`, `track` object with `position`/`duration`
- **Different Status Values**: `"streaming"` and `"streaming_paused"` for streaming tracks
- **Clean Messages**: "Started streaming: SomaFM: Lush" instead of appending "[STREAMING]"
- **Advanced Filtering**: `streaming_only` and `local_only` parameters in `search_advanced` tool
- **Stream URL Extraction**: Captures actual stream URLs for streaming tracks

**Technical Implementation:**
- **Database Schema**: Added `is_streaming`, `track_kind`, and `stream_url` columns to tracks table
- **JavaScript Detection**: Enhanced JXA scripts to detect streaming properties via `track.kind()` and `track.address()`
- **Appropriate Responses**: Streaming tracks show elapsed time but no total duration
- **Database Migration**: Schema version 2 with backward compatibility

**Files Modified:**
- `database/schema.go` - Added migration for streaming fields
- `database/database.go` - Updated Track struct and search functions with streaming support
- `database/migrate.go` - Updated JSON parsing and database insertion for streaming fields
- `itunes/scripts/iTunes_Refresh_Library.js` - Added streaming detection and metadata extraction
- `itunes/scripts/iTunes_Now_Playing.js` - Different response structures for streaming vs local tracks
- `itunes/itunes.go` - Updated Track struct and conversion functions
- `mcp-server/main.go` - Added streaming filters to search_advanced tool

**Result**: Clear differentiation between streaming and local tracks with appropriate user experience for each type.

## Development Notes

### Current Architecture (Phase 2 Complete - Database-First)
- **Search operations**: SQLite FTS5 database queries (<7ms performance)
- **Library refresh**: JXA script extraction directly to SQLite database population
- **Playback control**: AppleScript/JXA bridge to Apple Music app with persistent ID lookup
- **No direct Apple Music API**: All interactions via JXA automation scripts
- **Error handling**: Database errors and JXA exit codes (1 = no results, 2 = script error)
- **Search results**: Configurable limit via `ITUNES_SEARCH_LIMIT` (default: 15)
- **Storage**: SQLite only - cache system completely removed

### Key Functions
- `SearchTracks(query string) ([]Track, error)` - SQLite FTS5 database search (<7ms)
- `SearchTracksAdvanced(query string, filters *SearchFilters) ([]Track, error)` - Advanced search with filters
- `RefreshLibraryCache() error` - JXA script extraction with direct database population
- `PlayPlaylistTrackWithStatus(playlist, album, track, trackID string) (*PlayResult, error)` - Enhanced playback with status
- `GetTrackByPersistentID(id string) (*Track, error)` - Direct database lookup by persistent ID
- `ListPlaylists() ([]Playlist, error)` - List all playlists with metadata
- `GetPlaylistTracks(playlist string, useID bool) ([]Track, error)` - Get tracks in specific playlist

### Script Usage (Post-Consolidation & ID Enhancement)
- `itunes/scripts/iTunes_Refresh_Library.js` - Library data extraction via JXA using `track.persistentID()`
- `itunes/scripts/iTunes_Play_Playlist_Track.js` - ID-based playback control via JXA with structured error responses
- `autoload/` - Symlinks to `itunes/scripts/` + legacy Vim-specific files

### Recent Architecture Improvements

#### Phase 2 Complete: SQLite Database Integration (2025-07-22)
**Major System Overhaul**: Complete transition from JSON cache to SQLite database backend.

**Key Achievements:**
- **Database-First Architecture**: SQLite is now the only storage mechanism (cache system completely removed)
- **FTS5 Search Performance**: <7ms search times achieved (target was <10ms)
- **7 Advanced MCP Tools**: Complete toolset with `search_advanced`, `list_playlists`, `get_playlist_tracks`
- **Persistent ID Integration**: Full Apple Music persistent ID support throughout system
- **Migration Complete**: Successful migration of 9,000+ track libraries to SQLite
- **Code Cleanup**: Removed `itunes/cache.go` and all cache-related dependencies (go-cache)
- **Enhanced Reliability**: Eliminated JSON cache inconsistencies and performance issues

**Commit References:**
- [b605351](../../commit/b6053513678a7a3aa3d12ab72f1b31a764dcea4a) - Documentation corrections for album vs playlist playback context
- [4be5a56](../../commit/4be5a567780b163b15581728e45ce73dfcfd8da2) - Final Phase 2 completion with dead code cleanup
- [b9b52d0](../../commit/b9b52d03f21b7a618cfdc47230ef2f9a6493cf92) - MCP Server Database Integration with Advanced Tools
- [59f587d](../../commit/59f587d2cd7978cbc01a61ca2b01e4aebbf6f2ea) - Database-backed search as default implementation

#### ID-Based Playback Implementation (2025-01-21)
**Major Reliability Improvement**: Replaced fragile name-based track matching with persistent ID lookup.

**Solution**: Implemented ID-based track lookup using Apple Music's persistent IDs with name-based fallback for maximum reliability in complex track name scenarios.
