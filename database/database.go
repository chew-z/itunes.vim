package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Constants for database configuration
const (
	PrimaryDBPath = "~/Music/iTunes/itunes_library.db"
	BackupDBPath  = "~/Music/iTunes/itunes_library_backup.db"
)

// DatabaseManager handles all database operations
type DatabaseManager struct {
	DB *sql.DB
}

// Track represents a music track with Apple Music persistent ID
type Track struct {
	ID           int64
	PersistentID string
	Name         string
	Album        string
	Collection   string
	Artist       string
	Playlists    []string
	Genre        string
	Rating       int
	Starred      bool
	Ranking      float64
	Duration     int
	PlayCount    int
	LastPlayed   *time.Time
	DateAdded    *time.Time
}

// Playlist represents a playlist with Apple Music persistent ID
type Playlist struct {
	ID           int64
	PersistentID string
	Name         string
	Genre        string
	SpecialKind  string
	TrackCount   int
}

// SearchFilters contains search parameters
type SearchFilters struct {
	Genre         string
	Artist        string
	Album         string
	Playlist      string
	Starred       *bool
	MinRating     int
	Limit         int
	UsePlaylistID bool
}

// DatabaseStats contains database statistics
type DatabaseStats struct {
	TrackCount    int64
	PlaylistCount int64
	ArtistCount   int64
	AlbumCount    int64
	GenreCount    int64
	DatabaseSize  int64
}

// NewDatabaseManager creates a new database manager instance
func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	// Expand home directory
	if strings.HasPrefix(dbPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = filepath.Join(home, dbPath[2:])
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	dm := &DatabaseManager{DB: db}

	// Initialize schema
	if err := InitSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return dm, nil
}

// Close closes the database connection
func (dm *DatabaseManager) Close() error {
	return dm.DB.Close()
}

// GetOrCreateArtist gets or creates an artist and returns its ID
func (dm *DatabaseManager) GetOrCreateArtist(name string) (int64, error) {
	if name == "" {
		name = "Unknown Artist"
	}

	var id int64
	err := dm.DB.QueryRow("SELECT id FROM artists WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to query artist: %w", err)
	}

	// Artist doesn't exist, create it
	result, err := dm.DB.Exec("INSERT INTO artists (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("failed to insert artist: %w", err)
	}

	return result.LastInsertId()
}

// GetOrCreateGenre gets or creates a genre and returns its ID
func (dm *DatabaseManager) GetOrCreateGenre(name string) (int64, error) {
	if name == "" {
		name = "Unknown"
	}

	var id int64
	err := dm.DB.QueryRow("SELECT id FROM genres WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to query genre: %w", err)
	}

	// Genre doesn't exist, create it
	result, err := dm.DB.Exec("INSERT INTO genres (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("failed to insert genre: %w", err)
	}

	return result.LastInsertId()
}

// GetOrCreateAlbum gets or creates an album and returns its ID
func (dm *DatabaseManager) GetOrCreateAlbum(name string, artistID int64, genreID int64) (int64, error) {
	if name == "" {
		name = "Unknown Album"
	}

	var id int64
	err := dm.DB.QueryRow("SELECT id FROM albums WHERE name = ? AND artist_id = ?", name, artistID).Scan(&id)
	if err == nil {
		return id, nil
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to query album: %w", err)
	}

	// Album doesn't exist, create it
	result, err := dm.DB.Exec("INSERT INTO albums (name, artist_id, genre_id) VALUES (?, ?, ?)", name, artistID, genreID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert album: %w", err)
	}

	return result.LastInsertId()
}

