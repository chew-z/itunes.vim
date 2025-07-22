# iTunes Phase II Migration Plan: SQLite + Apple Music Persistent IDs

## 1. Revised Database Schema (Practical & Focused)

### SQLite Schema with Apple Music Persistent IDs & FTS5
```sql
-- Artists table
CREATE TABLE artists (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Genres table (shared across albums, playlists, tracks)
CREATE TABLE genres (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Albums table with genre support (no direct persistent ID - handled via tracks)
CREATE TABLE albums (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    artist_id INTEGER NOT NULL REFERENCES artists(id),
    genre_id INTEGER REFERENCES genres(id),
    year INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name, artist_id)
);

-- Tracks table with Apple Music persistent IDs
CREATE TABLE tracks (
    id INTEGER PRIMARY KEY,
    persistent_id TEXT NOT NULL UNIQUE, -- Apple Music persistentID (128-bit UUID as hex)
    name TEXT NOT NULL,
    artist_id INTEGER NOT NULL REFERENCES artists(id),
    album_id INTEGER REFERENCES albums(id),
    genre_id INTEGER REFERENCES genres(id), -- Track-level genre override
    rating INTEGER CHECK (rating >= 0 AND rating <= 100), -- User rating
    starred BOOLEAN DEFAULT FALSE,
    ranking INTEGER, -- Custom ranking/priority
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Playlists table with Apple Music persistent IDs
CREATE TABLE playlists (
    id INTEGER PRIMARY KEY,
    persistent_id TEXT UNIQUE, -- Apple Music playlist persistentID (128-bit UUID as hex)
    name TEXT NOT NULL UNIQUE,
    genre_id INTEGER REFERENCES genres(id), -- Playlist genre/category
    special_kind TEXT, -- Apple Music specialKind (e.g., "Library", "Music", "Purchased")
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Many-to-many relationship between playlists and tracks
CREATE TABLE playlist_tracks (
    id INTEGER PRIMARY KEY,
    playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id INTEGER NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(playlist_id, track_id)
);

-- Full-Text Search for fast track search (FTS5)
CREATE VIRTUAL TABLE tracks_fts USING fts5(
    persistent_id,
    name,
    artist_name,
    album_name,
    genre_name,
    playlist_names, -- Space-separated list of playlists containing this track
    content='', -- External content table (we'll populate manually)
    tokenize='porter'
);

-- Essential indexes for search performance
CREATE INDEX idx_tracks_persistent_id ON tracks(persistent_id);
CREATE INDEX idx_tracks_name ON tracks(name);
CREATE INDEX idx_tracks_artist ON tracks(artist_id);
CREATE INDEX idx_tracks_album ON tracks(album_id);
CREATE INDEX idx_tracks_genre ON tracks(genre_id);
CREATE INDEX idx_tracks_starred ON tracks(starred);
CREATE INDEX idx_tracks_rating ON tracks(rating);
CREATE INDEX idx_playlists_persistent_id ON playlists(persistent_id);
CREATE INDEX idx_playlists_name ON playlists(name);
CREATE INDEX idx_playlists_genre ON playlists(genre_id);
CREATE INDEX idx_playlist_tracks_playlist ON playlist_tracks(playlist_id, position);
CREATE INDEX idx_playlist_tracks_track ON playlist_tracks(track_id);

-- Triggers to maintain FTS5 search index
CREATE TRIGGER tracks_fts_insert AFTER INSERT ON tracks 
BEGIN
    INSERT INTO tracks_fts(rowid, persistent_id, name, artist_name, album_name, genre_name, playlist_names)
    SELECT 
        new.id,
        new.persistent_id,
        new.name,
        a.name,
        COALESCE(al.name, ''),
        COALESCE(g.name, ''),
        COALESCE(GROUP_CONCAT(p.name, ' '), '')
    FROM tracks t
    LEFT JOIN artists a ON t.artist_id = a.id
    LEFT JOIN albums al ON t.album_id = al.id  
    LEFT JOIN genres g ON t.genre_id = g.id
    LEFT JOIN playlist_tracks pt ON t.id = pt.track_id
    LEFT JOIN playlists p ON pt.playlist_id = p.id
    WHERE t.id = new.id
    GROUP BY t.id;
END;

CREATE TRIGGER tracks_fts_update AFTER UPDATE ON tracks 
BEGIN
    DELETE FROM tracks_fts WHERE rowid = old.id;
    INSERT INTO tracks_fts(rowid, persistent_id, name, artist_name, album_name, genre_name, playlist_names)
    SELECT 
        new.id,
        new.persistent_id,
        new.name,
        a.name,
        COALESCE(al.name, ''),
        COALESCE(g.name, ''),
        COALESCE(GROUP_CONCAT(p.name, ' '), '')
    FROM tracks t
    LEFT JOIN artists a ON t.artist_id = a.id
    LEFT JOIN albums al ON t.album_id = al.id  
    LEFT JOIN genres g ON t.genre_id = g.id
    LEFT JOIN playlist_tracks pt ON t.id = pt.track_id
    LEFT JOIN playlists p ON pt.playlist_id = p.id
    WHERE t.id = new.id
    GROUP BY t.id;
END;

CREATE TRIGGER tracks_fts_delete AFTER DELETE ON tracks 
BEGIN
    DELETE FROM tracks_fts WHERE rowid = old.id;
END;
```

