# iTunes Phase II Migration Plan: PostgreSQL + Simplified Normalized Schema

## 1. Revised Database Schema (Practical & Focused)

### Core Tables with Genre Support
```sql
-- Artists table
CREATE TABLE artists (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Genres table (shared across albums, playlists, tracks)
CREATE TABLE genres (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Albums table with genre support
CREATE TABLE albums (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    artist_id INTEGER NOT NULL REFERENCES artists(id),
    genre_id INTEGER REFERENCES genres(id),
    year INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name, artist_id)
);

-- Simplified tracks table (focused on search & playback)
CREATE TABLE tracks (
    id SERIAL PRIMARY KEY,
    itunes_id VARCHAR(50) NOT NULL UNIQUE, -- Apple Music persistentID
    name VARCHAR(255) NOT NULL,
    artist_id INTEGER NOT NULL REFERENCES artists(id),
    album_id INTEGER REFERENCES albums(id),
    genre_id INTEGER REFERENCES genres(id), -- Track-level genre override
    rating INTEGER CHECK (rating >= 0 AND rating <= 100), -- User rating
    starred BOOLEAN DEFAULT FALSE,
    ranking INTEGER, -- Custom ranking/priority
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Playlists table with genre support
CREATE TABLE playlists (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    genre_id INTEGER REFERENCES genres(id), -- Playlist genre/category
    special_kind VARCHAR(50), -- Apple Music specialKind
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Many-to-many relationship between playlists and tracks
CREATE TABLE playlist_tracks (
    id SERIAL PRIMARY KEY,
    playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id INTEGER NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(playlist_id, track_id)
);

-- Essential indexes for search performance
CREATE INDEX idx_tracks_name ON tracks(name);
CREATE INDEX idx_tracks_itunes_id ON tracks(itunes_id);
CREATE INDEX idx_tracks_artist ON tracks(artist_id);
CREATE INDEX idx_tracks_album ON tracks(album_id);
CREATE INDEX idx_tracks_genre ON tracks(genre_id);
CREATE INDEX idx_tracks_starred ON tracks(starred);
CREATE INDEX idx_tracks_rating ON tracks(rating);
CREATE INDEX idx_albums_genre ON albums(genre_id);
CREATE INDEX idx_playlists_genre ON playlists(genre_id);
CREATE INDEX idx_playlist_tracks_playlist ON playlist_tracks(playlist_id, position);
```

## 2. Docker Compose Setup
```yaml
# docker-compose.yml
version: '3.8'
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: itunes
      POSTGRES_USER: itunes
      POSTGRES_PASSWORD: itunes_password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U itunes"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
```

## 3. Database Layer (`database.go`)
```go
package itunes

import (
    "context"
    "time"
    
    "github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseManager struct {
    pool *pgxpool.Pool
}

// Track struct (enhanced from current minimal version)
type Track struct {
    ID         string   `json:"id"`         // Apple Music persistentID
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

type SearchFilters struct {
    Genre     string  // Filter by genre
    Artist    string  // Filter by artist
    Album     string  // Filter by album
    Playlist  string  // Filter by playlist
    Starred   *bool   // Filter by starred status (nil = any)
    MinRating int     // Minimum rating (0-100)
    Limit     int     // Result limit (default 15)
}

// Core database operations with shared pool
func NewDatabaseManager(ctx context.Context, connString string) (*DatabaseManager, error)
func (dm *DatabaseManager) Close()
func (dm *DatabaseManager) SearchTracks(ctx context.Context, query string, filters SearchFilters) ([]Track, error)
func (dm *DatabaseManager) GetTrackByItunesID(ctx context.Context, itunesID string) (*Track, error)
func (dm *DatabaseManager) GetPlaylistTracks(ctx context.Context, playlistName string) ([]Track, error)

// Batch operations for refresh service
func (dm *DatabaseManager) BatchInsertTracks(ctx context.Context, tracks []Track) error
func (dm *DatabaseManager) BatchUpdateTracks(ctx context.Context, tracks []Track) error
func (dm *DatabaseManager) BatchInsertPlaylistTracks(ctx context.Context, playlistName string, trackIDs []string) error
```

## 4. Background Refresh Service (`refresh-service/`)

### Standalone Binary (`bin/itunes-refresh`)
- **Independent service**: No CLI/MCP dependencies
- **Scheduled execution**: Run via cron/systemd timer
- **Enhanced JXA script**: Extract genres for tracks, albums, playlists
- **Batch operations**: Efficient bulk database updates
- **Incremental sync**: Only update changed tracks

### Core Operations
```go
// Enhanced refresh operations
func RefreshLibraryToDatabase(ctx context.Context, db *DatabaseManager) error
func BatchProcessTracks(ctx context.Context, db *DatabaseManager, tracks []Track) error
func SyncPlaylists(ctx context.Context, db *DatabaseManager, playlists map[string][]string) error
```

## 5. Migration Strategy

