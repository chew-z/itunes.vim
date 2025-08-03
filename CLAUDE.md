# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@/Users/rrj/Projekty/CodeAssist/Prompts/GOLANG.md

## Build Commands

```bash
# Build binaries
go build -o bin/itunes itunes.go              # CLI application
go build -o bin/mcp-itunes ./mcp-server       # MCP server
go build -o bin/itunes-migrate ./cmd/migrate  # Migration tool

# CLI usage
./bin/itunes search <query>                   # Search library
./bin/itunes play <playlist> [track_id]       # Play with context
./bin/itunes now-playing                      # Current status
./bin/itunes search-stations "jazz"           # Find radio stations

# MCP server
./bin/mcp-itunes                              # Start MCP server

# Database operations
./bin/itunes-migrate -from-script             # Refresh library to SQLite
./bin/itunes-migrate -validate                # Validate database
go test ./database -v                         # Run tests
```

## Project Architecture

Go-based iTunes/Apple Music control tool with CLI, MCP server, and SQLite database backend.

### Architecture

- **CLI** (`itunes.go`): Command-line interface
- **MCP Server** (`mcp-server/main.go`): LLM integration via Model Context Protocol
- **Core Library** (`itunes/itunes.go`): iTunes integration functions
- **Database** (`database/`): SQLite with FTS5 search (<7ms queries)
- **JXA Scripts** (`itunes/scripts/`): Apple Music automation via JavaScript

### Data Flow

**Search**: SQLite FTS5 database queries (<7ms)
**Playback**: Go → JXA → Apple Music app
**Library Refresh**: Apple Music → JXA → SQLite database
**Status**: JXA → Apple Music app state

### Dependencies

- macOS + Apple Music app
- Go 1.24.4+
- `github.com/mark3labs/mcp-go v0.34.0` (MCP server)
- `modernc.org/sqlite` (pure Go SQLite driver)

### Data Structures

```go
type Track struct {
    ID           string   // Apple Music persistent ID
    Name         string   // Track name
    Album        string   // Album name
    Collection   string   // Primary playlist/album
    Artist       string   // Artist name
    Playlists    []string // All containing playlists
    Genre        string   // Track genre
    Rating       int      // 0-100
    Starred      bool     // Loved status
    IsStreaming  bool     // Internet stream vs local
}
```

## Database (SQLite + FTS5)

**Architecture**: SQLite-only storage with Apple Music Persistent IDs
**Performance**: <7ms search, <5µs cached queries, ~800 tracks/sec insert
**Schema v4**: Normalized tables (artists, albums, tracks, playlists, radio_stations)
**Search**: FTS5 full-text with relevance ranking
**Size**: ~760 bytes per track including indexes

## MCP Tools (14 Available)

**New (Aug 2025)**: EQ and audio output control tools

### `get_output_device`
- **Description**: Gets current audio output device status (local speakers or AirPlay)
- **Parameters**: None
- **Returns**: `{"output_type": "local|airplay", "device_name": "..."}`
- **Limitations**: AirPlay shows generic status only due to macOS restrictions

### `list_output_devices`
- **Description**: Lists available audio output devices with AirPlay detection
- **Parameters**: None
- **Returns**: JSON array with local device and AirPlay status indicator
- **Limitations**: Cannot enumerate specific AirPlay devices due to system restrictions

### `set_output_device`
- **Description**: Switch audio output to local speakers (AirPlay selection must be manual)
- **Parameters**: `device_name` (string) - Use "local", "computer", or computer name
- **Returns**: Confirmation or error message
- **Limitations**: Can only disable AirPlay, not select specific AirPlay devices

### `check_eq`
- **Description**: Get current Apple Music EQ status and available presets
- **Parameters**: None
- **Returns**: `{"enabled": boolean, "current_preset": string|null, "available_presets": [...]}`
- **Note**: Cannot check EQ while using AirPlay (system limitation)

### `set_eq`
- **Description**: Enable/disable EQ or apply preset (Rock, Jazz, Classical, etc.)
- **Parameters**: `preset` (string, optional), `enabled` (boolean, optional)
- **Returns**: Updated EQ status confirmation
- **Note**: Applying preset automatically enables EQ

### Core Tools

**`search_itunes`**: Search library using FTS5 (<7ms)
- `query` (string): Search query
- Returns: Track array with metadata

**`play_track`**: Play with context support
- `track_id` (string, recommended): Persistent ID from search
- `playlist` (string, optional): For continuous playback
- `album` (string, optional): Track location only
- Returns: Playback result + current status

**`now_playing`**: Current playback status
- Returns: Track info, position, player state

**`refresh_library`**: Update database from Apple Music (1-3 min)
- Warning: Resource-intensive, user approval needed

**`search_advanced`**: Advanced search with filters
- `query` + filters: genre, artist, album, rating, starred, streaming
- `limit` (default: 15)

