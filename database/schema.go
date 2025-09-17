package database

import (
	"database/sql"
	"fmt"

	"go.uber.org/zap"
)

// SchemaVersion represents the current database schema version
const SchemaVersion = 6

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
	{
		Version:     2,
		Description: "Add streaming track support with detection fields",
		Up: `
		-- Add streaming detection columns to tracks table
		ALTER TABLE tracks ADD COLUMN is_streaming BOOLEAN DEFAULT FALSE;
		ALTER TABLE tracks ADD COLUMN track_kind VARCHAR(100);
		ALTER TABLE tracks ADD COLUMN stream_url VARCHAR(500);

		-- Create indexes for streaming queries
		CREATE INDEX IF NOT EXISTS idx_tracks_streaming ON tracks(is_streaming);
		CREATE INDEX IF NOT EXISTS idx_tracks_kind ON tracks(track_kind);

		-- Update schema version
		INSERT INTO schema_migrations (version, description) VALUES (2, 'Add streaming track support with detection fields');
		`,
		Down: `
		-- Remove streaming columns and indexes
		DROP INDEX IF EXISTS idx_tracks_kind;
		DROP INDEX IF EXISTS idx_tracks_streaming;

		-- Note: SQLite doesn't support DROP COLUMN, so we'd need to recreate the table
		-- This is a simplified down migration that warns about data loss
		CREATE TABLE tracks_backup AS SELECT
			id, persistent_id, name, artist_id, album_id, genre_id, collection,
			rating, starred, ranking, duration, play_count, last_played,
			date_added, created_at, updated_at
		FROM tracks;

		DROP TABLE tracks;

		CREATE TABLE tracks (
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

		INSERT INTO tracks SELECT * FROM tracks_backup;
		DROP TABLE tracks_backup;

		-- Recreate original indexes
		CREATE INDEX IF NOT EXISTS idx_tracks_persistent_id ON tracks(persistent_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_artist_id ON tracks(artist_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_album_id ON tracks(album_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_genre_id ON tracks(genre_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_collection ON tracks(collection);
		CREATE INDEX IF NOT EXISTS idx_tracks_starred ON tracks(starred);
		CREATE INDEX IF NOT EXISTS idx_tracks_rating ON tracks(rating);
		CREATE INDEX IF NOT EXISTS idx_tracks_ranking ON tracks(ranking DESC);
		CREATE INDEX IF NOT EXISTS idx_tracks_date_added ON tracks(date_added);

		-- Remove migration record
		DELETE FROM schema_migrations WHERE version = 2;
		`,
	},
	{
		Version:     3,
		Description: "Add radio stations table with genre integration",
		Up: `
		-- Radio stations table leveraging existing genres
		CREATE TABLE IF NOT EXISTS radio_stations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			description TEXT,
			genre_id INTEGER,
			country TEXT,
			language TEXT,
			quality TEXT, -- e.g., "128k AAC", "320k MP3"
			homepage TEXT,
			verified_at DATETIME,
			is_active BOOLEAN DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (genre_id) REFERENCES genres(id)
		);

		-- FTS5 virtual table for radio station search
		CREATE VIRTUAL TABLE IF NOT EXISTS radio_stations_fts USING fts5(
			name,
			description,
			genre_name,
			country,
			language,
			tokenize='unicode61 remove_diacritics 2'
		);

		-- Triggers to keep FTS5 table in sync (using LEFT JOIN for safety)
		CREATE TRIGGER IF NOT EXISTS radio_stations_fts_insert AFTER INSERT ON radio_stations
		BEGIN
			INSERT INTO radio_stations_fts(rowid, name, description, genre_name, country, language)
			SELECT
				NEW.id,
				NEW.name,
				COALESCE(NEW.description, ''),
				COALESCE(g.name, 'Unknown'),
				COALESCE(NEW.country, ''),
				COALESCE(NEW.language, '')
			FROM (SELECT 1) AS dummy
			LEFT JOIN genres g ON g.id = NEW.genre_id;
		END;

		CREATE TRIGGER IF NOT EXISTS radio_stations_fts_update AFTER UPDATE ON radio_stations
		BEGIN
			UPDATE radio_stations_fts
			SET name = NEW.name,
				description = COALESCE(NEW.description, ''),
				genre_name = COALESCE((SELECT name FROM genres WHERE id = NEW.genre_id), 'Unknown'),
				country = COALESCE(NEW.country, ''),
				language = COALESCE(NEW.language, '')
			WHERE rowid = NEW.id;
		END;

		CREATE TRIGGER IF NOT EXISTS radio_stations_fts_delete AFTER DELETE ON radio_stations
		BEGIN
			DELETE FROM radio_stations_fts WHERE rowid = OLD.id;
		END;

		-- Update timestamp trigger
		CREATE TRIGGER IF NOT EXISTS update_radio_stations_timestamp AFTER UPDATE ON radio_stations
		BEGIN
			UPDATE radio_stations SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;

		-- Indexes for performance
		CREATE INDEX IF NOT EXISTS idx_radio_stations_genre_id ON radio_stations(genre_id);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_country ON radio_stations(country);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_active ON radio_stations(is_active);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_verified ON radio_stations(verified_at);

		-- Update schema version
		INSERT INTO schema_migrations (version, description) VALUES (3, 'Add radio stations table with genre integration');
		`,
		Down: `
		DROP TRIGGER IF EXISTS update_radio_stations_timestamp;
		DROP TRIGGER IF EXISTS radio_stations_fts_delete;
		DROP TRIGGER IF EXISTS radio_stations_fts_update;
		DROP TRIGGER IF EXISTS radio_stations_fts_insert;
		DROP TABLE IF EXISTS radio_stations_fts;
		DROP INDEX IF EXISTS idx_radio_stations_verified;
		DROP INDEX IF EXISTS idx_radio_stations_active;
		DROP INDEX IF EXISTS idx_radio_stations_country;
		DROP INDEX IF EXISTS idx_radio_stations_genre_id;
		DROP TABLE IF EXISTS radio_stations;
		DELETE FROM schema_migrations WHERE version = 3;
		`,
	},
	{
		Version:     4,
		Description: "Clean up radio stations schema - remove superficial fields",
		Up: `
		-- Create new simplified radio_stations table
		CREATE TABLE IF NOT EXISTS radio_stations_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			description TEXT,
			genre_id INTEGER,
			homepage TEXT, -- https:// web URLs for browser access
			verified_at DATETIME,
			is_active BOOLEAN DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (genre_id) REFERENCES genres(id)
		);

		-- Copy data from old table, converting homepage URLs
		INSERT INTO radio_stations_new (
			id, name, url, description, genre_id, homepage,
			verified_at, is_active, created_at, updated_at
		)
		SELECT
			id, name, url, description, genre_id,
			-- Convert itmss:// homepage URLs back to https:// for web browser access
			CASE
				WHEN homepage LIKE 'itmss://music.apple.com%' THEN
					REPLACE(REPLACE(homepage, 'itmss://', 'https://'), '?app=music', '')
				ELSE homepage
			END,
			verified_at, is_active, created_at, updated_at
		FROM radio_stations;

		-- Drop old table and rename new one
		DROP TABLE radio_stations;
		ALTER TABLE radio_stations_new RENAME TO radio_stations;

		-- Recreate simplified FTS5 table
		DROP TABLE IF EXISTS radio_stations_fts;
		CREATE VIRTUAL TABLE IF NOT EXISTS radio_stations_fts USING fts5(
			name,
			description,
			genre_name,
			tokenize='unicode61 remove_diacritics 2'
		);

		-- Triggers to keep FTS5 table in sync (simplified)
		DROP TRIGGER IF EXISTS radio_stations_fts_insert;
		DROP TRIGGER IF EXISTS radio_stations_fts_update;
		DROP TRIGGER IF EXISTS radio_stations_fts_delete;

		CREATE TRIGGER IF NOT EXISTS radio_stations_fts_insert AFTER INSERT ON radio_stations
		BEGIN
			INSERT INTO radio_stations_fts(rowid, name, description, genre_name)
			SELECT
				NEW.id,
				NEW.name,
				COALESCE(NEW.description, ''),
				(SELECT name FROM genres WHERE id = NEW.genre_id)
			;
		END;

		CREATE TRIGGER IF NOT EXISTS radio_stations_fts_update AFTER UPDATE ON radio_stations
		BEGIN
			UPDATE radio_stations_fts
			SET name = NEW.name,
				description = COALESCE(NEW.description, ''),
				genre_name = COALESCE((SELECT name FROM genres WHERE id = NEW.genre_id), 'Unknown')
			WHERE rowid = NEW.id;
		END;

		CREATE TRIGGER IF NOT EXISTS radio_stations_fts_delete AFTER DELETE ON radio_stations
		BEGIN
			DELETE FROM radio_stations_fts WHERE rowid = OLD.id;
		END;

		-- Update timestamp trigger
		CREATE TRIGGER IF NOT EXISTS update_radio_stations_timestamp AFTER UPDATE ON radio_stations
		BEGIN
			UPDATE radio_stations SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;

		-- Simplified indexes for performance
		CREATE INDEX IF NOT EXISTS idx_radio_stations_genre_id ON radio_stations(genre_id);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_active ON radio_stations(is_active);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_verified ON radio_stations(verified_at);

		-- Repopulate FTS5 table with existing data
		INSERT INTO radio_stations_fts(rowid, name, description, genre_name)
		SELECT
			rs.id,
			rs.name,
			COALESCE(rs.description, ''),
			COALESCE(g.name, 'Unknown')
		FROM radio_stations rs
		LEFT JOIN genres g ON g.id = rs.genre_id;

		-- Update schema version
		INSERT INTO schema_migrations (version, description) VALUES (4, 'Clean up radio stations schema - remove superficial fields');
		`,
		Down: `
		-- Recreate old table structure
		CREATE TABLE IF NOT EXISTS radio_stations_old (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			description TEXT,
			genre_id INTEGER,
			country TEXT,
			language TEXT,
			quality TEXT,
			homepage TEXT,
			verified_at DATETIME,
			is_active BOOLEAN DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (genre_id) REFERENCES genres(id)
		);

		-- Copy data back with default values for removed fields
		INSERT INTO radio_stations_old (
			id, name, url, description, genre_id,
			country, language, quality, homepage,
			verified_at, is_active, created_at, updated_at
		)
		SELECT
			id, name, url, description, genre_id,
			'US', 'English', '256k AAC',
			-- Convert https:// homepage URLs back to itmss://
			CASE
				WHEN homepage LIKE 'https://music.apple.com%' THEN
					REPLACE(homepage, 'https://', 'itmss://') || '?app=music'
				ELSE homepage
			END,
			verified_at, is_active, created_at, updated_at
		FROM radio_stations;

		-- Drop new table and rename old one
		DROP TABLE radio_stations;
		ALTER TABLE radio_stations_old RENAME TO radio_stations;

		-- Recreate old FTS5 structure and triggers (simplified for rollback)
		DROP TABLE IF EXISTS radio_stations_fts;
		CREATE VIRTUAL TABLE IF NOT EXISTS radio_stations_fts USING fts5(
			name, description, genre_name, country, language,
			tokenize='unicode61 remove_diacritics 2'
		);

		DELETE FROM schema_migrations WHERE version = 4;
		`,
	},
	{
		Version:     5,
		Description: "Make genre_id in radio_stations not-nullable",
		Up: `
		-- Ensure 'Unknown' genre exists and get its ID
		INSERT OR IGNORE INTO genres (name) VALUES ('Unknown');

		-- Update existing NULL genre_id to point to 'Unknown' genre
		UPDATE radio_stations
		SET genre_id = (SELECT id FROM genres WHERE name = 'Unknown')
		WHERE genre_id IS NULL;

		-- Recreate table with NOT NULL constraint
		CREATE TABLE radio_stations_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			description TEXT,
			genre_id INTEGER NOT NULL, -- Changed to NOT NULL
			homepage TEXT,
			verified_at DATETIME,
			is_active BOOLEAN DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (genre_id) REFERENCES genres(id)
		);

		INSERT INTO radio_stations_new (
			id, name, url, description, genre_id, homepage,
			verified_at, is_active, created_at, updated_at
		)
		SELECT
			id, name, url, description, genre_id, homepage,
			verified_at, is_active, created_at, updated_at
		FROM radio_stations;

		DROP TABLE radio_stations;
		ALTER TABLE radio_stations_new RENAME TO radio_stations;

		-- Recreate indexes
		CREATE INDEX IF NOT EXISTS idx_radio_stations_genre_id ON radio_stations(genre_id);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_active ON radio_stations(is_active);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_verified ON radio_stations(verified_at);

		-- Repopulate FTS5 table
		INSERT INTO radio_stations_fts (rowid, name, description, genre_name)
		SELECT rs.id, rs.name, rs.description, g.name
		FROM radio_stations rs
		JOIN genres g ON rs.genre_id = g.id;

		-- Update schema version
		INSERT INTO schema_migrations (version, description) VALUES (5, 'Make genre_id in radio_stations not-nullable');
		`,
		Down: `
		-- Recreate table without NOT NULL constraint
		CREATE TABLE radio_stations_old (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			description TEXT,
			genre_id INTEGER, -- Reverted: NULL allowed
			homepage TEXT,
			verified_at DATETIME,
			is_active BOOLEAN DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (genre_id) REFERENCES genres(id)
		);

		INSERT INTO radio_stations_old (
			id, name, url, description, genre_id, homepage,
			verified_at, is_active, created_at, updated_at
		)
		SELECT
			id, name, url, description, genre_id, homepage,
			verified_at, is_active, created_at, updated_at
		FROM radio_stations;

		DROP TABLE radio_stations;
		ALTER TABLE radio_stations_old RENAME TO radio_stations;

		-- Recreate indexes
		CREATE INDEX IF NOT EXISTS idx_radio_stations_genre_id ON radio_stations(genre_id);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_active ON radio_stations(is_active);
		CREATE INDEX IF NOT EXISTS idx_radio_stations_verified ON radio_stations(verified_at);

		DELETE FROM schema_migrations WHERE version = 5;
		`,
	},
	{
		Version:     6,
		Description: "Rebuild and repopulate FTS index for radio stations",
		Up: `
		-- Drop existing FTS table and triggers
		DROP TRIGGER IF EXISTS radio_stations_fts_insert;
		DROP TRIGGER IF EXISTS radio_stations_fts_update;
		DROP TRIGGER IF EXISTS radio_stations_fts_delete;
		DROP TABLE IF EXISTS radio_stations_fts;

		-- Recreate FTS5 virtual table for radio station search
		CREATE VIRTUAL TABLE radio_stations_fts USING fts5(
			name,
			description,
			genre_name,
			tokenize='unicode61 remove_diacritics 2'
		);

		-- Triggers to keep FTS5 table in sync
		CREATE TRIGGER radio_stations_fts_insert AFTER INSERT ON radio_stations
		BEGIN
			INSERT INTO radio_stations_fts(rowid, name, description, genre_name)
			SELECT
				NEW.id,
				NEW.name,
				COALESCE(NEW.description, ''),
				(SELECT name FROM genres WHERE id = NEW.genre_id)
			;
		END;

		CREATE TRIGGER radio_stations_fts_update AFTER UPDATE ON radio_stations
		BEGIN
			UPDATE radio_stations_fts
			SET name = NEW.name,
				description = COALESCE(NEW.description, ''),
				genre_name = (SELECT name FROM genres WHERE id = NEW.genre_id)
			WHERE rowid = NEW.id;
		END;

		CREATE TRIGGER radio_stations_fts_delete AFTER DELETE ON radio_stations
		BEGIN
			DELETE FROM radio_stations_fts WHERE rowid = OLD.id;
		END;

		-- Repopulate FTS5 table with all existing data
		INSERT INTO radio_stations_fts(rowid, name, description, genre_name)
		SELECT
			rs.id,
			rs.name,
			COALESCE(rs.description, ''),
			g.name
		FROM radio_stations rs
		JOIN genres g ON g.id = rs.genre_id;

		-- Update schema version
		INSERT INTO schema_migrations (version, description) VALUES (6, 'Rebuild and repopulate FTS index for radio stations');
		`,
		Down: `
		-- Drop new FTS table and triggers
		DROP TRIGGER IF EXISTS radio_stations_fts_insert;
		DROP TRIGGER IF EXISTS radio_stations_fts_update;
		DROP TRIGGER IF EXISTS radio_stations_fts_delete;
		DROP TABLE IF EXISTS radio_stations_fts;

		-- Recreate old FTS5 structure (from migration 4)
		CREATE VIRTUAL TABLE radio_stations_fts USING fts5(
			name,
			description,
			genre_name,
			tokenize='unicode61 remove_diacritics 2'
		);

		-- Recreate old triggers
		CREATE TRIGGER radio_stations_fts_insert AFTER INSERT ON radio_stations
		BEGIN
			INSERT INTO radio_stations_fts(rowid, name, description, genre_name)
			SELECT
				NEW.id,
				NEW.name,
				COALESCE(NEW.description, ''),
				COALESCE(g.name, 'Unknown')
			FROM (SELECT 1) AS dummy
			LEFT JOIN genres g ON g.id = NEW.genre_id;
		END;

		DELETE FROM schema_migrations WHERE version = 6;
		`,
	},
}

// InitSchema initializes the database schema
func InitSchema(db *sql.DB, logger *zap.Logger) error {
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
			logger.Info("Applying migration", zap.Int("version", migration.Version), zap.String("description", migration.Description))

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

			logger.Info("Successfully applied migration", zap.Int("version", migration.Version))
		}
	}

	// Run ANALYZE to update SQLite statistics
	if _, err := db.Exec("ANALYZE"); err != nil {
		logger.Warn("Failed to run ANALYZE", zap.Error(err))
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
func MigrateUp(db *sql.DB, logger *zap.Logger) error {
	return InitSchema(db, logger)
}

// MigrateDown rolls back to a specific version
func MigrateDown(db *sql.DB, targetVersion int, logger *zap.Logger) error {
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
			logger.Info("Rolling back migration", zap.Int("version", migration.Version), zap.String("description", migration.Description))

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

			logger.Info("Successfully rolled back migration", zap.Int("version", migration.Version))
		}
	}

	return nil
}
