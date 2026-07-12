# Database

SQLite is the only persistent store — no cache files, no JSON state. Driver is
`modernc.org/sqlite` (pure Go, no CGO). Default path
`~/Music/iTunes/itunes_library.db`, overridable via `ITUNES_DB_PATH`.

## Schema (v6)

Normalized around Apple Music **persistent IDs** — the stable currency that
survives renames and library edits.

```
artists ──┐
genres ───┼──< tracks >──< playlist_tracks >── playlists
albums ───┘      │
                 └── tracks_fts (FTS5)

genres ───< radio_stations ── radio_stations_fts (FTS5)
```

### Tables

- **`tracks`** — `persistent_id` (unique), `name`, FKs to `artists`/`albums`/
  `genres`, `collection`, `rating` (0–100), `starred`, `ranking`,
  `is_streaming`, `track_kind`, `stream_url`, play stats, timestamps.
- **`playlists`** — `persistent_id` (unique), `name`, `special_kind`
  (`none` = user playlist), `track_count`.
- **`playlist_tracks`** — playlist↔track join with position.
- **`artists` / `albums` / `genres`** — name lookup tables.
- **`radio_stations`** — `name`, `url` (unique, `itmss://` playback protocol),
  `description`, `genre_id` (NOT NULL, FK to `genres`), `homepage`
  (`https://` web URL), `is_active`, `verified_at`.
  Note: no flat `genre` column — the Go layer joins `genres` and presents
  `genre` in JSON.
- **`schema_migrations`** — applied migration versions.

### FTS5

Two virtual tables kept in sync by triggers on insert/update/delete:

- `tracks_fts(name, artist_name, album_name)` — tokenizer
  `unicode61 remove_diacritics 2`, so "Kraków" matches "krakow".
- `radio_stations_fts(name, description, genre_name)`.

FTS matches on whole-word prefixes; the query builder in `database/search.go`
(`prepareFTSQuery`) sanitizes user input into FTS syntax.

### Migration history

| v | Change |
|---|---|
| 1 | Initial schema: artists, genres, albums, tracks, playlists, FTS5 |
| 2 | Streaming support: `is_streaming`, `track_kind`, `stream_url` |
| 3 | Radio stations table with genre integration |
| 4 | Station cleanup: dual-URL system (`itmss://` playback + `https://` homepage), superficial fields removed |
| 5 | `radio_stations.genre_id` made NOT NULL |
| 6 | Rebuild/repopulate stations FTS index |

Migrations live in `database/schema.go` (`InitSchema` applies pending versions
inside transactions; `MigrateDown` reverts).

## Search path

`database/search.go`:

1. `SearchManager.SearchWithCache` — checks the in-memory `SearchCache`
   (LRU-ish, TTL-based; hits in <5µs).
2. On miss, `SearchTracksOptimized` builds SQL via `SearchQueryBuilder`
   (FTS5 match + optional filters: genre/artist/album/playlist partial match,
   `min_rating`, `starred`, `streaming_only`/`local_only`) with relevance +
   popularity scoring.
3. Results cached, returned. Cold queries <7ms on an 11k-track library.

## Refresh / repopulation

`bin/itunes-migrate -from-script` (or the `refresh_library` MCP tool):

1. `RefreshLibraryCache` runs the JXA extraction script (~15 s for 11k tracks —
   see [JXA-AUTOMATION.md](JXA-AUTOMATION.md)) producing JSON with all tracks
   and playlists.
2. `Client.MigrateFromJSON` → `database/migrate.go` repopulates SQLite in
   batched transactions (~800 tracks/s) keyed by persistent ID — existing rows
   update, new rows insert. Radio stations are untouched by refresh.
3. Validation pass checks counts, FTS integrity, and sample searches
   (`bin/itunes-migrate -validate` runs it standalone).

## Operational notes

- Size: ~760 bytes/track including indexes (11k tracks ≈ 7 MB).
- The DB is safe to inspect read-only: `sqlite3 "file:...?immutable=1"`.
- `ITUNES_SEARCH_LIMIT` (default 15) caps result counts at the API layer.