// InsertTrack inserts a new track
func (dm *DatabaseManager) InsertTrack(track *Track) error {
	// Get or create related entities
	artistID, err := dm.GetOrCreateArtist(track.Artist)
	if err != nil {
		return fmt.Errorf("failed to get/create artist: %w", err)
	}

	genreID, err := dm.GetOrCreateGenre(track.Genre)
	if err != nil {
		return fmt.Errorf("failed to get/create genre: %w", err)
	}

	albumID, err := dm.GetOrCreateAlbum(track.Album, artistID, genreID)
	if err != nil {
		return fmt.Errorf("failed to get/create album: %w", err)
	}

	// Insert track
	query := `
		INSERT INTO tracks (
			persistent_id, name, artist_id, album_id, genre_id,
			collection, rating, starred, ranking, duration,
			play_count, last_played, date_added
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := dm.DB.Exec(query,
		track.PersistentID, track.Name, artistID, albumID, genreID,
		track.Collection, track.Rating, track.Starred, track.Ranking, track.Duration,
		track.PlayCount, track.LastPlayed, track.DateAdded,
	)
	if err != nil {
		return fmt.Errorf("failed to insert track: %w", err)
	}

	track.ID, err = result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return nil
}

// GetTrackByPersistentID retrieves a track by its Apple Music persistent ID
func (dm *DatabaseManager) GetTrackByPersistentID(persistentID string) (*Track, error) {
	query := `
		SELECT
			t.id, t.persistent_id, t.name, al.name, t.collection,
			ar.name, g.name, t.rating, t.starred, t.ranking,
			t.duration, t.play_count, t.last_played, t.date_added
		FROM tracks t
		LEFT JOIN artists ar ON ar.id = t.artist_id
		LEFT JOIN albums al ON al.id = t.album_id
		LEFT JOIN genres g ON g.id = t.genre_id
		WHERE t.persistent_id = ?
	`

	track := &Track{}
	err := dm.DB.QueryRow(query, persistentID).Scan(
		&track.ID, &track.PersistentID, &track.Name, &track.Album, &track.Collection,
		&track.Artist, &track.Genre, &track.Rating, &track.Starred, &track.Ranking,
		&track.Duration, &track.PlayCount, &track.LastPlayed, &track.DateAdded,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query track: %w", err)
	}

	// Get playlists for the track
	playlistQuery := `
		SELECT p.name
		FROM playlists p
		JOIN playlist_tracks pt ON pt.playlist_id = p.id
		WHERE pt.track_id = ?
		ORDER BY p.name
	`

	rows, err := dm.DB.Query(playlistQuery, track.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlists: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var playlistName string
		if err := rows.Scan(&playlistName); err != nil {
			return nil, fmt.Errorf("failed to scan playlist: %w", err)
		}
		track.Playlists = append(track.Playlists, playlistName)
	}

	return track, nil
}

// SearchTracks searches for tracks with filters
func (dm *DatabaseManager) SearchTracks(query string, filters *SearchFilters) ([]Track, error) {
	if filters == nil {
		filters = &SearchFilters{Limit: 15}
	}
	if filters.Limit <= 0 {
		filters.Limit = 15
	}

	// Build WHERE clause
	var conditions []string
	var args []interface{}

	if query != "" {
		conditions = append(conditions, "(t.name LIKE ? OR ar.name LIKE ? OR al.name LIKE ?)")
		searchPattern := "%" + query + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	if filters.Genre != "" {
		conditions = append(conditions, "g.name = ?")
		args = append(args, filters.Genre)
	}

	if filters.Artist != "" {
		conditions = append(conditions, "ar.name = ?")
		args = append(args, filters.Artist)
	}

	if filters.Album != "" {
		conditions = append(conditions, "al.name = ?")
		args = append(args, filters.Album)
	}

	if filters.Starred != nil && *filters.Starred {
		conditions = append(conditions, "t.starred = 1")
	}

	if filters.MinRating > 0 {
		conditions = append(conditions, "t.rating >= ?")
		args = append(args, filters.MinRating)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	sqlQuery := fmt.Sprintf(`
		SELECT
			t.id, t.persistent_id, t.name, al.name, t.collection,
			ar.name, g.name, t.rating, t.starred, t.ranking,
			t.duration, t.play_count, t.last_played, t.date_added
		FROM tracks t
		LEFT JOIN artists ar ON ar.id = t.artist_id
		LEFT JOIN albums al ON al.id = t.album_id
		LEFT JOIN genres g ON g.id = t.genre_id
		%s
		ORDER BY t.ranking DESC, t.name ASC
		LIMIT ?
	`, whereClause)

	args = append(args, filters.Limit)

	rows, err := dm.DB.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tracks: %w", err)
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		track := Track{}
		err := rows.Scan(
			&track.ID, &track.PersistentID, &track.Name, &track.Album, &track.Collection,
			&track.Artist, &track.Genre, &track.Rating, &track.Starred, &track.Ranking,
			&track.Duration, &track.PlayCount, &track.LastPlayed, &track.DateAdded,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan track: %w", err)
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// SearchTracksWithFTS searches tracks using full-text search
func (dm *DatabaseManager) SearchTracksWithFTS(query string, filters *SearchFilters) ([]Track, error) {
	if filters == nil {
		filters = &SearchFilters{Limit: 15}
	}
	if filters.Limit <= 0 {
		filters.Limit = 15
	}

	// Build WHERE clause for additional filters
	var conditions []string
	var args []interface{}

	// Add FTS match condition
	if query != "" {
		conditions = append(conditions, "t.id IN (SELECT rowid FROM tracks_fts WHERE tracks_fts MATCH ?)")
		args = append(args, query)
	}

	if filters.Genre != "" {
		conditions = append(conditions, "g.name = ?")
		args = append(args, filters.Genre)
	}

	if filters.Artist != "" {
		conditions = append(conditions, "ar.name = ?")
		args = append(args, filters.Artist)
	}

	if filters.Album != "" {
		conditions = append(conditions, "al.name = ?")
		args = append(args, filters.Album)
	}

	if filters.Starred != nil && *filters.Starred {
		conditions = append(conditions, "t.starred = 1")
	}

	if filters.MinRating > 0 {
		conditions = append(conditions, "t.rating >= ?")
		args = append(args, filters.MinRating)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	sqlQuery := fmt.Sprintf(`
		SELECT
			t.id, t.persistent_id, t.name, al.name, t.collection,
			ar.name, g.name, t.rating, t.starred, t.ranking,
			t.duration, t.play_count, t.last_played, t.date_added
		FROM tracks t
		LEFT JOIN artists ar ON ar.id = t.artist_id
		LEFT JOIN albums al ON al.id = t.album_id
		LEFT JOIN genres g ON g.id = t.genre_id
		%s
		ORDER BY t.ranking DESC, t.name ASC
		LIMIT ?
	`, whereClause)

	args = append(args, filters.Limit)

	rows, err := dm.DB.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tracks with FTS: %w", err)
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		track := Track{}
		err := rows.Scan(
			&track.ID, &track.PersistentID, &track.Name, &track.Album, &track.Collection,
			&track.Artist, &track.Genre, &track.Rating, &track.Starred, &track.Ranking,
			&track.Duration, &track.PlayCount, &track.LastPlayed, &track.DateAdded,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan track: %w", err)
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// GetStats returns database statistics
func (dm *DatabaseManager) GetStats() (*DatabaseStats, error) {
	stats := &DatabaseStats{}

	// Get counts
	queries := map[string]*int64{
		"SELECT COUNT(*) FROM tracks":    &stats.TrackCount,
		"SELECT COUNT(*) FROM playlists": &stats.PlaylistCount,
		"SELECT COUNT(*) FROM artists":   &stats.ArtistCount,
		"SELECT COUNT(*) FROM albums":    &stats.AlbumCount,
		"SELECT COUNT(*) FROM genres":    &stats.GenreCount,
	}

	for query, target := range queries {
		if err := dm.DB.QueryRow(query).Scan(target); err != nil {
			return nil, fmt.Errorf("failed to get count: %w", err)
		}
	}

	// Get database size
	var pageCount, pageSize int64
	if err := dm.DB.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		log.Printf("Failed to get page count: %v", err)
	}
	if err := dm.DB.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		log.Printf("Failed to get page size: %v", err)
	}
	stats.DatabaseSize = pageCount * pageSize

	return stats, nil
}

// Vacuum optimizes the database by reclaiming unused space and defragmenting tables
func (dm *DatabaseManager) Vacuum() error {
	_, err := dm.DB.Exec("VACUUM")
	return err
}

// RunMigrations ensures all database migrations are applied
func (dm *DatabaseManager) RunMigrations() error {
	return InitSchema(dm.DB)
}

// BatchInsertTracks inserts multiple tracks in a single transaction
func (dm *DatabaseManager) BatchInsertTracks(tracks []Track) error {
	tx, err := dm.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statements
	trackStmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO tracks (
			persistent_id, name, artist_id, album_id, genre_id,
			collection, rating, starred, ranking, duration,
			play_count, last_played, date_added, created_at, updated_at
		) VALUES (
			?, ?,
			(SELECT id FROM artists WHERE name = ?),
			(SELECT id FROM albums WHERE name = ? AND artist_id = (SELECT id FROM artists WHERE name = ?)),
			(SELECT id FROM genres WHERE name = ?),
			?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare track statement: %w", err)
	}
	defer trackStmt.Close()

	for _, track := range tracks {
		// Ensure artist, album, and genre exist
		if track.Artist != "" {
			if err := dm.upsertArtistTx(tx, track.Artist); err != nil {
				return fmt.Errorf("failed to upsert artist: %w", err)
			}
		}

		genre := track.Genre
		if genre == "" {
			genre = "Unknown"
		}
		if err := dm.upsertGenreTx(tx, genre); err != nil {
			return fmt.Errorf("failed to upsert genre: %w", err)
		}

		if track.Album != "" && track.Artist != "" {
			if err := dm.upsertAlbumTx(tx, track.Album, track.Artist, genre); err != nil {
				return fmt.Errorf("failed to upsert album: %w", err)
			}
		}

		// Insert track
		_, err := trackStmt.Exec(
			track.PersistentID, track.Name, track.Artist,
			track.Album, track.Artist, genre,
			track.Collection, track.Rating, track.Starred,
			track.Ranking, track.Duration, track.PlayCount,
			track.LastPlayed, track.DateAdded,
		)
		if err != nil {
			return fmt.Errorf("failed to insert track '%s': %w", track.Name, err)
		}
	}

	return tx.Commit()
}

// UpsertArtist inserts or updates an artist
func (dm *DatabaseManager) UpsertArtist(name string) error {
	_, err := dm.DB.Exec(`INSERT OR IGNORE INTO artists (name) VALUES (?)`, name)
	return err
}

// upsertArtistTx inserts or updates an artist within a transaction
func (dm *DatabaseManager) upsertArtistTx(tx *sql.Tx, name string) error {
	_, err := tx.Exec(`INSERT OR IGNORE INTO artists (name) VALUES (?)`, name)
	return err
}

// UpsertAlbum inserts or updates an album
func (dm *DatabaseManager) UpsertAlbum(name, artistName, genreName string) error {
	// Ensure artist and genre exist
	if err := dm.UpsertArtist(artistName); err != nil {
		return err
	}
	if err := dm.UpsertGenre(genreName); err != nil {
		return err
	}

	_, err := dm.DB.Exec(`
		INSERT OR IGNORE INTO albums (name, artist_id, genre_id)
		VALUES (?,
			(SELECT id FROM artists WHERE name = ?),
			(SELECT id FROM genres WHERE name = ?))
	`, name, artistName, genreName)
	return err
}

// upsertAlbumTx inserts or updates an album within a transaction
func (dm *DatabaseManager) upsertAlbumTx(tx *sql.Tx, name, artistName, genreName string) error {
	_, err := tx.Exec(`
		INSERT OR IGNORE INTO albums (name, artist_id, genre_id)
		VALUES (?,
			(SELECT id FROM artists WHERE name = ?),
			(SELECT id FROM genres WHERE name = ?))
	`, name, artistName, genreName)
	return err
}

// UpsertGenre inserts or updates a genre
func (dm *DatabaseManager) UpsertGenre(name string) error {
	_, err := dm.DB.Exec(`INSERT OR IGNORE INTO genres (name) VALUES (?)`, name)
	return err
}

// upsertGenreTx inserts or updates a genre within a transaction
func (dm *DatabaseManager) upsertGenreTx(tx *sql.Tx, name string) error {
	_, err := tx.Exec(`INSERT OR IGNORE INTO genres (name) VALUES (?)`, name)
	return err
}

// SyncPlaylist updates playlist-track relationships
func (dm *DatabaseManager) SyncPlaylist(playlistID string, trackPersistentIDs []string) error {
	tx, err := dm.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get playlist internal ID
	var internalPlaylistID int64
	err = tx.QueryRow(`SELECT id FROM playlists WHERE persistent_id = ?`, playlistID).Scan(&internalPlaylistID)
	if err != nil {
		return fmt.Errorf("playlist not found: %w", err)
	}

	// Clear existing associations
	_, err = tx.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ?`, internalPlaylistID)
	if err != nil {
		return fmt.Errorf("failed to clear playlist tracks: %w", err)
	}

	// Insert new associations
	stmt, err := tx.Prepare(`
		INSERT INTO playlist_tracks (playlist_id, track_id, position)
		SELECT ?, id, ? FROM tracks WHERE persistent_id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, trackID := range trackPersistentIDs {
		_, err := stmt.Exec(internalPlaylistID, i, trackID)
		if err != nil {
			return fmt.Errorf("failed to add track to playlist: %w", err)
		}
	}

	return tx.Commit()
}

// GetPlaylistByPersistentID retrieves a playlist by its persistent ID
func (dm *DatabaseManager) GetPlaylistByPersistentID(persistentID string) (*Playlist, error) {
	playlist := &Playlist{}
	err := dm.DB.QueryRow(`
		SELECT p.id, p.persistent_id, p.name, COALESCE(g.name, ''), p.special_kind
		FROM playlists p
		LEFT JOIN genres g ON g.id = p.genre_id
		WHERE p.persistent_id = ?
	`, persistentID).Scan(&playlist.ID, &playlist.PersistentID, &playlist.Name, &playlist.Genre, &playlist.SpecialKind)

	if err != nil {
		return nil, fmt.Errorf("failed to get playlist: %w", err)
	}

	// Get track count
	err = dm.DB.QueryRow(`
		SELECT COUNT(*)
		FROM playlist_tracks pt
		JOIN playlists p ON p.id = pt.playlist_id
		WHERE p.persistent_id = ?
	`, persistentID).Scan(&playlist.TrackCount)

	if err != nil {
		log.Printf("Failed to get playlist track count: %v", err)
	}

	return playlist, nil
}

// ListPlaylists returns all user-created playlists
func (dm *DatabaseManager) ListPlaylists() ([]Playlist, error) {
	rows, err := dm.DB.Query(`
		SELECT p.id, p.persistent_id, p.name, COALESCE(g.name, ''), p.special_kind,
			(SELECT COUNT(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id) as track_count
		FROM playlists p
		LEFT JOIN genres g ON g.id = p.genre_id
		WHERE p.special_kind = 'none' OR p.special_kind IS NULL
		ORDER BY p.name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlists: %w", err)
	}
	defer rows.Close()

	var playlists []Playlist
	for rows.Next() {
		var playlist Playlist
		err := rows.Scan(&playlist.ID, &playlist.PersistentID, &playlist.Name, &playlist.Genre, &playlist.SpecialKind, &playlist.TrackCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan playlist: %w", err)
		}
		playlists = append(playlists, playlist)
	}

	return playlists, nil
}

