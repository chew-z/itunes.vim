#n. Apple Music Streaming Track Support Implementation Plan

*Generated: July 23, 2025*

## Research Summary

Based on comprehensive JXA script testing of the "Internet Songs" playlist, streaming tracks in Apple Music are **fully scriptable** and work seamlessly with existing infrastructure. This plan outlines enhancements to add streaming track detection and user-facing indicators.

### Key Research Findings

#### ✅ Streaming Track Detection (Reliable Indicators)
- `track.kind()` = `"Internet audio stream"` (perfect indicator)
- `track.size()` = `null` (vs local tracks have actual file sizes)
- `track.duration()` = `null` (vs local tracks have durations in seconds)
- `track.location()` = `"N/A"` (no local file path)
- `track.address` = stream URL (e.g., `http://ice6.somafm.com/insound-128-aac`)
- `track.class` = `"urlTrack"` (vs local tracks)

#### ✅ Streaming Track Scriptability (Contrary to Expectations)
**All JXA operations work perfectly with streaming tracks:**
- ✅ `track.play()` works reliably
- ✅ Persistent ID lookup works (`music.tracks.whose({persistentID: id})`)
- ✅ Playlist context playback works
- ✅ `track.reveal()` works
- ✅ Basic playback controls (pause/resume) work
- ✅ 55 properties available (same as local tracks)

#### ✅ Current Track Behavior
- Position tracking works (shows seconds elapsed)
- All metadata accessible via `music.currentTrack`
- Playback control works normally

## Implementation Plan

### Phase 1: Database Schema Enhancement

**Files to modify:** `database/schema.go`, `database/database.go`

#### Database Changes:
```sql
-- Add to tracks table
ALTER TABLE tracks ADD COLUMN is_streaming BOOLEAN DEFAULT FALSE;
ALTER TABLE tracks ADD COLUMN track_kind VARCHAR(100);
ALTER TABLE tracks ADD COLUMN stream_url VARCHAR(500);

-- Create index for streaming queries
CREATE INDEX idx_tracks_streaming ON tracks(is_streaming);
CREATE INDEX idx_tracks_kind ON tracks(track_kind);
```

#### Go Struct Updates:
```go
// Update Track struct in database/database.go
type Track struct {
    // ... existing fields ...
    IsStreaming  bool   `json:"is_streaming"`
    Kind         string `json:"track_kind"`
    StreamURL    string `json:"stream_url,omitempty"`
}
```

#### Migration Implementation:
- Create migration script for existing installations
- Handle NULL values gracefully for existing tracks
- Preserve backward compatibility

### Phase 2: Library Refresh Enhancement

**Files to modify:** `itunes/scripts/iTunes_Refresh_Library.js`, `itunes/itunes.go`

#### JXA Script Updates:
```javascript
// Extract streaming detection properties
let trackKind = track.kind.exists() ? track.kind() : '';
let isStreaming = trackKind === "Internet audio stream";
let streamURL = '';

if (isStreaming) {
    try {
        streamURL = track.address.exists() ? track.address() : '';
    } catch (e) { /* handle error */ }
}

// Add to track data
trackData.track_kind = trackKind;
trackData.is_streaming = isStreaming;
trackData.stream_url = streamURL;
```

#### Go Struct Updates:
```go
// Update Track struct in itunes/itunes.go
type Track struct {
    // ... existing fields ...
    IsStreaming bool   `json:"is_streaming"`
    Kind        string `json:"kind,omitempty"`
    StreamURL   string `json:"stream_url,omitempty"`
}
```

### Phase 3: MCP Tools Enhancement

**Files to modify:** `mcp-server/main.go`

#### Search Tools Updates:

**Enhanced `search_itunes` responses:**
```json
{
    "id": "CD48A79AC1F96E4C",
    "name": "SomaFM: The In-Sound (Special)",
    "artist": "",
    "album": "",
    "is_streaming": true,
    "kind": "Internet audio stream",
    "stream_url": "http://ice6.somafm.com/insound-128-aac"
}
```

