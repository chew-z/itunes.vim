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

**Shared Library Package (`itunes/itunes.go`)**

- Core functions for iTunes/Apple Music integration
- `SearchiTunesPlaylists()`: Executes JXA script to search Apple Music library
- `PlayPlaylistTrack()`: Executes JXA script to control playback
- Uses `osascript` with 10-second timeouts for all operations
- Shared by both CLI and MCP server implementations

**Main CLI Application (`itunes.go`)**

- Command-line interface with two main commands: `search` and `play`
- Uses shared library functions from `itunes` package
- Outputs search results to `itunes_search_results.json`

**MCP Server (`mcp-server/main.go`)**

- Model Context Protocol server implementation using mcp-go v0.34.0
- Exposes iTunes functionality as MCP tools for LLM applications
- Two tools: `search_itunes` and `play_track`
- Uses stdio transport for communication with Claude Code and other MCP clients
- Returns JSON for search results, text confirmations for playback

**JavaScript for Automation Scripts (`autoload/`)**

- `iTunes_Search2_fzf.js`: Searches Apple Music library and returns JSON array of tracks
- `iTunes_Play_Playlist_Track.js`: Controls playback of playlists and specific tracks
- Both scripts use `Application('Music')` to interface with Apple Music app

**Vim Plugin Integration**

- `plugin/itunes.vim`: Defines Vim commands (`:Tunes`, `:TunesRefresh`, `:TunesList`)
- `autoload/itunes.vim`: Core Vim functionality with fzf integration for interactive track selection
- Requires Vim 8+ for async operations, `osascript`, and `fzf`
- Caches library data in `iTunes_Library_Cache.txt` for performance

### Data Flow

1. **CLI**: Go CLI → shared library → JXA scripts → Apple Music app → JSON response → Go CLI
2. **MCP Server**: MCP client (Claude Code) → MCP server → shared library → JXA scripts → Apple Music app → response → MCP client
3. **Vim plugin**: Vim plugin → JXA scripts → Apple Music app → cached data → fzf interface
4. All components use AppleScript/JXA as the bridge to Apple Music

### Key Dependencies

- macOS with Apple Music app
- `osascript` command (built into macOS)
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

## Development Notes

- All Apple Music interactions go through AppleScript/JXA - no direct API calls
- The system relies on the Apple Music app being available and accessible
- Error handling includes specific exit codes from JXA scripts (1 = no results, 2 = script error)
- Vim plugin includes async library refresh to avoid blocking the editor
- Search results are cached both in files and memory for performance

## MCP Tools

The MCP server exposes two tools for LLM integration:

### `search_itunes`

- **Description**: Search iTunes/Apple Music library for tracks
- **Parameters**:
    - `query` (string, required): Search query for tracks in the iTunes library
- **Returns**: JSON array of matching tracks with metadata (name, artist, album, collection, ID)

### `play_track`

- **Description**: Play a playlist or specific track in iTunes/Apple Music
- **Parameters**:
    - `playlist` (string, required): Name of the playlist to play
    - `track` (string, optional): Optional specific track name to play within the playlist
- **Returns**: Text confirmation of playback status

### Usage with Claude Code

Configure the MCP server in your Claude Code MCP settings to enable iTunes integration in conversations.

## MCP Resources

The MCP server exposes three resources that provide access to cached data:

### `itunes://cache/stats`

- **Description**: Cache statistics and metadata
- **Content-Type**: application/json
- **Returns**: Current cache status including memory items, file cache items, and cache directory
- **Example**:
```json
{
  "memory_items": 3,
  "cache_dir": "/tmp/itunes-cache",
  "file_items": 5
}
```

### `itunes://cache/queries`

- **Description**: List of all cached search queries with metadata
- **Content-Type**: application/json
- **Returns**: Array of cached queries with their metadata
- **Example**:
```json
[
  {
    "query": "coldplay",
    "hash": "a1b2c3...",
    "timestamp": "2024-01-15T10:30:00Z",
    "track_count": 12,
    "source": "memory"
  }
]
```

### `itunes://cache/latest`

- **Description**: Most recent search results from cache
- **Content-Type**: application/json
- **Returns**: The latest search results (backward compatibility with CLI)
- **Example**:
```json
[
  {
    "id": "65350",
    "name": "Viva La Vida",
    "album": "Viva La Vida or Death and All His Friends",
    "collection": "Viva La Vida or Death and All His Friends",
    "artist": "Coldplay"
  }
]
```

### Using MCP Resources

MCP resources allow you to directly access cached data without using tools:

```bash
# Through Claude Code or MCP clients
claude: "Please read the resource itunes://cache/stats"
claude: "Show me the cached queries using itunes://cache/queries"
claude: "Display the latest search results from itunes://cache/latest"
```

Resources provide a more efficient way to access cached data compared to running search tools repeatedly, especially for examining cache metadata and previously executed searches.

## Caching System

The iTunes integration includes a sophisticated caching system that dramatically improves performance for repeated searches.

### Cache Architecture

**Dual-Level Caching:**
- **Memory Cache**: Fast in-memory storage using `github.com/patrickmn/go-cache`
- **File Cache**: Persistent storage in `$TMPDIR/itunes-cache/` directory

**Cache Structure:**
```
$TMPDIR/itunes-cache/
├── search_results.json                    # Latest CLI search (backward compatibility)  
└── searches/
    ├── {hash1}.json                       # Individual cached searches
    └── {hash2}.json                       # Query hash -> search results
```

### Cache Behavior

**CLI Application:**
- **First search**: Executes AppleScript, caches results, saves to `$TMPDIR`
- **Repeated search**: Returns cached results instantly with "(Using cached results)" message
- **Output location**: Moved from project directory to system temp directory

**MCP Server:**
- **First search**: Executes AppleScript, stores in memory + file cache
- **Repeated search**: Returns cached results from memory (sub-millisecond response)
- **Session persistence**: Memory cache survives for 10 minutes (configurable)
- **Cross-session**: File cache survives server restarts

### Cache Features

**Performance:**
- **Memory hits**: Near-instant response (~1ms vs ~2000ms for AppleScript)
- **Query normalization**: "Coldplay", "coldplay", " COLDPLAY " all use same cache entry
- **Automatic cleanup**: Expired entries removed automatically

**Reliability:**
- **Graceful degradation**: Cache failures don't break searches
- **TTL expiration**: Default 10-minute cache lifetime
- **Hash-based keys**: Consistent cache keys for same queries

### Cache Configuration

**Default Settings:**
- **Memory TTL**: 10 minutes
- **Cleanup interval**: 20 minutes  
- **Cache location**: `$TMPDIR/itunes-cache/`

**Environment Variables (Future):**
- `ITUNES_CACHE_TTL`: Custom cache expiration time
- `ITUNES_CACHE_DIR`: Custom cache directory location
- `ITUNES_CACHE_DISABLE`: Disable caching entirely