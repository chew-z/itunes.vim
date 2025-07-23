package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RefreshResponse matches the structure from iTunes refresh script
type RefreshResponse struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    *RefreshData           `json:"data"`
	Error   string                 `json:"error,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// RefreshData contains the library data from refresh
type RefreshData struct {
	Tracks    []JSONTrack    `json:"tracks"`
	Playlists []PlaylistData `json:"playlists"`
	Stats     RefreshStats   `json:"stats"`
}

// PlaylistData represents playlist metadata from refresh
type PlaylistData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	SpecialKind string `json:"special_kind"`
	TrackCount  int    `json:"track_count"`
	Genre       string `json:"genre,omitempty"`
}

// RefreshStats contains statistics from library refresh
type RefreshStats struct {
	TrackCount    int    `json:"track_count"`
	PlaylistCount int    `json:"playlist_count"`
	SkippedTracks int    `json:"skipped_tracks"`
	RefreshTime   string `json:"refresh_time"`
}

// JSONTrack represents the structure from iTunes JSON cache files
type JSONTrack struct {
	ID           string   `json:"id"`
	PersistentID string   `json:"persistent_id,omitempty"`
	Name         string   `json:"name"`
	Album        string   `json:"album"`
	Collection   string   `json:"collection"`
	Artist       string   `json:"artist"`
	Playlists    []string `json:"playlists"`
	Genre        string   `json:"genre,omitempty"`
	Rating       int      `json:"rating,omitempty"`
	Starred      bool     `json:"starred,omitempty"`
}

// MigrationProgress tracks the progress of a migration operation
type MigrationProgress struct {
	TotalTracks        int
	ProcessedTracks    int
	TotalPlaylists     int
	ProcessedPlaylists int
	StartTime          time.Time
	ElapsedTime        time.Duration
	Errors             []error
}

// ProgressCallback is called periodically during migration to report progress
type ProgressCallback func(progress MigrationProgress)

// MigrateFromJSON migrates data from JSON cache files to SQLite database
func (dm *DatabaseManager) MigrateFromJSON(cacheDir string, callback ProgressCallback) error {
	// Read the enhanced library file
	enhancedPath := filepath.Join(cacheDir, "library_enhanced.json")
	data, err := os.ReadFile(enhancedPath)
	if err != nil {
		// Try the regular library file
		libraryPath := filepath.Join(cacheDir, "library.json")
		data, err = os.ReadFile(libraryPath)
		if err != nil {
			return fmt.Errorf("failed to read library files: %w", err)
		}

		// Parse as track array for backward compatibility
		var tracks []JSONTrack
		if err := json.Unmarshal(data, &tracks); err != nil {
			return fmt.Errorf("failed to parse library.json: %w", err)
		}

		// Convert to refresh response format
		response := &RefreshResponse{
			Status: "success",
			Data:   &RefreshData{},
		}
		response.Data.Tracks = tracks

		// Extract playlists from track data for legacy format
		playlistMap := make(map[string]*PlaylistData)
		playlistTrackCounts := make(map[string]int)

		for _, track := range tracks {
			for _, playlistName := range track.Playlists {
				if _, exists := playlistMap[playlistName]; !exists {
					playlistMap[playlistName] = &PlaylistData{
						ID:          fmt.Sprintf("LEGACY_%s", strings.ReplaceAll(playlistName, " ", "_")),
						Name:        playlistName,
						SpecialKind: "none",
						TrackCount:  0,
					}
				}
				playlistTrackCounts[playlistName]++
			}
		}

		// Convert map to slice and set accurate track counts
		for name, playlist := range playlistMap {
			playlist.TrackCount = playlistTrackCounts[name]
			response.Data.Playlists = append(response.Data.Playlists, *playlist)
		}

		log.Printf("Extracted %d playlists from legacy format", len(response.Data.Playlists))

		return dm.populateFromRefreshResponse(response, callback)
	}

	// Parse enhanced library
	var response RefreshResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to parse library_enhanced.json: %w", err)
	}

	if response.Status != "success" {
		return fmt.Errorf("refresh response has error status: %s", response.Error)
	}

	return dm.populateFromRefreshResponse(&response, callback)
}

// PopulateFromRefreshScript populates the database from a RefreshResponse
func (dm *DatabaseManager) PopulateFromRefreshScript(response *RefreshResponse, callback ProgressCallback) error {
	if response.Status != "success" {
		return fmt.Errorf("refresh response has error status: %s", response.Error)
	}

	return dm.populateFromRefreshResponse(response, callback)
}

// populateFromRefreshResponse is the internal implementation for populating the database
func (dm *DatabaseManager) populateFromRefreshResponse(response *RefreshResponse, callback ProgressCallback) error {
	progress := MigrationProgress{
		TotalTracks:    len(response.Data.Tracks),
		TotalPlaylists: len(response.Data.Playlists),
		StartTime:      time.Now(),
		Errors:         []error{},
	}

	// Begin transaction for atomic operation
	tx, err := dm.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// First, insert all playlists
	playlistIDMap := make(map[string]int64)    // persistentID -> database ID
	playlistNameToID := make(map[string]int64) // name -> database ID

	for _, playlist := range response.Data.Playlists {
		playlistID, err := dm.insertPlaylistTx(tx, &playlist)
		if err != nil {
			progress.Errors = append(progress.Errors, fmt.Errorf("playlist %s: %w", playlist.Name, err))
			log.Printf("Error inserting playlist %s: %v", playlist.Name, err)
			continue
		}
		playlistIDMap[playlist.ID] = playlistID
		playlistNameToID[playlist.Name] = playlistID
		progress.ProcessedPlaylists++

		if callback != nil && progress.ProcessedPlaylists%10 == 0 {
			callback(progress)
		}
	}

	// Batch insert tracks
	const batchSize = 100
	for i := 0; i < len(response.Data.Tracks); i += batchSize {
		end := i + batchSize
		if end > len(response.Data.Tracks) {
			end = len(response.Data.Tracks)
		}

		batch := response.Data.Tracks[i:end]
		if err := dm.batchInsertTracksTx(tx, batch, playlistNameToID); err != nil {
			progress.Errors = append(progress.Errors, fmt.Errorf("batch %d-%d: %w", i, end, err))
			log.Printf("Error inserting batch %d-%d: %v", i, end, err)
			continue
		}

		progress.ProcessedTracks += len(batch)
		if callback != nil {
			progress.ElapsedTime = time.Since(progress.StartTime)
			callback(progress)
		}
	}

	// Update FTS index
	if err := dm.updateFTSIndexTx(tx); err != nil {
		return fmt.Errorf("failed to update FTS index: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Run ANALYZE to update statistics
	if _, err := dm.DB.Exec("ANALYZE"); err != nil {
		log.Printf("Warning: failed to run ANALYZE: %v", err)
	}

	// Final callback
	if callback != nil {
		progress.ElapsedTime = time.Since(progress.StartTime)
		callback(progress)
	}

	if len(progress.Errors) > 0 {
		log.Printf("Migration completed with %d errors", len(progress.Errors))
	}

	return nil
}

// insertPlaylistTx inserts a playlist within a transaction
func (dm *DatabaseManager) insertPlaylistTx(tx *sql.Tx, playlist *PlaylistData) (int64, error) {
	// Check if playlist already exists
	var existingID int64
	err := tx.QueryRow("SELECT id FROM playlists WHERE persistent_id = ?", playlist.ID).Scan(&existingID)
	if err == nil {
		// Update existing playlist
		_, err = tx.Exec(`
			UPDATE playlists
			SET name = ?, special_kind = ?, track_count = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, playlist.Name, playlist.SpecialKind, playlist.TrackCount, existingID)
		if err != nil {
			return 0, fmt.Errorf("failed to update playlist: %w", err)
		}
		return existingID, nil
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to check existing playlist: %w", err)
	}

	// Insert new playlist
	result, err := tx.Exec(`
		INSERT INTO playlists (persistent_id, name, special_kind, track_count)
		VALUES (?, ?, ?, ?)
	`, playlist.ID, playlist.Name, playlist.SpecialKind, playlist.TrackCount)
	if err != nil {
		return 0, fmt.Errorf("failed to insert playlist: %w", err)
	}

	return result.LastInsertId()
}

