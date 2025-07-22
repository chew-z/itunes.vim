package database

import (
	"database/sql"
	"fmt"
	"log"
)

// SchemaVersion represents the current database schema version
const SchemaVersion = 1

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          string
	Down        string
}

// Schema contains all database migrations
var Schema = []Migration{
	{
		Version:     1,
		Description: "Initial schema with artists, genres, albums, tracks, playlists, and FTS5",
		Up: `
		-- Enable foreign keys
		PRAGMA foreign_keys = ON;

		-- Artists table
		CREATE TABLE IF NOT EXISTS artists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Genres table
		CREATE TABLE IF NOT EXISTS genres (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Albums table
		CREATE TABLE IF NOT EXISTS albums (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			artist_id INTEGER,
			genre_id INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (artist_id) REFERENCES artists(id),
			FOREIGN KEY (genre_id) REFERENCES genres(id),
			UNIQUE(name, artist_id)
		);

		-- Tracks table with Apple Music persistent IDs
		CREATE TABLE IF NOT EXISTS tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			persistent_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			artist_id INTEGER,
			album_id INTEGER,
			genre_id INTEGER,
			collection TEXT,
			rating INTEGER DEFAULT 0 CHECK (rating >= 0 AND rating <= 100),
			starred BOOLEAN DEFAULT 0,
			ranking REAL DEFAULT 0.0,
			duration INTEGER DEFAULT 0,
			play_count INTEGER DEFAULT 0,
			last_played DATETIME,
			date_added DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (artist_id) REFERENCES artists(id),
			FOREIGN KEY (album_id) REFERENCES albums(id),
			FOREIGN KEY (genre_id) REFERENCES genres(id)
		);

		-- Playlists table with Apple Music persistent IDs
		CREATE TABLE IF NOT EXISTS playlists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			persistent_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			special_kind TEXT,
			genre_id INTEGER,
			track_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (genre_id) REFERENCES genres(id)
		);

		-- Playlist tracks junction table
		CREATE TABLE IF NOT EXISTS playlist_tracks (
			playlist_id INTEGER NOT NULL,
			track_id INTEGER NOT NULL,
			position INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
			FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE,
			PRIMARY KEY (playlist_id, track_id),
			UNIQUE(playlist_id, position)
		);

		-- FTS5 virtual table for full-text search
		CREATE VIRTUAL TABLE IF NOT EXISTS tracks_fts USING fts5(
			name,
			artist_name,
			album_name,
			tokenize='unicode61 remove_diacritics 2'
		);

		-- Triggers to keep FTS5 table in sync
		CREATE TRIGGER IF NOT EXISTS tracks_fts_insert AFTER INSERT ON tracks
		BEGIN
			INSERT INTO tracks_fts(rowid, name, artist_name, album_name)
			SELECT
				NEW.id,
				NEW.name,
				COALESCE(a.name, 'Unknown Artist'),
				COALESCE(al.name, 'Unknown Album')
			FROM artists a
			LEFT JOIN albums al ON al.id = NEW.album_id
			WHERE a.id = NEW.artist_id;
		END;

		CREATE TRIGGER IF NOT EXISTS tracks_fts_update AFTER UPDATE ON tracks
		BEGIN
			UPDATE tracks_fts
			SET name = NEW.name,
				artist_name = COALESCE((SELECT name FROM artists WHERE id = NEW.artist_id), 'Unknown Artist'),
				album_name = COALESCE((SELECT name FROM albums WHERE id = NEW.album_id), 'Unknown Album')
			WHERE rowid = NEW.id;
		END;

		CREATE TRIGGER IF NOT EXISTS tracks_fts_delete AFTER DELETE ON tracks
		BEGIN
			DELETE FROM tracks_fts WHERE rowid = OLD.id;
		END;

		-- Update timestamp triggers
		CREATE TRIGGER IF NOT EXISTS update_artists_timestamp AFTER UPDATE ON artists
		BEGIN
			UPDATE artists SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;

		CREATE TRIGGER IF NOT EXISTS update_genres_timestamp AFTER UPDATE ON genres
		BEGIN
			UPDATE genres SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;

		CREATE TRIGGER IF NOT EXISTS update_albums_timestamp AFTER UPDATE ON albums
		BEGIN
			UPDATE albums SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;

		CREATE TRIGGER IF NOT EXISTS update_tracks_timestamp AFTER UPDATE ON tracks
		BEGIN
			UPDATE tracks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;

		CREATE TRIGGER IF NOT EXISTS update_playlists_timestamp AFTER UPDATE ON playlists
		BEGIN
			UPDATE playlists SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;

		-- Indexes for performance
		CREATE INDEX IF NOT EXISTS idx_tracks_persistent_id ON tracks(persistent_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_artist_id ON tracks(artist_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_album_id ON tracks(album_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_genre_id ON tracks(genre_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_collection ON tracks(collection);
		CREATE INDEX IF NOT EXISTS idx_tracks_starred ON tracks(starred);
		CREATE INDEX IF NOT EXISTS idx_tracks_rating ON tracks(rating);
		CREATE INDEX IF NOT EXISTS idx_tracks_ranking ON tracks(ranking DESC);
		CREATE INDEX IF NOT EXISTS idx_tracks_date_added ON tracks(date_added);

		CREATE INDEX IF NOT EXISTS idx_albums_artist_id ON albums(artist_id);
		CREATE INDEX IF NOT EXISTS idx_albums_genre_id ON albums(genre_id);

		CREATE INDEX IF NOT EXISTS idx_playlists_persistent_id ON playlists(persistent_id);
		CREATE INDEX IF NOT EXISTS idx_playlists_genre_id ON playlists(genre_id);

		CREATE INDEX IF NOT EXISTS idx_playlist_tracks_track_id ON playlist_tracks(track_id);
		CREATE INDEX IF NOT EXISTS idx_playlist_tracks_position ON playlist_tracks(playlist_id, position);

		-- Schema version tracking
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		INSERT INTO schema_migrations (version, description) VALUES (1, 'Initial schema with artists, genres, albums, tracks, playlists, and FTS5');
		`,
		Down: `
		DROP TABLE IF EXISTS playlist_tracks;
		DROP TABLE IF EXISTS playlists;
		DROP TABLE IF EXISTS tracks_fts;
		DROP TABLE IF EXISTS tracks;
		DROP TABLE IF EXISTS albums;
		DROP TABLE IF EXISTS genres;
		DROP TABLE IF EXISTS artists;
		DROP TABLE IF EXISTS schema_migrations;
		`,
	},
}