// GetPlaylistTracks returns all tracks in a playlist
func (dm *DatabaseManager) GetPlaylistTracks(playlistPersistentID string, usePlaylistID bool) ([]Track, error) {
	var whereClause string
	if usePlaylistID {
		whereClause = "p.persistent_id = ?"
	} else {
		whereClause = "p.name = ?"
	}

	query := fmt.Sprintf(`
		SELECT
			t.id, t.persistent_id, t.name, al.name, t.collection,
			ar.name, g.name, t.rating, t.starred, t.ranking,
			t.duration, t.play_count, t.last_played, t.date_added
		FROM tracks t
		JOIN playlist_tracks pt ON pt.track_id = t.id
		JOIN playlists p ON p.id = pt.playlist_id
		LEFT JOIN artists ar ON ar.id = t.artist_id
		LEFT JOIN albums al ON al.id = t.album_id
		LEFT JOIN genres g ON g.id = t.genre_id
		WHERE %s
		ORDER BY pt.position
	`, whereClause)

	rows, err := dm.DB.Query(query, playlistPersistentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query playlist tracks: %w", err)
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		track := Track{}
		err := rows.Scan(
			&track.ID, &track.PersistentID, &track.Name, &track.Album, &track.Collection,
			&track.Artist, &track.Genre, &track.Rating, &track.Starred, &track.Ranking,
			&track.Duration, &track.PlayCount, &track.LastPlayed, &track.DateAdded,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan track: %w", err)
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// BatchInsertPlaylistTracks associates multiple tracks with a playlist
func (dm *DatabaseManager) BatchInsertPlaylistTracks(playlistID int64, trackIDs []int64) error {
	tx, err := dm.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing associations
	_, err = tx.Exec("DELETE FROM playlist_tracks WHERE playlist_id = ?", playlistID)
	if err != nil {
		return fmt.Errorf("failed to clear existing tracks: %w", err)
	}

	// Prepare insert statement
	stmt, err := tx.Prepare("INSERT INTO playlist_tracks (playlist_id, track_id, position) VALUES (?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert tracks with positions
	for i, trackID := range trackIDs {
		_, err = stmt.Exec(playlistID, trackID, i+1)
		if err != nil {
			return fmt.Errorf("failed to insert track %d: %w", trackID, err)
		}
	}

	// Update playlist track count
	_, err = tx.Exec("UPDATE playlists SET track_count = ? WHERE id = ?", len(trackIDs), playlistID)
	if err != nil {
		return fmt.Errorf("failed to update track count: %w", err)
	}

	return tx.Commit()
}
