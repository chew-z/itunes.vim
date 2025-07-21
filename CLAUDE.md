# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@/Users/rrj/Projekty/CodeAssist/Prompts/GOLANG.md

## Build and Development Commands

```bash
# Build the CLI binary
go build -o bin/itunes itunes.go

# Build the MCP server binary
go build -o bin/mcp-itunes ./mcp-server

# Run the CLI program
./bin/itunes search <query>          # Search iTunes library for tracks
./bin/itunes play <playlist> [track] # Play playlist or specific track

# Run the MCP server (for use with Claude Code and other LLM applications)
./bin/mcp-itunes                     # Starts MCP server via stdio transport

# Test the programs
go run itunes.go search "jazz"
go run itunes.go play "My Playlist"
go run ./mcp-server              # Test MCP server startup
```

## Project Architecture

This is a Go-based iTunes/Apple Music integration tool that bridges between command-line interface and macOS Apple Music app through AppleScript/JavaScript for Automation (JXA).

### Core Components

- **Shared Library** (`itunes/itunes.go`): Core functions for iTunes integration, including native Go search
- **CLI Application** (`itunes.go`): Command-line interface with search and play commands  
- **MCP Server** (`mcp-server/main.go`): Model Context Protocol server for LLM integration
- **JXA Scripts** (`itunes/scripts/`): JavaScript automation scripts for Apple Music control (library refresh and playback only)
- **Cache System** (`itunes/cache.go`): Dual-level (memory + file) caching system
- **Vim Plugin** (deprecated): Legacy Vim integration - scripts preserved in `autoload/` as symlinks

### Data Flow

**Search Operations (Native Go, no scripts needed):**
- **CLI**: Go CLI → Direct JSON cache read → Native Go search → Results
- **MCP Server**: LLM client → MCP server → Direct JSON cache read → Native Go search → Results

**Library Refresh & Playback (AppleScript/JXA bridge to Apple Music):**
- **Library Refresh**: Go → JXA script → Apple Music app → JSON cache file
- **Playback**: Go → JXA script → Apple Music app → Playback control

### Key Dependencies

- macOS with Apple Music app
- Go 1.24.4+ (as specified in go.mod)
- For MCP server: `github.com/mark3labs/mcp-go v0.34.0`
- For Vim integration: Vim 8+, fzf

### Track Data Structure

```go
type Track struct {
    ID         string `json:"id"`
    Name       string `json:"name"`
    Album      string `json:"album"`
    Collection string `json:"collection"` // Playlist name
    Artist     string `json:"artist"`
}
```

## MCP Tools

### `search_itunes`
- **Description**: Search iTunes/Apple Music library for tracks
- **Parameters**: `query` (string, required) - Search query for tracks
- **Returns**: JSON array of matching tracks with metadata

### `play_track`
- **Description**: Play a track or album using reliable ID-based lookup. RECOMMENDED: Use `track_id` for best reliability.
- **Parameters**:
  - `track_id` (string, optional): **RECOMMENDED** - Use the exact `id` field value from search results. Most reliable method that avoids encoding/character issues with complex track names.
  - `playlist` (string, optional): Collection name from search results. Use exact `collection` field value.
  - `track` (string, optional): **FALLBACK** - Use the exact `name` field value from search results. Only use if `track_id` not available. Less reliable with complex names.
- **Returns**: Text confirmation of playback status with method used (ID vs name)

### `refresh_library`
- **Description**: Refresh iTunes library cache (1-3 minutes for large libraries)
- **Parameters**: None
- **Warning**: Resource-intensive operation - only use with user approval

## Usage Patterns

**RECOMMENDED: ID-based playback (most reliable):**
```json
{"track_id": "B258396D58E2ECC9"}
```

**ID-based with playlist context:**
```json
{"playlist": "City Lights - Single", "track_id": "A1B2C3D4E5F6789A"}
```

**FALLBACK: Name-based playback (less reliable):**
```json
{"track": "SomaFM: Lush (#1): Sensuous and mellow female vocals..."}
```

**Key Requirements:**
- **PREFERRED**: Use exact `id` field values from `search_itunes` results for maximum reliability
- **FALLBACK**: Use exact `name` field values only when ID not available
- For empty `collection` fields: ID-based lookup works universally
- Track ID lookup is immune to encoding/character issues that affect name matching

## Caching System

**Dual-Level Caching:**
- **Memory Cache**: Fast in-memory storage (10-minute TTL)
- **File Cache**: Persistent storage in `$TMPDIR/itunes-cache/`

**Performance:**
- **Search operations**: Native Go JSON parsing and search (~1-5ms) - **NO AppleScript overhead**
- **Cached search**: Returns instantly from memory cache (~1ms)
- **Library refresh**: JXA script execution (~2-3 minutes for full library scan)
- **CLI**: Saves results to `$TMPDIR/itunes-cache/search_results.json`
- **MCP**: Stores in memory + file cache for cross-session persistence

## MCP Resources

- `itunes://cache/stats` - Cache statistics and metadata  
- `itunes://cache/queries` - List of all cached search queries
- `itunes://cache/latest` - Most recent search results

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

### 2. Native Go Search Implementation (2025-01-21)
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

### 2. Script Consolidation (2025-01-21)
**Problem**: Duplicate JavaScript files in `autoload/` and `itunes/scripts/` directories.

**Solution**: 
- Removed duplicates from `autoload/`
- Created symbolic links from `autoload/` → `itunes/scripts/` for remaining JXA scripts
- Maintained single source of truth in `itunes/scripts/`

**Files Affected:**
- Converted to symlinks: `iTunes_Play_Playlist_Track.js`, `iTunes_Refresh_Library.js`
- Preserved unique files: `Play.js`, `Search.js`, `Search2.js` (legacy Vim integration)

### 3. Empty Collection Field Support (2025-01-21)
**Problem**: SomaFM tracks had empty `collection` fields, causing playlist lookup failures.
**Solution**: Made playlist parameter optional with direct track playback fallback.

### 4. Performance Optimization (2025-01-21)  
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