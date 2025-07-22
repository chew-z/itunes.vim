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
- **Cache System** (`itunes/cache.go`): Dual-level (memory + file) caching system
- **Database Layer** (`itunes/database/`): SQLite database with FTS5 for high-performance persistent storage and search (Phase 2)

### Data Flow

**Search Operations (Native Go, no scripts needed):**
- **CLI**: Go CLI → Direct JSON cache read → Native Go search → Results
- **MCP Server**: LLM client → MCP server → Direct JSON cache read → Native Go search → Results

**Library Refresh, Playback & Status (AppleScript/JXA bridge to Apple Music):**
- **Library Refresh**: Go → JXA script → Apple Music app → JSON cache file
- **Playback**: Go → JXA script → Apple Music app → Playback control → Current track status
- **Now Playing**: Go → JXA script → Apple Music app → Current playback status

### Key Dependencies

- macOS with Apple Music app
- Go 1.24.4+ (as specified in go.mod)
- For MCP server: `github.com/mark3labs/mcp-go v0.34.0`
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

## Phase 2: SQLite Migration (In Progress)

### Overview
Phase 2 introduces a SQLite database backend with Apple Music Persistent ID support for enhanced reliability and performance:

- **SQLite Database**: Persistent storage with normalized schema (artists, albums, tracks, playlists)
- **FTS5 Search**: Full-text search with <10ms query performance
- **Persistent IDs**: Apple Music's stable identifiers for reliable track identification (Step 2 ✅)
- **Migration Path**: Gradual migration from JSON cache to SQLite backend
- **Enhanced JXA Scripts**: Extract persistent IDs for tracks and playlists (Step 2 ✅)

### Database Schema
- Artists, Genres, Albums tables with proper normalization
- Tracks table with Apple Music persistent IDs
- Playlists and playlist_tracks junction tables
- FTS5 virtual table for high-performance search
- Comprehensive indexes for query optimization

### Performance Targets
- Search operations: <10ms with 5000+ tracks
- Insert operations: <1ms per track
- Database initialization: <100ms
- Zero external dependencies beyond SQLite driver

## MCP Tools

### `search_itunes`
- **Description**: Search iTunes/Apple Music library for tracks
- **Parameters**: `query` (string, required) - Search query for tracks
- **Returns**: JSON array of matching tracks with metadata

### `play_track`
- **Description**: Play a track with proper context for continuous playback. Use `track_id` with either `playlist` or `album` for optimal experience. The `playlist` parameter now works with actual user-created playlists.
- **Parameters**:
  - `track_id` (string, optional): **RECOMMENDED** - Use the exact `id` field value from search results. Most reliable method that avoids encoding/character issues.
  - `playlist` (string, optional): **For playlist context** - Use when playing from a user-created playlist. Use exact `collection` field value or a value from the `playlists` array.
  - `album` (string, optional): **For album context** - Use the exact `album` field value from search results. Provides album context for continuous playback.
  - `track` (string, optional): **FALLBACK** - Use the exact `name` field value from search results. Only use if `track_id` not available. Less reliable with complex names.
- **Returns**: **Enhanced** - JSON object with playback result and current track info after a 1-second delay

### `now_playing`
- **Description**: Get current playback status and track information from Apple Music
- **Parameters**: None
- **Returns**: JSON object with current track details, playback position, and player status ("playing", "paused", "stopped", "error")

### `refresh_library`
- **Description**: Refresh iTunes library cache (1-3 minutes for large libraries)
- **Parameters**: None
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
- **Description**: Advanced search with filters for genre, artist, album, rating, and starred status
- **Parameters**:
  - `query` (string, required): The search query for track names, artists, or albums
  - `genre` (string, optional): Filter by genre (partial match supported)
  - `artist` (string, optional): Filter by artist name (partial match supported)
  - `album` (string, optional): Filter by album name (partial match supported)
  - `playlist` (string, optional): Filter to tracks in a specific playlist
  - `min_rating` (number, optional): Minimum rating (0-100). Only returns tracks with rating >= this value
  - `starred` (boolean, optional): If true, only return starred/loved tracks. If false, return all tracks
  - `limit` (number, optional): Maximum number of results to return. Default is 15
- **Returns**: JSON array of matching tracks with metadata

## Usage Patterns

**BEST PRACTICE: ID-based with album context (continuous playback):**
```json
{"track_id": "B258396D58E2ECC9", "album": "Cul-De-Sac & Knife In The Water"}
```

**BEST PRACTICE: ID-based with playlist context (continuous playback):**
```json
{"track_id": "B258396D58E2ECC9", "playlist": "My Jazz Collection"}
```

**ID-only playback (single track only):**
```json
{"track_id": "B258396D58E2ECC9"}
```

**FALLBACK: Name-based with album context:**
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

## Database System

**SQLite with FTS5:**
- **Primary Storage**: SQLite database at `~/.itunes/itunes.db`
- **Search Engine**: FTS5 full-text search with relevance ranking
- **Persistent IDs**: Apple Music's stable identifiers for reliable track identification

**Performance:**
- **Search operations**: FTS5 database search (<10ms) with advanced filtering
- **Cached search**: Database-level query caching (<1ms for repeated queries)
- **Library refresh**: JXA script execution (~2-3 minutes) followed by database migration
- **Database size**: ~760 bytes per track including indexes
- **Search limit**: Configurable via `ITUNES_SEARCH_LIMIT` environment variable (default: 15)

