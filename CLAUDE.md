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

- **Shared Library** (`itunes/itunes.go`): Core functions for iTunes integration
- **CLI Application** (`itunes.go`): Command-line interface with search and play commands  
- **MCP Server** (`mcp-server/main.go`): Model Context Protocol server for LLM integration
- **JXA Scripts** (`autoload/`, `itunes/scripts/`): JavaScript automation scripts for Apple Music control
- **Vim Plugin**: Optional Vim integration with fzf for interactive track selection

### Data Flow

All components use AppleScript/JXA as the bridge to Apple Music app:
- **CLI**: Go CLI → JXA scripts → Apple Music app → JSON response
- **MCP Server**: LLM client → MCP server → JXA scripts → Apple Music app → response
- **Vim plugin**: Vim → JXA scripts → Apple Music app → cached data → fzf interface

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
- **Description**: Play a track or album. Can play within playlist context or individual tracks directly.
- **Parameters**:
  - `playlist` (string, optional): Collection name from search results. Use exact `collection` field value.
  - `track` (string, optional): Specific track name to play. Use exact `name` field value from search results.
- **Returns**: Text confirmation of playback status

### `refresh_library`
- **Description**: Refresh iTunes library cache (1-3 minutes for large libraries)
- **Parameters**: None
- **Warning**: Resource-intensive operation - only use with user approval

## Usage Patterns

**Normal playlist context:**
```json
{"playlist": "City Lights - Single", "track": "City Lights"}
```

**Direct track playback (for tracks with empty collection fields):**
```json
{"track": "SomaFM: Lush (#1): Sensuous and mellow female vocals..."}
```

**Key Requirements:**
- Use EXACT field values from `search_itunes` results
- For empty `collection` fields: use direct track playback with just `track` parameter
- Track name matching is case-sensitive and character-perfect

## Caching System

**Dual-Level Caching:**
- **Memory Cache**: Fast in-memory storage (10-minute TTL)
- **File Cache**: Persistent storage in `$TMPDIR/itunes-cache/`

**Performance:**
- **First search**: Executes AppleScript (~2 seconds)
- **Cached search**: Returns instantly from memory (~1ms)
- **CLI**: Saves results to `$TMPDIR/itunes-cache/search_results.json`
- **MCP**: Stores in memory + file cache for cross-session persistence

## MCP Resources

- `itunes://cache/stats` - Cache statistics and metadata  
- `itunes://cache/queries` - List of all cached search queries
- `itunes://cache/latest` - Most recent search results

## Recent Critical Fixes (2025-01-21)

### 1. Empty Collection Field Support
**Problem**: SomaFM tracks had empty `collection` fields, causing playlist lookup failures.
**Solution**: Made playlist parameter optional with direct track playback fallback.

### 2. Performance Optimization  
**Problem**: Direct track search was timing out due to inefficient playlist iteration.
**Solution**: Optimized to search main library (`music.libraryPlaylists[0]`) instead of all playlists.

**Files Modified:**
- `mcp-server/main.go` - Made playlist parameter optional
- `itunes/itunes.go` - Fixed argument passing for empty playlist parameters  
- `itunes/scripts/iTunes_Play_Playlist_Track.js` - Added direct track playback fallback
- `autoload/iTunes_Play_Playlist_Track.js` - Same fallback logic for CLI usage

**Result**: Universal playability - all tracks returned by `search_itunes` can now be played.

## Development Notes

- All Apple Music interactions go through AppleScript/JXA - no direct API calls
- Error handling includes specific exit codes from JXA scripts (1 = no results, 2 = script error)
- Search results cached for performance, limited to 15 tracks per search for LLM efficiency
- Structured JSON responses from all scripts for consistent error handling