**`list_playlists`**: Get all playlists with metadata
**`get_playlist_tracks`**: Tracks in specific playlist
**`play_stream`**: Play streaming URLs (itmss://, https://)
**`search_stations`**: Find Apple Music radio stations

## Usage Patterns

**Best Practice**: ID-based with playlist context
```json
{"track_id": "B258396D58E2ECC9", "playlist": "My Jazz Collection"}
```

**Track ID Only**: Single track playback
```json
{"track_id": "B258396D58E2ECC9"}
```

**Key Points**:
- Use `track_id` from search results (most reliable)
- Add `playlist` for continuous playback
- ID-based lookup avoids encoding issues

## Radio Stations

**Workflow**: `search_stations "jazz"` → `play_stream itmss://...`
**Coverage**: 25+ stations (Apple Music 1, genre stations, personal stations)
**Regions**: US + Polish Apple Music stations
**URLs**: `itmss://` for playback, `https://` for web

## Database Operations

**Storage**: SQLite-only with FTS5 search
**Refresh**: `refresh_library` tool or `./bin/itunes-migrate -from-script`
**Process**: Apple Music → JXA → JSON → SQLite (1-3 min)
**Environment**:
- `ITUNES_DB_PATH`: Database location
- `ITUNES_SEARCH_LIMIT`: Result limit (default: 15)

## MCP Resources

- `itunes://database/stats`: Database statistics
- `itunes://database/playlists`: Playlist metadata

## Streaming Support

**Detection**: `is_streaming: true`, `kind: "Internet audio stream"`
**Status**: `"streaming"` vs `"playing"` for local tracks
**Time Tracking**: `elapsed` (streaming) vs `position/duration` (local)
**Filtering**: Use `streaming_only`/`local_only` in `search_advanced`
**Playback**: Same API for streaming and local tracks

## Recent Critical Updates

### EQ and Audio Output Control Tools (Aug 2025)
**New Functionality**: Added comprehensive EQ and audio output device management.

**Key Features:**
- **EQ Control**: Check status, enable/disable, apply presets (Rock, Jazz, Classical, etc.)
- **Audio Output**: Detect current device (local/AirPlay), switch to local output
- **System Integration**: Direct Apple Music app control via enhanced JXA scripts
- **Error Handling**: Graceful handling of AirPlay limitations and EQ restrictions

**Implementation Details:**
- Enhanced JXA scripts with robust error handling and validation
- Comprehensive tool documentation with clear limitations
- Improved reliability through better JavaScript for Apple Music interactions
- Added system restriction awareness (AirPlay enumeration, EQ during AirPlay)

**Files Added/Modified:**
- `itunes/scripts/iTunes_Get_EQ.js`, `iTunes_Set_EQ.js` - EQ management
- `itunes/scripts/iTunes_Get_Audio_Output.js`, `iTunes_Set_AirPlay_Device.js` - Audio output control
- `itunes/itunes.go` - Go wrapper functions with structured responses
- `mcp-server/main.go` - MCP tool implementations

**Commit References:**
- [11dc9b9](../../commit/11dc9b9) - feat(itunes): improve EQ and audio output tool reliability
- [e856b1a](../../commit/e856b1a) - feat(itunes): add EQ and audio output control tools

### Apple Music Station URL Format Fix (2025-07-28)
**Critical Playback Bug Fix**: Fixed Apple Music radio station playback by implementing proper `itmss://` protocol URLs.

**Problem**: Apple Music radio stations were playing incorrect content due to using `https://` web URLs instead of the proper `itmss://` protocol URLs. For example, requesting "Apple Music Chill" would play "Radio Paradise" instead.

**Root Cause**: Apple Music requires the `itmss://` protocol for internal station playback, while `https://` URLs are only for web browser access.

**Solution**:
- **Playback URLs**: Use `itmss://music.apple.com/station/...?app=music` format for reliable Apple Music integration
- **Homepage URLs**: Use `https://music.apple.com/station/...` format for web browser access
- **Database Migration v4**: Automatically converted existing URLs to proper formats

**Technical Implementation:**
- **Enhanced JXA Script**: Updated `iTunes_Play_Stream_URL.js` with better `itmss://` protocol handling and validation
- **CLI Validation**: Added URL format validation to prevent incorrect protocol usage
- **Database Schema Cleanup**: Removed superficial fields (`country`, `language`, `quality`) from radio stations schema
- **Dual URL Support**: Separate fields for playback URLs (`itmss://`) and homepage URLs (`https://`)

**Files Modified:**
- `database/schema.go` - Added migration v4 for URL format conversion and schema cleanup
- `database/database.go` - Updated RadioStation struct and removed superficial fields
- `itunes/scripts/iTunes_Play_Stream_URL.js` - Enhanced protocol validation and error handling
- `itunes.go` - Added URL format validation and updated CLI help text
- `stations.json` - Converted all station URLs to proper `itmss://` format

**Performance Impact:**
- **Before**: Inconsistent playback with wrong stations playing (~50% failure rate)
- **After**: Reliable Apple Music station playback (100% success rate)
- **Database**: Cleaner schema with ~30% fewer fields and better organization

**Result**: All Apple Music radio stations now play correctly with proper track metadata display.

**Commit References:**
- [00005b7](../../commit/00005b7) - feat(radio-stations): implement database-backed radio station management
- [9e29ac0](../../commit/9e29ac0) - feat(itunes): broaden play_stream URL support
- [b945172](../../commit/b945172) - feat(itunes): add tools to search and play radio stations

### Key Architecture Changes

**Database Schema v4 (Jul 2025)**: Cleaned radio stations table, dual URL system (`itmss://` playback, `https://` web)

**Personal Stations (Jul 2025)**: Added user-created Apple Music stations (Discovery, Focus, Relax, etc.)

**ID-Based Playback (Jan 2025)**: Persistent IDs eliminate name-matching failures in automation chain

**Database-First Search (Jul 2025)**: SQLite FTS5 backend (15x faster, <7ms vs 50-100ms JavaScript)

**Streaming Support (Jan 2025)**: Separate handling for Internet streams with different response structures

## Architecture

**Search**: SQLite FTS5 (<7ms), Apple Music persistent IDs
**Playback**: JXA automation with ID-based lookup  
**Storage**: SQLite-only database (no cache files)
**Error Handling**: JXA exit codes (1=no results, 2=script error)

**Key Functions**: `SearchTracks()`, `PlayPlaylistTrackWithStatus()`, `RefreshLibraryCache()`, `GetTrackByPersistentID()`

