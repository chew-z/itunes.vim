# Streaming Track Response Structure Fix

*Generated: July 23, 2025*

## Problem Analysis

The current streaming track implementation is botched. Instead of providing completely different response structures for streaming vs local tracks as specified in the original plan, it incorrectly appends "[STREAMING]" to message fields and returns empty/null fields for streaming tracks.

### Current Botched Output:
```json
{
  "success": true,
  "message": "Now playing: SomaFM: Lush (#1): Sensuous and mellow female vocals, many with an electronic influence. [STREAMING]",
  "now_playing": {
    "status": "playing",
    "track": {
      "id": "B258396D58E2ECC9",
      "name": "SomaFM: Lush (#1): Sensuous and mellow female vocals, many with an electronic influence.",
      "artist": "",
      "album": "",
      "position": "2:38",
      "duration": "",
      "position_seconds": 158,
      "duration_seconds": 0
    },
    "display": "SomaFM: Lush (#1): Sensuous and mellow female vocals, many with an electronic influence. [STREAMING]"
  }
}
```

## Solution: Completely New Response Structure for Streaming Tracks

### Core Concept
Create two entirely different response structures - one for streaming tracks, one for local tracks. No shared fields between them except basic identifiers.

### New Streaming Track Response Structure:
```json
{
  "success": true,
  "message": "Started streaming: SomaFM: Lush",
  "now_playing": {
    "status": "streaming",
    "stream": {
      "id": "B258396D58E2ECC9",
      "name": "SomaFM: Lush (#1): Sensuous and mellow female vocals, many with an electronic influence.",
      "stream_url": "http://ice6.somafm.com/insound-128-aac",
      "kind": "Internet audio stream",
      "elapsed": "2:38",
      "elapsed_seconds": 158
    },
    "display": "SomaFM: Lush (#1): Sensuous and mellow female vocals, many with an electronic influence."
  }
}
```

### Local Track Response Structure (unchanged):
```json
{
  "success": true,
  "message": "Now playing: Humming In The Night by Akira Kosemura",
  "now_playing": {
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
}
```

## Implementation Plan

### 1. JavaScript Script Changes (`iTunes_Now_Playing.js`)

#### Current Issues:
- Line 83: Adds "[STREAMING]" to display field
- Returns empty artist/album fields for streaming tracks
- Uses same structure for both streaming and local tracks

#### Fix:
```javascript
if (isStreaming) {
    return JSON.stringify({
        status: "streaming",
        stream: {
            id: trackID,
            name: trackName,
            stream_url: streamURL,
            kind: trackKind,
            elapsed: formatTime(playerPosition),
            elapsed_seconds: Math.floor(playerPosition)
        },
        display: trackName
    });
} else {
    return JSON.stringify({
        status: "playing",
        track: {
            id: trackID,
            name: trackName,
            artist: artistName,
            album: albumName,
            position: formatTime(playerPosition),
            duration: formatTime(duration),
            position_seconds: Math.floor(playerPosition),
            duration_seconds: Math.floor(duration)
        },
        display: trackName + " – " + artistName
    });
}
```

### 2. Go Response Handler Changes (`itunes.go`)

#### Current Issues:
- Line 334: Uses display field directly in message, including "[STREAMING]"
- No detection of streaming vs local track types
- Single message format for both types

#### Fix:
```go
// Detect if this is a streaming track response
if nowPlaying.Status == "streaming" && nowPlaying.Stream != nil {
    result.Message = fmt.Sprintf("Started streaming: %s", nowPlaying.Stream.Name)
} else if nowPlaying.Status == "playing" && nowPlaying.Track != nil {
    if nowPlaying.Track.Artist != "" {
        result.Message = fmt.Sprintf("Now playing: %s by %s", nowPlaying.Track.Name, nowPlaying.Track.Artist)
    } else {
        result.Message = fmt.Sprintf("Now playing: %s", nowPlaying.Track.Name)
    }
} else {
    result.Message = "Playback command sent successfully"
}
```

### 3. Go Struct Updates

#### Add New Streaming Response Structures:
```go
type StreamingTrack struct {
    ID             string `json:"id"`
    Name           string `json:"name"`
    StreamURL      string `json:"stream_url"`
    Kind           string `json:"kind"`
    Elapsed        string `json:"elapsed"`
    ElapsedSeconds int    `json:"elapsed_seconds"`
}

type NowPlayingStatus struct {
    Status  string         `json:"status"`
    Track   *NowPlayingTrack `json:"track,omitempty"`   // For local tracks
    Stream  *StreamingTrack  `json:"stream,omitempty"`  // For streaming tracks
    Display string         `json:"display"`
    Message string         `json:"message"`
}
```

### 4. MCP Tools Updates (`mcp-server/main.go`)

#### Update tool descriptions and responses to handle both structures:
- `now_playing` tool: Return appropriate structure based on track type
- `play_track` tool: Generate appropriate success messages
- Update documentation to reflect new streaming response format

## Key Benefits

### 1. Clear Semantic Difference
- **Streaming**: `status: "streaming"`, `stream` object, `elapsed` time
- **Local**: `status: "playing"`, `track` object, `position`/`duration` time

### 2. No Field Confusion
- Streaming tracks don't have meaningless empty `artist`/`album` fields
- Local tracks don't have irrelevant `stream_url` fields
- Each structure contains only relevant information

### 3. Professional Messages
- **Streaming**: "Started streaming: SomaFM: Lush"
- **Local**: "Now playing: Humming In The Night by Akira Kosemura"
- No ugly "[STREAMING]" appendages

### 4. Future-Proof Design
- Easy to add streaming-specific features (bitrate, station metadata, etc.)
- Easy to add local track features without affecting streaming
- Clear API contract for consumers

## Files to Modify

1. **`itunes/scripts/iTunes_Now_Playing.js`** - Implement branching response generation
2. **`itunes/itunes.go`** - Add streaming structs, update response handling
3. **`mcp-server/main.go`** - Update tool responses to handle both structures
4. **`CLAUDE.md`** - Update documentation with new response examples

## Testing Strategy

1. **Test streaming tracks**: Use SomaFM stations from "Internet Songs" playlist
2. **Test local tracks**: Use regular library tracks
3. **Verify MCP tools**: Ensure both `now_playing` and `play_track` work correctly
4. **Validate response structures**: Confirm no "[STREAMING]" artifacts remain

## Expected Outcome

Clean, professional response structures that properly differentiate between streaming internet radio and local music files, providing appropriate metadata and messaging for each type without confusing empty fields or ugly string appendages.