**New `search_advanced` parameters:**
```go
mcp.WithBoolean("streaming_only",
    mcp.Description("If true, only return streaming tracks. If false, return all tracks."),
),
mcp.WithBoolean("local_only", 
    mcp.Description("If true, only return local (non-streaming) tracks. If false, return all tracks."),
),
```

#### Status Tools Updates:

**Enhanced `now_playing` responses:**
```json
{
    "status": "playing",
    "track": {
        "id": "CD48A79AC1F96E4C",
        "name": "SomaFM: The In-Sound (Special)",
        "is_streaming": true,
        "kind": "Internet audio stream",
        "stream_url": "http://ice6.somafm.com/insound-128-aac"
    }
}
```

**Enhanced `play_track` responses:**
```json
{
    "success": true,
    "message": "Started playing streaming track: SomaFM: The In-Sound (Special)",
    "now_playing": {
        "track": {
            "is_streaming": true,
            "kind": "Internet audio stream"
        }
    }
}
```

### Phase 4: User Experience Improvements

**Files to modify:** `CLAUDE.md` (documentation)

#### Documentation Updates:
- Document streaming track behavior in MCP tools section
- Add streaming track usage examples
- Update tool descriptions to clarify streaming support
- Add troubleshooting guidance for streaming-specific scenarios

#### Display Enhancements:
- **Search result indicators**: `[STREAM]` prefix for streaming tracks
- **Consistent terminology**: Use "streaming" vs "local" throughout
- **Clear streaming status** in now-playing displays

## Implementation Approach

### Low-Risk Strategy:
1. **Additive changes only** - no breaking changes to existing APIs
2. **Backward compatibility** - new fields optional, existing functionality unchanged
3. **Gradual rollout** - database migration handles existing data gracefully
4. **Optional features** - streaming filters/indicators are enhancements, not requirements

### Testing Strategy:
- **Use existing test scripts** (`test_internet_songs_properties.js`, `test_streaming_playback.js`, `test_current_streaming_track.js`)
- **Test with "Internet Songs" playlist** for streaming tracks
- **Verify local track handling** remains unchanged
- **Test database migration** with existing libraries

## Expected Benefits

### For Users:
- **Clear streaming identification** in search results and status
- **Enhanced search capabilities** with streaming/local filtering
- **Better understanding** of content type (streaming vs local)
- **Improved debugging** with stream URLs and track kinds

### For System:
- **Future-proof architecture** for additional streaming features
- **Consistent metadata** across streaming and local tracks
- **Enhanced analytics** potential with streaming usage data
- **Better error handling** with streaming-specific context

## Files Modified Summary

| File | Purpose | Changes |
|------|---------|---------|
| `database/schema.go` | Database structure | Add streaming fields and migration |
| `database/database.go` | Track struct and queries | Update Track struct, add streaming queries |
| `itunes/scripts/iTunes_Refresh_Library.js` | Library extraction | Extract streaming metadata |
| `itunes/itunes.go` | Core library interface | Update Track struct and conversion functions |
| `mcp-server/main.go` | MCP tool definitions | Enhance tools with streaming support |
| `CLAUDE.md` | Documentation | Document streaming track features |

## Effort Estimate

**Medium complexity** - Mostly additive changes leveraging existing infrastructure. Research has proven that streaming tracks work seamlessly with current JXA scripts, reducing implementation risk significantly.

## Risk Assessment

### Low Risk:
- Streaming tracks are fully scriptable (proven by research)
- Additive database changes maintain backward compatibility
- Existing playback mechanisms work without modification

### Medium Risk:
- Database migration for existing large libraries
- Ensuring consistent streaming detection across all tracks

### Mitigation:
- Comprehensive testing with existing test scripts
- Gradual feature rollout with optional enhancements
- Fallback to existing behavior if streaming detection fails

---

## Research Test Scripts Created

For future reference and validation:

1. **`test_internet_songs_properties.js`** - Examines streaming track properties vs local tracks
2. **`test_streaming_playback.js`** - Tests streaming track playback control capabilities  
3. **`test_current_streaming_track.js`** - Analyzes streaming tracks as current playing track

These scripts can be re-run to validate implementation or test new streaming track scenarios.