// batchInsertTracksTx inserts a batch of tracks within a transaction
func (dm *DatabaseManager) batchInsertTracksTx(tx *sql.Tx, tracks []JSONTrack, playlistNameToID map[string]int64) error {
	// Prepare statements for efficiency
	artistStmt, err := tx.Prepare("INSERT OR IGNORE INTO artists (name) VALUES (?)")
	if err != nil {
		return fmt.Errorf("failed to prepare artist statement: %w", err)
	}
	defer artistStmt.Close()

	genreStmt, err := tx.Prepare("INSERT OR IGNORE INTO genres (name) VALUES (?)")
	if err != nil {
		return fmt.Errorf("failed to prepare genre statement: %w", err)
	}
	defer genreStmt.Close()

	// Process each track
	for _, track := range tracks {
		trackID, err := dm.insertTrackTx(tx, &track, artistStmt, genreStmt)
		if err != nil {
			log.Printf("Error inserting track %s: %v", track.Name, err)
			continue
		}

		// Handle playlist associations
		if err := dm.insertPlaylistTracksTx(tx, trackID, track.Playlists, playlistNameToID); err != nil {
			log.Printf("Error associating track %s with playlists: %v", track.Name, err)
		}
	}

	return nil
}

// insertTrackTx inserts a single track within a transaction
func (dm *DatabaseManager) insertTrackTx(tx *sql.Tx, track *JSONTrack, artistStmt, genreStmt *sql.Stmt) (int64, error) {
	// Use ID as PersistentID if PersistentID is empty (legacy format)
	persistentID := track.PersistentID
	if persistentID == "" {
		persistentID = track.ID
	}

	// Check if track already exists
	var existingID int64
	err := tx.QueryRow("SELECT id FROM tracks WHERE persistent_id = ?", persistentID).Scan(&existingID)
	if err == nil {
		// Update existing track
		return dm.updateTrackTx(tx, existingID, track)
	}

	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to check existing track: %w", err)
	}

	// Ensure artist exists
	if track.Artist == "" {
		track.Artist = "Unknown Artist"
	}
	if _, err := artistStmt.Exec(track.Artist); err != nil {
		return 0, fmt.Errorf("failed to insert artist: %w", err)
	}

	var artistID int64
	err = tx.QueryRow("SELECT id FROM artists WHERE name = ?", track.Artist).Scan(&artistID)
	if err != nil {
		return 0, fmt.Errorf("failed to get artist ID: %w", err)
	}

	// Ensure genre exists
	if track.Genre == "" {
		track.Genre = "Unknown"
	}
	if _, err := genreStmt.Exec(track.Genre); err != nil {
		return 0, fmt.Errorf("failed to insert genre: %w", err)
	}

	var genreID int64
	err = tx.QueryRow("SELECT id FROM genres WHERE name = ?", track.Genre).Scan(&genreID)
	if err != nil {
		return 0, fmt.Errorf("failed to get genre ID: %w", err)
	}

	// Get or create album
	if track.Album == "" {
		track.Album = "Unknown Album"
	}

	var albumID int64
	err = tx.QueryRow("SELECT id FROM albums WHERE name = ? AND artist_id = ?", track.Album, artistID).Scan(&albumID)
	if err == sql.ErrNoRows {
		// Create album
		result, err := tx.Exec("INSERT INTO albums (name, artist_id, genre_id) VALUES (?, ?, ?)", track.Album, artistID, genreID)
		if err != nil {
			return 0, fmt.Errorf("failed to insert album: %w", err)
		}
		albumID, err = result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get album ID: %w", err)
		}
	} else if err != nil {
		return 0, fmt.Errorf("failed to check existing album: %w", err)
	}

	// Insert track
	result, err := tx.Exec(`
		INSERT INTO tracks (
			persistent_id, name, artist_id, album_id, genre_id,
			collection, rating, starred, date_added
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, persistentID, track.Name, artistID, albumID, genreID,
		track.Collection, track.Rating, track.Starred, time.Now())

	if err != nil {
		return 0, fmt.Errorf("failed to insert track: %w", err)
	}

	return result.LastInsertId()
}

// updateTrackTx updates an existing track within a transaction
func (dm *DatabaseManager) updateTrackTx(tx *sql.Tx, trackID int64, track *JSONTrack) (int64, error) {
	// Get or create artist
	if track.Artist == "" {
		track.Artist = "Unknown Artist"
	}

	var artistID int64
	err := tx.QueryRow("SELECT id FROM artists WHERE name = ?", track.Artist).Scan(&artistID)
	if err == sql.ErrNoRows {
		result, err := tx.Exec("INSERT INTO artists (name) VALUES (?)", track.Artist)
		if err != nil {
			return 0, fmt.Errorf("failed to insert artist: %w", err)
		}
		artistID, err = result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get artist ID: %w", err)
		}
	} else if err != nil {
		return 0, fmt.Errorf("failed to get artist ID: %w", err)
	}

	// Get or create genre
	if track.Genre == "" {
		track.Genre = "Unknown"
	}

	var genreID int64
	err = tx.QueryRow("SELECT id FROM genres WHERE name = ?", track.Genre).Scan(&genreID)
	if err == sql.ErrNoRows {
		result, err := tx.Exec("INSERT INTO genres (name) VALUES (?)", track.Genre)
		if err != nil {
			return 0, fmt.Errorf("failed to insert genre: %w", err)
		}
		genreID, err = result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get genre ID: %w", err)
		}
	} else if err != nil {
		return 0, fmt.Errorf("failed to get genre ID: %w", err)
	}

	// Get or create album
	if track.Album == "" {
		track.Album = "Unknown Album"
	}

	var albumID int64
	err = tx.QueryRow("SELECT id FROM albums WHERE name = ? AND artist_id = ?", track.Album, artistID).Scan(&albumID)
	if err == sql.ErrNoRows {
		result, err := tx.Exec("INSERT INTO albums (name, artist_id, genre_id) VALUES (?, ?, ?)", track.Album, artistID, genreID)
		if err != nil {
			return 0, fmt.Errorf("failed to insert album: %w", err)
		}
		albumID, err = result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get album ID: %w", err)
		}
	} else if err != nil {
		return 0, fmt.Errorf("failed to check existing album: %w", err)
	}

	// Update track
	_, err = tx.Exec(`
		UPDATE tracks SET
			name = ?, artist_id = ?, album_id = ?, genre_id = ?,
			collection = ?, rating = ?, starred = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, track.Name, artistID, albumID, genreID,
		track.Collection, track.Rating, track.Starred, trackID)

	if err != nil {
		return 0, fmt.Errorf("failed to update track: %w", err)
	}

	return trackID, nil
}

