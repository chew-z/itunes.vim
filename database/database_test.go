package database

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDatabaseManager tests database initialization and schema creation
func TestDatabaseManager(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database manager
	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer dm.Close()

	// Verify schema version
	version, err := GetSchemaVersion(dm.DB)
	if err != nil {
		t.Fatalf("Failed to get schema version: %v", err)
	}
	if version != SchemaVersion {
		t.Errorf("Expected schema version %d, got %d", SchemaVersion, version)
	}

	// Test stats on empty database
	stats, err := dm.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	if stats.TrackCount != 0 {
		t.Errorf("Expected 0 tracks, got %d", stats.TrackCount)
	}
}

// TestSchemaIdempotency tests that schema creation is idempotent
func TestSchemaIdempotency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database and initialize schema multiple times
	for i := 0; i < 3; i++ {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}

		err = InitSchema(db)
		if err != nil {
			t.Fatalf("Failed to initialize schema on iteration %d: %v", i, err)
		}

		version, err := GetSchemaVersion(db)
		if err != nil {
			t.Fatalf("Failed to get schema version: %v", err)
		}
		if version != SchemaVersion {
			t.Errorf("Expected schema version %d, got %d", SchemaVersion, version)
		}

		db.Close()
	}
}

// TestBasicCRUDOperations tests basic insert and select operations
func TestBasicCRUDOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer dm.Close()

	// Insert a track
	track := &Track{
		PersistentID: "TEST123456789",
		Name:         "Test Track",
		Artist:       "Test Artist",
		Album:        "Test Album",
		Genre:        "Test Genre",
		Collection:   "Test Collection",
		Rating:       80,
		Starred:      true,
		Ranking:      0.95,
		Duration:     240,
		PlayCount:    10,
		LastPlayed:   nil,
		DateAdded:    nil,
	}

	err = dm.InsertTrack(track)
	if err != nil {
		t.Fatalf("Failed to insert track: %v", err)
	}
	if track.ID == 0 {
		t.Error("Expected track ID to be set after insert")
	}

	// Retrieve track by persistent ID
	retrieved, err := dm.GetTrackByPersistentID(track.PersistentID)
	if err != nil {
		t.Fatalf("Failed to retrieve track: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected to retrieve track, got nil")
	}

	// Verify fields
	if retrieved.Name != track.Name {
		t.Errorf("Expected name %s, got %s", track.Name, retrieved.Name)
	}
	if retrieved.Artist != track.Artist {
		t.Errorf("Expected artist %s, got %s", track.Artist, retrieved.Artist)
	}
	if retrieved.Album != track.Album {
		t.Errorf("Expected album %s, got %s", track.Album, retrieved.Album)
	}
	if retrieved.Genre != track.Genre {
		t.Errorf("Expected genre %s, got %s", track.Genre, retrieved.Genre)
	}
	if retrieved.Rating != track.Rating {
		t.Errorf("Expected rating %d, got %d", track.Rating, retrieved.Rating)
	}
	if retrieved.Starred != track.Starred {
		t.Errorf("Expected starred %v, got %v", track.Starred, retrieved.Starred)
	}

	// Test search
	results, err := dm.SearchTracks("Test", nil)
	if err != nil {
		t.Fatalf("Failed to search tracks: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(results))
	}

	// Test FTS search
	ftsResults, err := dm.SearchTracksWithFTS("Test", nil)
	if err != nil {
		t.Fatalf("Failed to search tracks with FTS: %v", err)
	}
	if len(ftsResults) != 1 {
		t.Errorf("Expected 1 FTS search result, got %d", len(ftsResults))
	}

	// Verify stats
	stats, err := dm.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	if stats.TrackCount != 1 {
		t.Errorf("Expected 1 track, got %d", stats.TrackCount)
	}
	if stats.ArtistCount != 1 {
		t.Errorf("Expected 1 artist, got %d", stats.ArtistCount)
	}
	if stats.AlbumCount != 1 {
		t.Errorf("Expected 1 album, got %d", stats.AlbumCount)
	}
	if stats.GenreCount != 1 {
		t.Errorf("Expected 1 genre, got %d", stats.GenreCount)
	}
}