## 2. SQLite Database Location & Management

### Database File Location
- **Primary location**: `$TMPDIR/itunes-cache/library.db` (matches current cache pattern)
- **Backup location**: `~/.config/itunes/library.db` (persistent across reboots)
- **Schema migrations**: Embedded SQL scripts with version tracking

### Database Initialization
```go
// Database file management in database.go
const (
    PrimaryDBPath = filepath.Join(os.TempDir(), "itunes-cache", "library.db")
    BackupDBPath  = filepath.Join(os.Getenv("HOME"), ".config", "itunes", "library.db")
    SchemaVersion = 1
)

func InitDatabase() error {
    // Create cache directory
    if err := os.MkdirAll(filepath.Dir(PrimaryDBPath), 0755); err != nil {
        return err
    }
    
    // Initialize with schema
    db, err := sql.Open("sqlite3", PrimaryDBPath)
    // ... schema creation and migrations
}
```

## 3. Database Layer (`database.go`)
```go
package itunes

import (
    "context"
    "database/sql"
    "time"
    
    _ "modernc.org/sqlite" // Pure Go SQLite driver
)

type DatabaseManager struct {
    db *sql.DB
}

// Enhanced Track struct with Apple Music persistent IDs
type Track struct {
    ID         string   `json:"id"`         // Apple Music persistentID (hex format)
    Name       string   `json:"name"`       // Track name
    Album      string   `json:"album"`      // Album name
    Collection string   `json:"collection"` // Primary playlist or album
    Artist     string   `json:"artist"`     // Artist name
    Playlists  []string `json:"playlists"`  // All playlists containing track
    Genre      string   `json:"genre"`      // Track/album genre
    Rating     *int     `json:"rating"`     // User rating (0-100, nil if unrated)
    Starred    bool     `json:"starred"`    // Starred/favorite status
    Ranking    *int     `json:"ranking"`    // Custom ranking/priority
}

// Enhanced Playlist struct with persistent IDs
type Playlist struct {
    ID          string `json:"id"`           // Apple Music playlist persistentID
    Name        string `json:"name"`         // Playlist name
    Genre       string `json:"genre"`        // Playlist genre/category
    SpecialKind string `json:"special_kind"` // Apple Music specialKind
    TrackCount  int    `json:"track_count"`  // Number of tracks
}

type SearchFilters struct {
    Genre       string  // Filter by genre
    Artist      string  // Filter by artist
    Album       string  // Filter by album
    Playlist    string  // Filter by playlist (supports both name and persistentID)
    Starred     *bool   // Filter by starred status (nil = any)
    MinRating   int     // Minimum rating (0-100)
    Limit       int     // Result limit (default 15)
    UsePlaylistID bool  // If true, Playlist field is treated as persistentID
}

// Core database operations with SQLite
func NewDatabaseManager(ctx context.Context, dbPath string) (*DatabaseManager, error)
func (dm *DatabaseManager) Close() error
func (dm *DatabaseManager) SearchTracks(ctx context.Context, query string, filters SearchFilters) ([]Track, error)
func (dm *DatabaseManager) SearchTracksWithFTS(ctx context.Context, query string, filters SearchFilters) ([]Track, error)
func (dm *DatabaseManager) GetTrackByPersistentID(ctx context.Context, persistentID string) (*Track, error)
func (dm *DatabaseManager) GetPlaylistTracks(ctx context.Context, playlistName string) ([]Track, error)
func (dm *DatabaseManager) GetPlaylistByPersistentID(ctx context.Context, persistentID string) (*Playlist, error)
func (dm *DatabaseManager) ListPlaylists(ctx context.Context) ([]Playlist, error)

// Batch operations for refresh service with persistent ID support
func (dm *DatabaseManager) BatchInsertTracks(ctx context.Context, tracks []Track) error
func (dm *DatabaseManager) BatchUpdateTracks(ctx context.Context, tracks []Track) error
func (dm *DatabaseManager) UpsertPlaylist(ctx context.Context, persistentID, name, specialKind, genre string) error
func (dm *DatabaseManager) BatchInsertPlaylistTracks(ctx context.Context, playlistPersistentID string, trackPersistentIDs []string) error

// Migration and maintenance
func (dm *DatabaseManager) RunMigrations(ctx context.Context) error
func (dm *DatabaseManager) Vacuum(ctx context.Context) error
func (dm *DatabaseManager) GetStats(ctx context.Context) (DatabaseStats, error)

type DatabaseStats struct {
    TrackCount    int `json:"track_count"`
    PlaylistCount int `json:"playlist_count"`
    ArtistCount   int `json:"artist_count"`
    AlbumCount    int `json:"album_count"`
    GenreCount    int `json:"genre_count"`
    DatabaseSize  int `json:"database_size_bytes"`
}
```

