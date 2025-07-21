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

- **Description**: Play a track or album in iTunes/Apple Music
- **Parameters**:
    - `playlist` (string, required): Collection name from search results (album, playlist, or compilation). Use the `collection` field value from `search_itunes` results.
    - `track` (string, optional): Optional specific track name to play within the collection. If omitted, plays the entire collection.
- **Returns**: Text confirmation of playback status
- **Usage Example**: After searching for "City Lights", use the `collection` value "City Lights - Single" as the playlist parameter

### Usage with Claude Code

Configure the MCP server in your Claude Code MCP settings to enable iTunes integration in conversations.

#### Typical Workflow Example

1. **Search for music**:
   ```
   User: "Search for City Lights by Roberto Bocchetti"
   Claude: Uses search_itunes with query "City Lights Roberto Bocchetti"
   Result: [{"name": "City Lights", "artist": "Roberto Bocchetti", "collection": "City Lights - Single", ...}]
   ```

2. **Play the track**:
   ```
   User: "Play that track"
   Claude: Uses play_track with playlist="City Lights - Single" and track="City Lights"
   Result: "Started playing track 'City Lights' from playlist 'City Lights - Single'"
   ```

**Key Point**: Always use the `collection` field value from search results as the `playlist` parameter in `play_track`.

## Recent Critical Fix: Playlist Context Playback (2025-01-21)

### Problem Identified
The MCP server was only playing individual tracks instead of playing tracks within their playlist context, causing playback to stop after each track rather than continuing with the playlist. This differed from the original VIM implementation which maintained playlist context for continuous playback.

### Root Cause Analysis  
1. **Search Implementation**: Scripts only searched `music.libraryPlaylists[0]` (main library)
2. **Data Structure**: `collection` field contained album names instead of actual playlist names
3. **Playback Logic**: Played individual tracks without setting playlist context

### Solution Implemented

#### 1. Updated Search Logic (`autoload/iTunes_Search2_fzf.js` & `itunes/scripts/iTunes_Search2_fzf.js`)
- **Before**: `music.search(music.libraryPlaylists[0], { for: searchQuery })`
- **After**: Iterates through **all playlists** using `music.playlists()`
- **Key Change**: `collection` field now contains actual playlist names instead of album names
- **Behavior**: Tracks can appear multiple times if they exist in multiple playlists (beneficial feature)

```javascript
// New implementation searches all playlists
let playlists = music.playlists();
for (let playlist of playlists) {
    let playlistName = playlist.name();
    // ... search within each playlist and set collection: playlistName
}
```

#### 2. Updated Playback Logic (`autoload/iTunes_Play_Playlist_Track.js` & `itunes/scripts/iTunes_Play_Playlist_Track.js`)
- **Before**: `music.search()` + `foundTracks[0].play()` (single track only)
- **After**: Playlist-based playback with context preservation

```javascript
// New implementation: find playlist → set context → play track
let playlist = /* find by name */;
playlist.reveal();      // Set visual context  
playlist.play();        // Set playback context
foundTrack.play();      // Play specific track within playlist
```

#### 3. File Updates Made
- ✅ `autoload/iTunes_Search2_fzf.js` - Updated for CLI usage
- ✅ `autoload/iTunes_Play_Playlist_Track.js` - Updated for CLI usage  
- ✅ `itunes/scripts/iTunes_Search2_fzf.js` - Updated for MCP server (embedded)
- ✅ `itunes/scripts/iTunes_Play_Playlist_Track.js` - Updated for MCP server (embedded)
- ✅ Go embed directives automatically include updated scripts on rebuild

#### 4. Verification Results (Gemini CLI Analysis)
- ✅ **Changes Correctly Implemented**: All modifications properly address the core issue
- ✅ **Script Integration**: Search output (`collection`) → Play input (`playlist`) workflow confirmed  
- ✅ **MCP Server Integration**: Tool descriptions updated, handlers work correctly
- ✅ **Problem Resolution**: Before (single track) → After (playlist context + continuous playback)

### Performance Considerations
- **Trade-off**: New search iterates through all playlists (potentially slower for large libraries)
- **Mitigation**: Existing caching system (memory + file cache) maintains performance for repeated searches
- **Benefit**: Users can now choose playlist context for tracks that appear in multiple playlists

### Impact
- **MCP Server**: Now provides same continuous playback experience as original VIM implementation
- **CLI**: Also benefits from playlist context preservation  
- **User Experience**: Selecting a track plays it within playlist context and continues with next tracks

### Testing Status
- **Build Status**: ✅ Both `bin/mcp-itunes` and `bin/itunes` rebuild successfully
- **Script Compatibility**: ✅ Both autoload/ and embedded versions synchronized
- **Integration**: ✅ Go embed system correctly includes updated JavaScript

**Key Point**: Always use the `collection` field value from search results as the `playlist` parameter in `play_track`.

## Critical JavaScript/JXA Script Fixes (2025-01-21)

### Problem: Silent Script Failures and Performance Issues

Following analysis with Gemini AI, several critical issues were identified and resolved in the JXA (JavaScript for Automation) scripts that were causing silent failures and timeout issues.

