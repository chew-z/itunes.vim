# JXA Automation

All Apple Music control goes through JavaScript for Automation (JXA) scripts
run via `osascript`. This layer is the only part of the system that talks to
the Music app.

## Execution model

Scripts live in `itunes/scripts/`, are embedded into the binaries with
`go:embed` (`itunes/scripts.go`), and are executed by a single helper:

```go
runScript(ctx, scriptContent, args) ([]byte, error)   // itunes/itunes.go
```

- Pipes the script to `osascript -l JavaScript - <args...>` via **stdin** —
  no temp files. Everything after `-` arrives as `argv` in the script's
  `run(argv)`.
- Uses `exec.CommandContext`, so caller context cancellation/deadline kills
  the osascript process (`signal: killed` in the error means the ctx deadline
  fired, not a script bug).
- Returns stdout even when the script fails, because scripts report structured
  errors as JSON on stdout.

### Exit-code convention

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | No results (not an error — e.g. nothing playing) |
| 2 | Script error |

`ITunesError.Unwrap()` exposes the underlying `*exec.ExitError` so callers can
`errors.As` to inspect codes.

### Timeouts

Each wrapper applies its own deadline on top of the caller's context: 10 s for
status/EQ/audio/playback, 30 s for stream URLs, 10 min for library refresh.

## Script inventory

| Script | Go wrapper | Purpose |
|---|---|---|
| `iTunes_Refresh_Library.js` | `RefreshLibraryCache` | Bulk-extract full library to JSON |
| `iTunes_Play_Playlist_Track.js` | `PlayPlaylistTrack(WithStatus)` | Play by persistent ID / playlist / name |
| `iTunes_Play_Stream_URL.js` | `PlayStreamURL` | Play `itmss://` / `https://` streams |
| `iTunes_Now_Playing.js` | `GetNowPlaying` | Current track + player state |
| `iTunes_Get_EQ.js` / `iTunes_Set_EQ.js` | `GetEQStatus` / `SetEQStatus` | EQ presets |
| `iTunes_Get_Audio_Output.js` | `GetAudioOutput` | Local vs AirPlay detection |
| `iTunes_List_AirPlay_Devices.js` | `ListAirPlayDevices` | Enumerate outputs |
| `iTunes_Set_AirPlay_Device.js` | `SetAirPlayDevice` | Switch output (local only) |

## The bulk-fetch rule (critical)

**Never loop per-object property reads over a large collection.** Each JXA
property access on an object reference is a separate Apple Event — a
synchronous round-trip to the Music app. Reading 8 properties × 11,000 tracks
is ~90,000 Apple Events and does not finish in any reasonable time (this made
`refresh_library` unusable as the library grew; killed even at a 10-minute
timeout).

Instead, fetch a property for the **whole collection in one Apple Event**,
receiving a parallel array:

```javascript
// WRONG — one Apple Event per track per property (minutes to never):
for (let i = 0; i < tracks.length; i++) {
    names.push(tracks[i].name())
}

// RIGHT — one Apple Event total (~milliseconds):
let names = libraryPlaylist.tracks.name()        // array of all names
let ids   = libraryPlaylist.tracks.persistentID() // parallel array
```

`iTunes_Refresh_Library.js` is built entirely on this pattern: parallel arrays
for id/name/artist/album/kind/genre/rating, a per-playlist bulk
`tracks.persistentID()` for membership mapping, and a separate
`urlTracks.address()` bulk fetch (stream URLs only exist on URL tracks) joined
by persistent ID. Full extraction of an 11k-track library: ~14 seconds.

Caveats of bulk mode:

- No per-item try/catch — a property that throws poisons the whole call.
  Fetch fragile properties on a narrower collection (as with `urlTracks`)
  or accept nulls in the array.
- Arrays are index-aligned; filter/skip logic must use indices consistently.

## Platform limitations

- **AirPlay**: macOS does not allow programmatic selection of a specific
  AirPlay device; `SetAirPlayDevice` can only switch back to local output.
  Device enumeration is likewise restricted.
- **EQ under AirPlay**: EQ status cannot be read or set while AirPlay is
  active; `GetEQStatus` detects this and returns a descriptive error.
- **Automation permission**: the calling terminal/app needs Automation rights
  for Music (System Settings → Privacy & Security → Automation). Without it,
  scripts fail with permission errors.
- **Station URLs**: in-app playback requires the `itmss://` protocol;
  `https://music.apple.com/...` URLs open the wrong content (see
  [DATABASE.md](DATABASE.md) migration v4).