## 4. Enhanced Refresh Service with Persistent ID Support

### Standalone Binary (`bin/itunes-refresh`)
- **Independent service**: No CLI/MCP dependencies
- **Scheduled execution**: Run via cron/systemd timer
- **Enhanced JXA script**: Extract persistent IDs, genres for tracks, albums, playlists
- **Atomic transactions**: SQLite transactions for reliable library updates
- **Incremental sync**: Track-level change detection using persistent IDs
- **Playlist tracking**: Full playlist persistent ID and membership tracking

### Enhanced JXA Script Updates
```javascript
// Enhanced iTunes_Refresh_Library.js with persistent ID support
function extractTrackData() {
    let tracks = [];
    let music = Application('Music');
    
    // Get all tracks with persistent IDs
    let allTracks = music.libraryPlaylists[0].tracks;
    for (let i = 0; i < allTracks.length; i++) {
        let track = allTracks[i];
        tracks.push({
            persistent_id: track.persistentID(), // Apple Music 128-bit UUID
            name: track.name(),
            artist: track.artist(),
            album: track.album(),
            genre: track.genre(),
            rating: track.rating(),
            // ... other fields
        });
    }
    
    // Get all playlists with persistent IDs
    let playlists = [];
    let userPlaylists = music.userPlaylists;
    for (let i = 0; i < userPlaylists.length; i++) {
        let playlist = userPlaylists[i];
        let trackIds = [];
        for (let j = 0; j < playlist.tracks.length; j++) {
            trackIds.push(playlist.tracks[j].persistentID());
        }
        playlists.push({
            persistent_id: playlist.persistentID(), // Playlist persistent ID
            name: playlist.name(),
            special_kind: playlist.specialKind(),
            track_persistent_ids: trackIds
        });
    }
    
    return {tracks: tracks, playlists: playlists};
}
```