// TestSearchFilters tests search with various filters
func TestSearchFilters(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer dm.Close()

	// Insert test tracks
	tracks := []Track{
		{PersistentID: "ID1", Name: "Jazz Track 1", Artist: "Jazz Artist", Album: "Jazz Album", Genre: "Jazz", Rating: 100, Starred: true},
		{PersistentID: "ID2", Name: "Jazz Track 2", Artist: "Jazz Artist", Album: "Jazz Album", Genre: "Jazz", Rating: 80, Starred: false},
		{PersistentID: "ID3", Name: "Rock Track", Artist: "Rock Artist", Album: "Rock Album", Genre: "Rock", Rating: 60, Starred: true},
		{PersistentID: "ID4", Name: "Pop Track", Artist: "Pop Artist", Album: "Pop Album", Genre: "Pop", Rating: 40, Starred: false},
	}

	for i := range tracks {
		if err := dm.InsertTrack(&tracks[i]); err != nil {
			t.Fatalf("Failed to insert track %d: %v", i, err)
		}
	}

	tests := []struct {
		name     string
		query    string
		filters  SearchFilters
		expected int
	}{
		{"Genre filter", "", SearchFilters{Genre: "Jazz"}, 2},
		{"Artist filter", "", SearchFilters{Artist: "Jazz Artist"}, 2},
		{"Album filter", "", SearchFilters{Album: "Rock Album"}, 1},
		{"Starred filter", "", SearchFilters{Starred: boolPtr(true)}, 2},
		{"Rating filter", "", SearchFilters{MinRating: 80}, 2},
		{"Combined filters", "", SearchFilters{Genre: "Jazz", MinRating: 90}, 1},
		{"Query with filter", "Track", SearchFilters{Genre: "Rock"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := dm.SearchTracks(tt.query, &tt.filters)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}
			if len(results) != tt.expected {
				t.Errorf("Expected %d results, got %d", tt.expected, len(results))
			}
		})
	}
}

// BenchmarkInsertTracks benchmarks track insertion performance
func BenchmarkInsertTracks(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database manager: %v", err)
	}
	defer dm.Close()

	// Prepare test data
	artists := []string{"Artist A", "Artist B", "Artist C", "Artist D", "Artist E"}
	albums := []string{"Album 1", "Album 2", "Album 3", "Album 4", "Album 5"}
	genres := []string{"Rock", "Jazz", "Pop", "Classical", "Electronic"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		track := &Track{
			PersistentID: fmt.Sprintf("BENCH%d", i),
			Name:         fmt.Sprintf("Track %d", i),
			Artist:       artists[i%len(artists)],
			Album:        albums[i%len(albums)],
			Genre:        genres[i%len(genres)],
			Rating:       rand.Intn(101),
			Starred:      i%2 == 0,
			Ranking:      rand.Float64(),
			Duration:     180 + rand.Intn(240),
			PlayCount:    rand.Intn(100),
		}

		if err := dm.InsertTrack(track); err != nil {
			b.Fatalf("Failed to insert track: %v", err)
		}
	}
}

// BenchmarkSearchPerformance benchmarks search performance with 1000+ tracks
func BenchmarkSearchPerformance(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database manager: %v", err)
	}
	defer dm.Close()

	// Insert 1000 tracks
	artists := []string{"Jazz Artist", "Rock Artist", "Pop Artist", "Classical Artist", "Electronic Artist"}
	albums := []string{"Greatest Hits", "Live Album", "Studio Sessions", "Compilation", "B-Sides"}
	genres := []string{"Jazz", "Rock", "Pop", "Classical", "Electronic"}
	words := []string{"Love", "Night", "Dream", "Dance", "Heart", "Soul", "Fire", "Rain", "Sun", "Moon"}

	for i := 0; i < 1000; i++ {
		track := &Track{
			PersistentID: fmt.Sprintf("PERF%d", i),
			Name:         fmt.Sprintf("%s %s %d", words[i%len(words)], words[(i+1)%len(words)], i),
			Artist:       artists[i%len(artists)],
			Album:        albums[i%len(albums)],
			Genre:        genres[i%len(genres)],
			Rating:       rand.Intn(101),
			Starred:      i%3 == 0,
			Ranking:      rand.Float64(),
			Duration:     180 + rand.Intn(240),
			PlayCount:    rand.Intn(100),
		}

		if err := dm.InsertTrack(track); err != nil {
			b.Fatalf("Failed to insert track: %v", err)
		}
	}

	// Run ANALYZE to update statistics
	if _, err := dm.DB.Exec("ANALYZE"); err != nil {
		b.Logf("Warning: failed to run ANALYZE: %v", err)
	}

	b.ResetTimer()

	// Benchmark regular search
	b.Run("RegularSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			query := words[i%len(words)]
			_, err := dm.SearchTracks(query, &SearchFilters{Limit: 15})
			if err != nil {
				b.Fatalf("Search failed: %v", err)
			}
		}
	})

	// Benchmark FTS search
	b.Run("FTSSearch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			query := words[i%len(words)]
			_, err := dm.SearchTracksWithFTS(query, &SearchFilters{Limit: 15})
			if err != nil {
				b.Fatalf("FTS search failed: %v", err)
			}
		}
	})
}