// InitSchema initializes the database schema
func InitSchema(db *sql.DB) error {
	// Set optimal SQLite pragmas for performance
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -64000", // 64MB cache
		"PRAGMA temp_store = MEMORY",
		"PRAGMA mmap_size = 268435456", // 256MB memory map
		"PRAGMA page_size = 4096",
		"PRAGMA foreign_keys = ON",
		"PRAGMA analysis_limit = 1000",
		"PRAGMA optimize",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to set pragma %s: %w", pragma, err)
		}
	}

	// Check current schema version
	var currentVersion int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		// Table doesn't exist yet
		currentVersion = 0
	}

	// Apply migrations
	for _, migration := range Schema {
		if migration.Version > currentVersion {
			log.Printf("Applying migration %d: %s", migration.Version, migration.Description)

			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}

			if _, err := tx.Exec(migration.Up); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
			}

			log.Printf("Successfully applied migration %d", migration.Version)
		}
	}

	// Run ANALYZE to update SQLite statistics
	if _, err := db.Exec("ANALYZE"); err != nil {
		log.Printf("Warning: failed to run ANALYZE: %v", err)
	}

	return nil
}

// GetSchemaVersion returns the current schema version
func GetSchemaVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// MigrateUp applies all pending migrations
func MigrateUp(db *sql.DB) error {
	return InitSchema(db)
}

// MigrateDown rolls back to a specific version
func MigrateDown(db *sql.DB, targetVersion int) error {
	currentVersion, err := GetSchemaVersion(db)
	if err != nil {
		return err
	}

	if targetVersion >= currentVersion {
		return fmt.Errorf("target version %d must be less than current version %d", targetVersion, currentVersion)
	}

	// Apply down migrations in reverse order
	for i := len(Schema) - 1; i >= 0; i-- {
		migration := Schema[i]
		if migration.Version > targetVersion && migration.Version <= currentVersion {
			log.Printf("Rolling back migration %d: %s", migration.Version, migration.Description)

			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}

			if _, err := tx.Exec(migration.Down); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to rollback migration %d: %w", migration.Version, err)
			}

			// Remove from schema_migrations
			if _, err := tx.Exec("DELETE FROM schema_migrations WHERE version = ?", migration.Version); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to update schema_migrations: %w", err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit rollback %d: %w", migration.Version, err)
			}

			log.Printf("Successfully rolled back migration %d", migration.Version)
		}
	}

	return nil
}