### Core Operations with Persistent ID Support
```go
// Enhanced refresh operations with persistent IDs
func RefreshLibraryToDatabase(ctx context.Context, db *DatabaseManager) error
func BatchProcessTracks(ctx context.Context, db *DatabaseManager, tracks []Track) error
func SyncPlaylistsWithPersistentIDs(ctx context.Context, db *DatabaseManager, playlists []PlaylistData) error
func DetectChangedTracks(ctx context.Context, db *DatabaseManager, newTracks []Track) ([]Track, error)
func CleanupOrphanedTracks(ctx context.Context, db *DatabaseManager, activePersistentIDs []string) error

type PlaylistData struct {
    PersistentID        string   `json:"persistent_id"`
    Name                string   `json:"name"`
    SpecialKind         string   `json:"special_kind"`
    TrackPersistentIDs  []string `json:"track_persistent_ids"`
}
```

## 5. Migration Strategy (SQLite-First with Persistent IDs)

### Phase 1: SQLite Infrastructure & Performance Validation
1. **SQLite setup**: Create schema with FTS5 and persistent ID indexes
2. **Performance PoC**: Build test database with realistic data (50k+ tracks)
3. **Benchmarking**: Validate 1-5ms search performance with FTS5 queries
4. **Database layer**: Implement `database.go` with SQLite backend

### Phase 2: Enhanced Refresh Service with Persistent IDs
1. **JXA script enhancement**: Extract both track and playlist persistent IDs
2. **Standalone binary**: Create `refresh-service/` with atomic SQLite transactions
3. **Incremental sync**: Use persistent IDs for change detection
4. **Playlist tracking**: Full playlist membership with persistent ID relationships

### Phase 3: Database-Backed API with FTS5 Search
1. **Search replacement**: Replace `SearchTracksFromCache()` with FTS5 database queries
2. **Persistent ID lookup**: Add `GetTrackByPersistentID()` and `GetPlaylistByPersistentID()`
3. **Advanced filtering**: Genre, rating, starred filters with SQL performance
4. **Backward compatibility**: Maintain existing MCP tool interfaces with enhanced data

### Phase 4: Migration, Testing & Deployment
1. **Data migration**: Convert existing JSON cache to SQLite with persistent ID mapping
2. **Performance validation**: Ensure FTS5 search meets 1-5ms target in production
3. **Integration testing**: Verify MCP server, CLI functionality with persistent IDs
4. **Fallback mechanism**: Keep JSON cache as backup during initial deployment
5. **Cleanup**: Remove file-based cache system after successful validation

## 6. Apple Music Persistent ID Integration Details

### Understanding Apple Music Persistent IDs

**Track Persistent IDs:**
- **Format**: 128-bit UUID represented as hexadecimal (e.g., `9F2DB5BF5802AF9A`)
- **Stability**: Remains constant across library rebuilds, app restarts, and macOS updates
- **Universality**: Same ID across all Apple Music clients (Mac, iOS, etc.)
- **Usage**: Primary key for reliable track identification and playbook

**Playlist Persistent IDs:**
- **Format**: Same 128-bit UUID format as tracks
- **Stability**: Persistent across playlist renames and library changes
- **Smart Playlists**: Have persistent IDs just like regular playlists
- **System Playlists**: Library, Purchased, etc. have stable persistent IDs

### Advantages of Persistent ID-Based Architecture

**Database Benefits:**
- **Reliable relationships**: Foreign keys based on stable persistent IDs
- **Change detection**: Efficient incremental sync using persistent ID comparison
- **Cross-session consistency**: Same track/playlist references across app restarts
- **Rename handling**: Playlist/track renames don't break relationships

**Playback Benefits:**  
- **Consistent playback**: Same persistent ID always plays the same track
- **Playlist context**: Reliable playlist-based continuous playback
- **Reliability**: Eliminates name encoding/parsing issues that caused intermittent failures

### Persistent ID Migration from Current System

**Current Issue**: 
- Existing system uses persistent IDs correctly (as confirmed in previous debugging)
- JSON cache structure already stores these IDs in the `id` field
- Database migration can directly map `track.id` → `tracks.persistent_id`