**Environment Variables:**
- `ITUNES_DB_PATH`: Override the primary database path (default: `~/Music/iTunes/itunes_library.db`)
- `ITUNES_BACKUP_DB_PATH`: Override the backup database path (default: `~/Music/iTunes/itunes_library_backup.db`)
- `ITUNES_SEARCH_LIMIT`: Set the maximum number of search results (default: 15)

## MCP Resources

- `itunes://database/stats` - Database statistics and metadata (track count, playlist count, database size, etc.)
- `itunes://database/playlists` - List of all playlists in the iTunes library with metadata

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

### 3. Phase 2 Step 2: Enhanced JXA for Persistent IDs (2025-01-22)
**Major Enhancement**: Full persistent ID extraction for tracks and playlists with enhanced metadata.

**Achievement**: Successfully enhanced the refresh system to extract Apple Music persistent IDs while maintaining backward compatibility.

**Key Improvements:**
- **Track Enhancement**: Added persistent ID, genre, rating, and starred status to track data
- **Playlist Extraction**: Complete playlist metadata with persistent IDs and special kinds
- **Structured Response**: Separate arrays for tracks and playlists with statistics
- **Multiple Cache Files**: `library.json` (backward compatible), `library_enhanced.json`, and `playlists.json`
- **100% Success Rate**: All 9,393 tracks in test library have persistent IDs extracted

**Files Modified:**
- `itunes/scripts/iTunes_Refresh_Library.js` - Enhanced to extract persistent IDs and playlist data
- `itunes/itunes.go` - Added `PlaylistData`, `RefreshStats`, enhanced `Track` struct
- `itunes/itunes_test.go` - Created comprehensive test suite with 8 test functions
- Cache output now includes three files for different use cases

### 4. Native Go Search Implementation (2025-01-21)
**Major Architecture Change**: Replaced JavaScript-based search with native Go implementation.

**Problem**: `iTunes_Search2_fzf.js` script added unnecessary overhead - search didn't need Apple Music interaction, only JSON file reading.

**Solution**: Implemented `SearchTracksFromCache()` function in Go with direct JSON cache reading.

**Performance Impact:**
- **Before**: Go → osascript → JavaScript → JSON parsing → search → JSON response → Go parsing (~50-100ms)
- **After**: Go → direct JSON file read → native search logic → results (~1-5ms)
- **Result**: ~20x faster search operations, eliminated process startup overhead

**Files Modified:**
- `itunes/itunes.go` - Added `SearchTracksFromCache()` function, removed `SearchiTunesPlaylists()`
- `itunes.go` (CLI) - Updated to use new Go search function
- `mcp-server/main.go` - Updated search handler to use new Go function
- `itunes/scripts.go` - Removed `searchScript` embed
- **REMOVED**: `itunes/scripts/iTunes_Search2_fzf.js` and `autoload/iTunes_Search2_fzf.js`

### 5. Script Consolidation (2025-01-21)
**Problem**: Duplicate JavaScript files in `autoload/` and `itunes/scripts/` directories.

**Solution**:
- Removed duplicates from `autoload/`
- Created symbolic links from `autoload/` → `itunes/scripts/` for remaining JXA scripts
- Maintained single source of truth in `itunes/scripts/`

**Files Affected:**
- Converted to symlinks: `iTunes_Play_Playlist_Track.js`, `iTunes_Refresh_Library.js`
- Preserved unique files: `Play.js`, `Search.js`, `Search2.js` (legacy Vim integration)

### 6. Empty Collection Field Support (2025-01-21)
**Problem**: SomaFM tracks had empty `collection` fields, causing playlist lookup failures.
**Solution**: Made playlist parameter optional with direct track playback fallback.

### 7. Performance Optimization (2025-01-21)
**Problem**: Direct track search was timing out due to inefficient playlist iteration.
**Solution**: Optimized to search main library (`music.libraryPlaylists[0]`) instead of all playlists.

**Result**: Universal playability - all tracks returned by `search_itunes` can now be played.

## Development Notes

### Current Architecture (Post-Optimization)
- **Search operations**: Pure Go implementation, no external scripts
- **Library refresh & playback**: AppleScript/JXA bridge to Apple Music app
- **No direct Apple Music API**: All interactions via JXA automation scripts
- **Error handling**: Native Go errors for search, JXA exit codes for Apple Music operations (1 = no results, 2 = script error)
- **Search results**: Limited to 15 tracks per search for LLM efficiency
- **JSON responses**: Structured format from all components for consistent error handling

### Key Functions
- `SearchTracksFromCache(query string) ([]Track, error)` - Native Go search (fastest)
- `RefreshLibraryCache() error` - JXA script for library data extraction with persistent IDs
- `PlayPlaylistTrack(playlist, track, trackID string) error` - JXA script for ID-based playback control

### Script Usage (Post-Consolidation & ID Enhancement)
- `itunes/scripts/iTunes_Refresh_Library.js` - Library data extraction via JXA using `track.persistentID()`
- `itunes/scripts/iTunes_Play_Playlist_Track.js` - ID-based playback control via JXA with structured error responses
- `autoload/` - Symlinks to `itunes/scripts/` + legacy Vim-specific files

### Recent Architecture Improvements (2025-01-21)
1. **Eliminated Intermittent Playback Failures**: Root cause was complex track names failing string matching through shell automation. Solution: Persistent ID lookup.
2. **Enhanced Error Reporting**: Changed from empty/cryptic errors to structured `"ERROR: specific details"` and `"OK: success message"` responses.
3. **Maintained Backward Compatibility**: Name-based playback still works as fallback when track ID not provided.
4. **Updated Track Data Structure**: Search results now contain Apple Music persistent IDs instead of numeric database IDs for reliable cross-session track identification.