// TestFTSSearchPerformance validates that FTS search returns results in <10ms
func TestFTSSearchPerformance(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "perf.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer dm.Close()

	// Insert 5000 tracks for realistic performance testing
	artists := []string{"Jazz Artist", "Rock Artist", "Pop Artist", "Classical Artist", "Electronic Artist",
		"Folk Artist", "Blues Artist", "Country Artist", "Hip Hop Artist", "Metal Artist"}
	albums := []string{"Greatest Hits", "Live Album", "Studio Sessions", "Compilation", "B-Sides",
		"Acoustic Sessions", "Remixes", "Deluxe Edition", "EP", "Singles"}
	genres := []string{"Jazz", "Rock", "Pop", "Classical", "Electronic", "Folk", "Blues", "Country", "Hip Hop", "Metal"}
	words := []string{"Love", "Night", "Dream", "Dance", "Heart", "Soul", "Fire", "Rain", "Sun", "Moon",
		"Star", "Light", "Dark", "Time", "Life", "Hope", "Peace", "War", "Joy", "Pain"}

	for i := 0; i < 5000; i++ {
		track := &Track{
			PersistentID: fmt.Sprintf("PERF%d", i),
			Name:         fmt.Sprintf("%s %s %s", words[i%len(words)], words[(i+1)%len(words)], words[(i+2)%len(words)]),
			Artist:       artists[i%len(artists)],
			Album:        albums[i%len(albums)],
			Genre:        genres[i%len(genres)],
			Rating:       rand.Intn(101),
			Starred:      i%3 == 0,
			Ranking:      rand.Float64(),
			Duration:     180 + rand.Intn(240),
			PlayCount:    rand.Intn(100),
		}

		if err := dm.InsertTrack(track); err != nil {
			t.Fatalf("Failed to insert track: %v", err)
		}
	}

	// Run ANALYZE to update statistics
	if _, err := dm.DB.Exec("ANALYZE"); err != nil {
		t.Logf("Warning: failed to run ANALYZE: %v", err)
	}

	// Test various search queries
	queries := []string{"Love", "Night Dream", "Jazz", "Rock Soul", "Dance Fire"}

	for _, query := range queries {
		start := time.Now()
		results, err := dm.SearchTracksWithFTS(query, &SearchFilters{Limit: 15})
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("FTS search failed for query '%s': %v", query, err)
		}

		if len(results) == 0 {
			t.Logf("No results found for query '%s'", query)
		}

		// Validate performance - warn at 5ms, fail at 10ms
		if duration > 10*time.Millisecond {
			t.Errorf("FTS search for '%s' took %v, expected <10ms", query, duration)
		} else if duration > 5*time.Millisecond {
			t.Logf("WARNING: FTS search for '%s' took %v (>5ms) with %d results", query, duration, len(results))
		} else {
			t.Logf("FTS search for '%s' completed in %v with %d results", query, duration, len(results))
		}
	}

	// Test filter performance
	filters := []SearchFilters{
		{Genre: "Jazz", Limit: 15},
		{Artist: "Rock Artist", Limit: 15},
		{Starred: boolPtr(true), Limit: 15},
		{MinRating: 80, Limit: 15},
	}

	for i, filter := range filters {
		start := time.Now()
		results, err := dm.SearchTracksWithFTS("", &filter)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Filtered search %d failed: %v", i, err)
		}

		if duration > 10*time.Millisecond {
			t.Errorf("Filtered search %d took %v, expected <10ms", i, duration)
		} else if duration > 5*time.Millisecond {
			t.Logf("WARNING: Filtered search %d took %v (>5ms) with %d results", i, duration, len(results))
		} else {
			t.Logf("Filtered search %d completed in %v with %d results", i, duration, len(results))
		}
	}
}

// TestVacuum tests database vacuum operation
func TestVacuum(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer dm.Close()

	// Insert and delete some data to create fragmentation
	for i := 0; i < 100; i++ {
		track := &Track{
			PersistentID: fmt.Sprintf("VACUUM%d", i),
			Name:         fmt.Sprintf("Track %d", i),
			Artist:       "Test Artist",
			Album:        "Test Album",
			Genre:        "Test Genre",
		}
		if err := dm.InsertTrack(track); err != nil {
			t.Fatalf("Failed to insert track: %v", err)
		}
	}

	// Get initial count
	var initialCount int64
	if err := dm.DB.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&initialCount); err != nil {
		t.Fatalf("Failed to get initial count: %v", err)
	}

	// Delete some tracks using persistent IDs to avoid FTS issues
	if _, err := dm.DB.Exec("DELETE FROM tracks WHERE persistent_id LIKE 'VACUUM%' AND CAST(SUBSTR(persistent_id, 7) AS INTEGER) % 2 = 0"); err != nil {
		t.Fatalf("Failed to delete tracks: %v", err)
	}

	// Run vacuum
	if err := dm.Vacuum(); err != nil {
		t.Fatalf("Failed to vacuum database: %v", err)
	}

	// Verify database still works
	stats, err := dm.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats after vacuum: %v", err)
	}
	if stats.TrackCount != 50 {
		t.Errorf("Expected 50 tracks after vacuum, got %d", stats.TrackCount)
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

// TestMain sets up and tears down test environment
func TestMain(m *testing.M) {
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Run tests
	code := m.Run()

	os.Exit(code)
}