**Migration Mapping:**
```go
// JSON to SQLite migration
type JSONTrack struct {
    ID   string `json:"id"`   // Already contains persistent ID
    Name string `json:"name"`
    // ... other fields
}

func MigrateTrack(jsonTrack JSONTrack) Track {
    return Track{
        ID:   jsonTrack.ID,  // Direct mapping - no conversion needed
        Name: jsonTrack.Name,
        // ... other fields map directly
    }
}
```

## 7. SQLite Performance Optimization Strategy

### FTS5 Search Performance Tuning
```sql
-- Optimized FTS5 configuration for 1-5ms target
CREATE VIRTUAL TABLE tracks_fts USING fts5(
    persistent_id UNINDEXED, -- Don't index ID in FTS (use regular index)
    name,
    artist_name,
    album_name,
    genre_name,
    playlist_names,
    tokenize='porter ascii', -- ASCII tokenizer for performance
    prefix=2,               -- Enable 2-character prefix matching
    content='',             -- External content (faster than content=table)
    columnsize=0            -- Disable column size tracking (faster writes)
);

-- Optimize database for read performance
PRAGMA journal_mode = WAL;          -- Write-Ahead Logging for concurrent reads
PRAGMA synchronous = NORMAL;        -- Balance safety vs performance
PRAGMA cache_size = 10000;          -- 10MB cache for fast queries
PRAGMA temp_store = memory;         -- Use memory for temporary tables
PRAGMA mmap_size = 268435456;       -- 256MB memory-mapped I/O
```

### Benchmark Target Performance
- **Current JSON search**: ~1-5ms for 50k+ track libraries
- **Target SQLite+FTS5**: Match or exceed current performance
- **Acceptable fallback**: 10-15ms if search quality significantly improves
- **Performance validation**: Test with real user libraries during migration

## 8. Key Benefits of Apple Music Persistent ID Architecture

### Reliability Improvements
- **Eliminates playback failures**: Root cause of previous intermittent issues was string matching
- **Cross-platform consistency**: Same IDs work across Mac, iOS, Apple TV
- **Rename resilience**: Track/playlist renames don't break saved searches or playlists
- **Library rebuild safety**: Persistent IDs survive library reconstruction

### Developer Experience
- **Simplified debugging**: Consistent, readable hex IDs instead of internal DB integers  
- **API reliability**: No more encoding/parsing issues with complex track names
- **Future-proof**: Apple Music API uses these same persistent IDs
- **Caching strategy**: Persistent IDs enable reliable cross-session caching

### Database Design Benefits
- **Natural foreign keys**: Persistent IDs work directly as foreign key references
- **Incremental sync**: Compare persistent ID lists to detect additions/removals
- **Conflict resolution**: Persistent IDs provide canonical source of truth
- **Backup/restore**: Database exports remain valid across different iTunes libraries

## 9. Implementation Priority & Risk Assessment

### High Priority (Phase 1)
- **SQLite schema creation** with persistent ID support
- **FTS5 performance validation** with realistic data
- **Basic database operations** (insert, search, retrieve)

### Medium Priority (Phase 2)  
- **Enhanced refresh service** with incremental sync
- **Playlist persistent ID extraction** and management
- **Advanced filtering** (genre, rating, starred)

### Low Priority (Phase 3)
- **Migration tooling** from JSON cache
- **Backup/restore mechanisms**  
- **Database optimization** and maintenance tools

### Risk Mitigation
- **Performance fallback**: Keep JSON cache as backup during initial rollout
- **Gradual migration**: Enable SQLite backend as opt-in feature initially
- **A/B testing**: Compare search performance between JSON and SQLite backends
- **Rollback plan**: Quick reversion to JSON cache if performance issues arise

**Bottom Line**: Apple Music persistent IDs combined with SQLite FTS5 provides a robust, performant foundation that eliminates the root causes of previous playback reliability issues while maintaining the 1-5ms search performance requirement.