// insertPlaylistTracksTx associates a track with its playlists
func (dm *DatabaseManager) insertPlaylistTracksTx(tx *sql.Tx, trackID int64, playlistNames []string, playlistNameToID map[string]int64) error {
	// For each playlist name, try to find it in our map or database
	for _, playlistName := range playlistNames {
		// First check if we have a playlist with this name in our name map
		playlistID, found := playlistNameToID[playlistName]

		if !found {
			// Try to find by name in database
			err := tx.QueryRow("SELECT id FROM playlists WHERE name = ?", playlistName).Scan(&playlistID)
			if err == sql.ErrNoRows {
				// Create playlist if it doesn't exist (should rarely happen now)
				log.Printf("Creating untracked playlist: %s", playlistName)
				result, err := tx.Exec("INSERT INTO playlists (persistent_id, name, special_kind) VALUES (?, ?, ?)",
					fmt.Sprintf("UNTRACKED_%s", strings.ReplaceAll(playlistName, " ", "_")),
					playlistName, "none")
				if err != nil {
					log.Printf("Failed to create playlist %s: %v", playlistName, err)
					continue
				}
				playlistID, _ = result.LastInsertId()
			} else if err != nil {
				log.Printf("Failed to find playlist %s: %v", playlistName, err)
				continue
			}
		}

		// Get the next position for this playlist
		var maxPosition sql.NullInt64
		err := tx.QueryRow(`
			SELECT MAX(position) FROM playlist_tracks WHERE playlist_id = ?
		`, playlistID).Scan(&maxPosition)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Failed to get max position for playlist %d: %v", playlistID, err)
			continue
		}

		nextPosition := int64(0)
		if maxPosition.Valid {
			nextPosition = maxPosition.Int64 + 1
		}

		// Insert playlist track association
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO playlist_tracks (playlist_id, track_id, position)
			VALUES (?, ?, ?)
		`, playlistID, trackID, nextPosition)
		if err != nil {
			log.Printf("Failed to associate track with playlist %s: %v", playlistName, err)
		}
	}

	return nil
}

// updateFTSIndexTx rebuilds the FTS index within a transaction
func (dm *DatabaseManager) updateFTSIndexTx(tx *sql.Tx) error {
	// Clear existing FTS data
	if _, err := tx.Exec("DELETE FROM tracks_fts"); err != nil {
		return fmt.Errorf("failed to clear FTS index: %w", err)
	}

	// Rebuild FTS index
	_, err := tx.Exec(`
		INSERT INTO tracks_fts(rowid, name, artist_name, album_name)
		SELECT
			t.id,
			t.name,
			COALESCE(ar.name, 'Unknown Artist'),
			COALESCE(al.name, 'Unknown Album')
		FROM tracks t
		LEFT JOIN artists ar ON ar.id = t.artist_id
		LEFT JOIN albums al ON al.id = t.album_id
	`)
	if err != nil {
		return fmt.Errorf("failed to rebuild FTS index: %w", err)
	}

	return nil
}

// BatchUpdateTracks updates multiple tracks efficiently
func (dm *DatabaseManager) BatchUpdateTracks(tracks []JSONTrack) error {
	tx, err := dm.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statements
	artistStmt, err := tx.Prepare("INSERT OR IGNORE INTO artists (name) VALUES (?)")
	if err != nil {
		return fmt.Errorf("failed to prepare artist statement: %w", err)
	}
	defer artistStmt.Close()

	genreStmt, err := tx.Prepare("INSERT OR IGNORE INTO genres (name) VALUES (?)")
	if err != nil {
		return fmt.Errorf("failed to prepare genre statement: %w", err)
	}
	defer genreStmt.Close()

	for _, track := range tracks {
		var trackID int64
		// Use ID as PersistentID if PersistentID is empty (legacy format)
		persistentID := track.PersistentID
		if persistentID == "" {
			persistentID = track.ID
		}
		err := tx.QueryRow("SELECT id FROM tracks WHERE persistent_id = ?", persistentID).Scan(&trackID)
		if err == nil {
			// Update existing track
			if _, err := dm.updateTrackTx(tx, trackID, &track); err != nil {
				log.Printf("Error updating track %s: %v", track.Name, err)
			}
		} else if err == sql.ErrNoRows {
			// Insert new track
			if _, err := dm.insertTrackTx(tx, &track, artistStmt, genreStmt); err != nil {
				log.Printf("Error inserting track %s: %v", track.Name, err)
			}
		} else {
			log.Printf("Error checking track %s: %v", track.Name, err)
		}
	}

	return tx.Commit()
}

// UpsertPlaylist inserts or updates a playlist
func (dm *DatabaseManager) UpsertPlaylist(playlist *PlaylistData) error {
	tx, err := dm.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := dm.insertPlaylistTx(tx, playlist); err != nil {
		return err
	}

	return tx.Commit()
}

// ValidateMigration checks if the migration was successful
func (dm *DatabaseManager) ValidateMigration(cacheDir string) (bool, []string) {
	var issues []string

	stats, err := dm.GetStats()
	if err != nil {
		issues = append(issues, fmt.Sprintf("failed to get database stats: %v", err))
		return false, issues
	}

	// Basic validation
	if stats.TrackCount == 0 {
		issues = append(issues, "no tracks found in database")
		return false, issues
	}

	// Check FTS index
	var ftsCount int64
	err = dm.DB.QueryRow("SELECT COUNT(*) FROM tracks_fts").Scan(&ftsCount)
	if err != nil {
		issues = append(issues, fmt.Sprintf("failed to query FTS index: %v", err))
		return false, issues
	}

	if ftsCount != stats.TrackCount {
		issues = append(issues, fmt.Sprintf("FTS index count %d doesn't match track count %d", ftsCount, stats.TrackCount))
		return false, issues
	}

	// Check sample search
	tracks, err := dm.SearchTracksWithFTS("music", &SearchFilters{Limit: 1})
	if err != nil {
		issues = append(issues, fmt.Sprintf("failed to perform test search: %v", err))
		return false, issues
	}

	log.Printf("Migration validation successful: %d tracks, %d playlists, %d artists, %d albums",
		stats.TrackCount, stats.PlaylistCount, stats.ArtistCount, stats.AlbumCount)

	if len(tracks) > 0 {
		log.Printf("Sample search returned: %s by %s", tracks[0].Name, tracks[0].Artist)
	}

	return true, issues
}