### Phase 1: Database Infrastructure
1. **Docker setup**: Create `docker-compose.yml` with PostgreSQL
2. **Schema creation**: Database tables with proper indexes
3. **Connection pool**: Implement `database.go` with shared pgxpool
4. **Migration tools**: Scripts to convert existing JSON cache

### Phase 2: Background Refresh Service
1. **Standalone binary**: Create `refresh-service/` package
2. **Enhanced JXA**: Modify scripts to extract genre information
3. **Batch operations**: Implement efficient bulk insert/update
4. **Scheduling**: Setup cron job for automatic refreshes

### Phase 3: API Integration
1. **Search replacement**: Replace `SearchTracksFromCache()` with database queries
2. **Advanced filtering**: Add genre, rating, starred filters to MCP tools
3. **Backward compatibility**: Maintain existing MCP resource interfaces
4. **CLI updates**: Update CLI to use database backend

### Phase 4: Migration & Testing
1. **Data migration**: Convert existing JSON cache to database
2. **Performance testing**: Ensure search performance matches current system (~1-5ms)
3. **Integration testing**: Verify MCP server and CLI functionality
4. **Cleanup**: Remove file-based cache system

## 6. Database Technology Considerations

### PostgreSQL vs SQLite Analysis

**CRITICAL PERFORMANCE CONCERN**: The current system achieves ~1-5ms search performance through in-memory JSON operations. This is a strict requirement that drives technology choice.

#### Option A: PostgreSQL (Original Plan)
**Pros:**
- Robust concurrent access
- Advanced indexing (pg_trgm for text search)
- Full SQL feature set
- Excellent for future scaling

**Cons:**
- **Performance Risk**: Network latency + query overhead likely exceeds 1-5ms target
- More complex deployment (Docker, connection pooling)
- May require hybrid in-memory caching to meet performance goals

#### Option B: SQLite (Alternative Recommendation)
**Pros:**
- **Zero network latency**: Embedded database = much better chance of meeting 1-5ms target
- **Simplified deployment**: Single file, no server management
- **FTS support**: Built-in Full-Text Search for fast text queries
- **Excellent Go support**: `modernc.org/sqlite` (pure Go) or `mattn/go-sqlite3` (CGO)
- **Concurrent reads**: Multiple read transactions, single writer (sufficient for our use case)

**Cons:**
- Single writer limitation (not an issue - only refresh service writes)
- Fewer advanced features than PostgreSQL
- File-based (though this matches our current cache approach)

### Recommended Approach: SQLite with Performance Validation

Given the strict 1-5ms requirement and single-writer usage pattern, **SQLite appears better suited** for this application:

```sql
-- SQLite schema with FTS for fast search
CREATE TABLE tracks_fts (
    id INTEGER PRIMARY KEY,
    itunes_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    artist TEXT NOT NULL,
    album TEXT NOT NULL,
    genre TEXT,
    content TEXT -- Combined searchable text: name + artist + album
);

CREATE VIRTUAL TABLE tracks_search USING fts5(
    content,
    content=tracks_fts,
    content_rowid=id
);

-- Triggers to keep FTS table synchronized
CREATE TRIGGER tracks_fts_insert AFTER INSERT ON tracks_fts 
BEGIN
    INSERT INTO tracks_search(rowid, content) VALUES (new.id, new.content);
END;
```

## 7. Revised Migration Strategy (SQLite-First)

### Phase 1: Performance Validation & Database Setup
1. **SQLite PoC**: Create test database with realistic data (50k+ tracks)
2. **Performance benchmarking**: Validate 1-5ms search target is achievable
3. **If SQLite succeeds**: Proceed with SQLite implementation
4. **If SQLite fails**: Fall back to PostgreSQL + hybrid in-memory cache

### Phase 2: Schema & Ingestion Pipeline
1. **SQLite setup**: Single file database with FTS indexes
2. **Bulk ingestion**: Use SQLite's efficient batch insert capabilities
3. **Atomic refresh**: Use transactions for safe library updates

### Phase 3: Application Integration
1. **Database layer**: Implement with SQLite backend
2. **Search optimization**: Leverage FTS5 for fast text search
3. **Backward compatibility**: Maintain existing MCP interfaces

### Phase 4: Deployment & Migration
1. **Data migration**: Convert JSON cache to SQLite
2. **Performance validation**: Ensure production performance meets targets
3. **Cleanup**: Remove file-based cache system

## 8. Key Benefits of SQLite-Based Design

- **Performance-first**: Zero network latency maximizes chance of meeting 1-5ms target
- **Operational simplicity**: No separate database server to manage
- **Transactional safety**: ACID compliance for reliable library updates
- **Full-text search**: Built-in FTS5 for efficient text queries
- **Single file**: Easy backup, version control, and deployment
- **Battle-tested**: SQLite is the most deployed database engine worldwide
- **Go ecosystem**: Excellent tooling and ORM support

**Bottom Line**: SQLite's embedded nature and zero network latency make it the pragmatic choice for achieving the critical 1-5ms search performance requirement while still providing SQL capabilities and data persistence.