#### Root Cause Analysis
1. **Silent Failures**: `$.exit()` calls prevented error reporting to the calling Go programs
2. **Performance Bottlenecks**: Library refresh iterated through duplicate tracks across all playlists
3. **Missing Error Context**: Scripts returned empty arrays `[]` instead of structured error information
4. **Path Inconsistencies**: Hardcoded `/tmp/` paths conflicted with proper macOS temp directories

#### Solutions Implemented

##### 1. Structured JSON Response Format
**All scripts now return consistent JSON responses:**

```javascript
// Success Response
{
  "status": "success",
  "data": [...],           // Track data or results
  "message": "Optional success message"
}

// Error Response  
{
  "status": "error",
  "message": "Detailed error description",
  "error": "ErrorName"     // Error type for debugging
}
```

**Files Updated:**
- `autoload/iTunes_Search2_fzf.js` & `itunes/scripts/iTunes_Search2_fzf.js`
- `autoload/iTunes_Play_Playlist_Track.js` & `itunes/scripts/iTunes_Play_Playlist_Track.js`
- `autoload/iTunes_Refresh_Library.js` & `itunes/scripts/iTunes_Refresh_Library.js` (new)

##### 2. Performance Optimization: Efficient Library Scanning

**Before (Inefficient):**
```javascript
// Iterated through ALL tracks in ALL playlists (massive duplication)
for (let playlist of music.playlists()) {
    for (let track of playlist.tracks()) {
        // Same track processed multiple times if in multiple playlists
    }
}
```

**After (Optimized):**
```javascript
// Process each unique track only once from main library
let libraryPlaylist = music.libraryPlaylists[0];
let libraryTracks = libraryPlaylist.tracks();
for (let i = 0; i < libraryTracks.length; i++) {
    // Each track processed exactly once
}
```

**Performance Impact:**
- **Large Libraries**: Reduced from potentially millions of operations to thousands
- **Timeout Prevention**: 3-minute timeout now sufficient for libraries with 9000+ tracks
- **Progress Reporting**: Added progress indicators for large library scans

##### 3. New MCP Tool: `refresh_library`

**Added comprehensive library refresh capability:**

```json
{
  "name": "refresh_library",
  "description": "Refresh the iTunes/Apple Music library cache. This scans all playlists and tracks to build a comprehensive searchable cache. Should be run when library changes or on first use. Takes 1-3 minutes for large libraries.",
  "parameters": {}
}
```

**Enhanced Response Format:**
```text
Library refresh completed successfully!

📊 **Cache Statistics:**
• **1,247 tracks** cached from your iTunes library
• **23 playlists** scanned  
• Cache location: /var/folders/.../T/itunes-cache/library.json

✅ You can now search for music with fast, token-efficient results (max 15 tracks per search).
```

##### 4. Proper macOS Temp Directory Usage

**Updated Path Resolution:**
```javascript
// JavaScript/JXA (NSTemporaryDirectory provides proper macOS temp path)
let tmpDir = $.NSTemporaryDirectory().js
let cacheDir = tmpDir + "itunes-cache"
let cacheFilePath = cacheDir + "/library.json"
```

```go
// Go (filepath.Join for cross-platform compatibility)
cacheDir := filepath.Join(os.TempDir(), "itunes-cache")
cacheFile := filepath.Join(cacheDir, "library.json")
```

#### Two-Phase Architecture Implementation

**Phase 1: Library Refresh (Background Operation)**
- **Tool**: `refresh_library` 
- **Duration**: 1-3 minutes for large libraries
- **Output**: Complete library cache in `$TMPDIR/itunes-cache/library.json`
- **When to Use**: First use, after library changes

**Phase 2: Fast Search (Real-time Operation)**
- **Tool**: `search_itunes`
- **Duration**: Instant (cache-based lookup)
- **Output**: Maximum 15 tracks with relevance ranking
- **Token Efficient**: Exact matches prioritized, results limited

#### Verification Results

**Testing Completed (2025-01-21):**
- ✅ Library refresh completes successfully (no more timeouts)
- ✅ Search returns exactly 15 relevant tracks with proper ranking
- ✅ Cache files created in correct temp directory (`$TMPDIR/itunes-cache/`)
- ✅ Structured error responses provide actionable debugging information
- ✅ No more silent failures - all errors reported with context
- ✅ Playlist context preserved for continuous playback

**Performance Metrics:**
- **Library Refresh**: ~2-3 minutes for 9000+ track libraries (previously timed out)
- **Search Response**: Instant (cache-based lookup)
- **Token Usage**: Maximum 15 tracks per search (LLM-friendly)
- **Cache Size**: ~1.6MB for 9000+ track library

#### Troubleshooting Guide

**Common Error Messages and Solutions:**

1. **"Cache file does not exist. Please refresh library first."**
   - **Solution**: Run `refresh_library` tool first to build cache
   - **Cause**: No library cache has been created yet

2. **"JXA script execution failed:" (empty message)**
   - **Cause**: This error should no longer occur with structured JSON responses
   - **Solution**: Update to latest version with structured error handling

3. **Permission/Automation Errors**
   - **Check**: System Settings > Privacy & Security > Automation
   - **Solution**: Allow your application to control "Music.app"

4. **Large Library Timeout**
   - **Expected**: Libraries with 5000+ tracks may take 2-3 minutes to refresh
   - **Timeout**: Increased to 180 seconds (3 minutes) for refresh operations

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