package itunes

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"itunes/database"
)

func TestDatabaseIntegration(t *testing.T) {
	// Setup test database
	testDBPath := filepath.Join(os.TempDir(), "itunes_test.db")
	defer os.Remove(testDBPath)

	// Override the default database path for testing
	originalPath := database.PrimaryDBPath
	database.PrimaryDBPath = testDBPath
	defer func() { database.PrimaryDBPath = originalPath }()

	// Initialize database
	err := InitDatabase()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer CloseDatabase()

	// Create test data
	testTracks := []database.Track{
		{
			PersistentID: "TEST001",
			Name:         "Test Song 1",
			Album:        "Test Album",
			Artist:       "Test Artist",
			Collection:   "Test Playlist",
			Genre:        "Rock",
			Rating:       80,
			Starred:      true,
			Playlists:    []string{"Test Playlist", "Favorites"},
		},
		{
			PersistentID: "TEST002",
			Name:         "Jazz Track",
			Album:        "Jazz Album",
			Artist:       "Jazz Artist",
			Collection:   "Jazz Collection",
			Genre:        "Jazz",
			Rating:       60,
			Starred:      false,
			Playlists:    []string{"Jazz Collection"},
		},
		{
			PersistentID: "TEST003",
			Name:         "Another Test Song",
			Album:        "Test Album",
			Artist:       "Test Artist",
			Collection:   "Test Playlist",
			Genre:        "Rock",
			Rating:       100,
			Starred:      true,
			Playlists:    []string{"Test Playlist"},
		},
	}

	// Insert test data
	err = dbManager.BatchInsertTracks(testTracks)
	if err != nil {
		t.Fatalf("Failed to insert test tracks: %v", err)
	}

	// Test basic search
	t.Run("BasicSearch", func(t *testing.T) {
		results, err := SearchTracks("test")
		if err != nil {
			t.Errorf("Search failed: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}
	})

	// Test search with filters
	t.Run("FilteredSearch", func(t *testing.T) {
		filters := &database.SearchFilters{
			Genre: "Jazz",
			Limit: 10,
		}
		results, err := SearchTracksFromDatabase("", filters)
		if err != nil {
			t.Errorf("Filtered search failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 jazz result, got %d", len(results))
		}
		if results[0].Genre != "Jazz" {
			t.Errorf("Expected Jazz genre, got %s", results[0].Genre)
		}
	})

	// Test starred filter
	t.Run("StarredFilter", func(t *testing.T) {
		starred := true
		filters := &database.SearchFilters{
			Starred: &starred,
			Limit:   10,
		}
		results, err := SearchTracksFromDatabase("", filters)
		if err != nil {
			t.Errorf("Starred filter search failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 starred results, got %d", len(results))
		}
	})

	// Test rating filter
	t.Run("RatingFilter", func(t *testing.T) {
		filters := &database.SearchFilters{
			MinRating: 70,
			Limit:     10,
		}
		results, err := SearchTracksFromDatabase("", filters)
		if err != nil {
			t.Errorf("Rating filter search failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 high-rated results, got %d", len(results))
		}
	})

	// Test playlist filter
	t.Run("PlaylistFilter", func(t *testing.T) {
		filters := &database.SearchFilters{
			Playlist: "Test Playlist",
			Limit:    10,
		}
		results, err := SearchTracksFromDatabase("", filters)
		if err != nil {
			t.Errorf("Playlist filter search failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 tracks in Test Playlist, got %d", len(results))
		}
	})

	// Test GetTrackByPersistentID
	t.Run("GetTrackByPersistentID", func(t *testing.T) {
		track, err := GetTrackByPersistentID("TEST001")
		if err != nil {
			t.Errorf("GetTrackByPersistentID failed: %v", err)
		}
		if track.Name != "Test Song 1" {
			t.Errorf("Expected 'Test Song 1', got '%s'", track.Name)
		}
		if len(track.Playlists) != 2 {
			t.Errorf("Expected 2 playlists, got %d", len(track.Playlists))
		}
	})

	// Test GetPlaylistTracks
	t.Run("GetPlaylistTracks", func(t *testing.T) {
		// First create the playlist
		err := dbManager.UpsertPlaylist(&database.Playlist{
			PersistentID: "PL_TEST",
			Name:         "Test Playlist",
			Genre:        "",
			SpecialKind:  "none",
		})
		if err != nil {
			t.Fatalf("Failed to create playlist: %v", err)
		}

		tracks, err := GetPlaylistTracks("Test Playlist", false)
		if err != nil {
			t.Errorf("GetPlaylistTracks failed: %v", err)
		}
		if len(tracks) != 2 {
			t.Errorf("Expected 2 tracks in playlist, got %d", len(tracks))
		}
	})

	// Test search limit configuration
	t.Run("SearchLimitConfig", func(t *testing.T) {
		originalLimit := SearchLimit
		SearchLimit = 1
		defer func() { SearchLimit = originalLimit }()

		results, err := SearchTracks("test")
		if err != nil {
			t.Errorf("Limited search failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result with limit, got %d", len(results))
		}
	})

	// Test database stats
	t.Run("DatabaseStats", func(t *testing.T) {
		stats, err := GetDatabaseStats()
		if err != nil {
			t.Errorf("GetDatabaseStats failed: %v", err)
		}
		if stats.TrackCount != 3 {
			t.Errorf("Expected 3 tracks, got %d", stats.TrackCount)
		}
		if stats.ArtistCount < 2 {
			t.Errorf("Expected at least 2 artists, got %d", stats.ArtistCount)
		}
		if stats.AlbumCount < 2 {
			t.Errorf("Expected at least 2 albums, got %d", stats.AlbumCount)
		}
	})
}

func TestDatabasePerformance(t *testing.T) {
	// Setup test database
	testDBPath := filepath.Join(os.TempDir(), "itunes_perf_test.db")
	defer os.Remove(testDBPath)

	// Override the default database path for testing
	originalPath := database.PrimaryDBPath
	database.PrimaryDBPath = testDBPath
	defer func() { database.PrimaryDBPath = originalPath }()

	// Initialize database
	err := InitDatabase()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer CloseDatabase()

	// Create larger test dataset
	var testTracks []database.Track
	for i := 0; i < 1000; i++ {
		track := database.Track{
			PersistentID: fmt.Sprintf("PERF%06d", i),
			Name:         fmt.Sprintf("Track %d", i),
			Album:        fmt.Sprintf("Album %d", i/10),
			Artist:       fmt.Sprintf("Artist %d", i/20),
			Collection:   fmt.Sprintf("Playlist %d", i/50),
			Genre:        []string{"Rock", "Jazz", "Classical", "Electronic"}[i%4],
			Rating:       (i % 5) * 20,
			Starred:      i%3 == 0,
			Playlists:    []string{fmt.Sprintf("Playlist %d", i/50)},
		}
		testTracks = append(testTracks, track)
	}

	// Insert test data
	start := time.Now()
	err = dbManager.BatchInsertTracks(testTracks)
	insertDuration := time.Since(start)
	if err != nil {
		t.Fatalf("Failed to insert test tracks: %v", err)
	}

	t.Logf("Inserted 1000 tracks in %v (%.2f tracks/sec)", insertDuration, 1000/insertDuration.Seconds())

	// Benchmark search performance
	t.Run("SearchPerformance", func(t *testing.T) {
		queries := []string{"Track", "Album", "Artist", "1", "50", "Jazz", "Rock"}

		for _, query := range queries {
			start := time.Now()
			results, err := SearchTracks(query)
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Search for '%s' failed: %v", query, err)
				continue
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Search for '%s' took %v, exceeding 10ms target", query, duration)
			}

			t.Logf("Search for '%s': %d results in %v", query, len(results), duration)
		}
	})

	// Benchmark filtered search performance
	t.Run("FilteredSearchPerformance", func(t *testing.T) {
		filters := []database.SearchFilters{
			{Genre: "Jazz", Limit: 15},
			{MinRating: 60, Limit: 15},
			{Artist: "Artist 10", Limit: 15},
			{Playlist: "Playlist 5", Limit: 15},
		}

		for i, filter := range filters {
			start := time.Now()
			results, err := SearchTracksFromDatabase("", &filter)
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Filtered search %d failed: %v", i, err)
				continue
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Filtered search %d took %v, exceeding 10ms target", i, duration)
			}

			t.Logf("Filtered search %d: %d results in %v", i, len(results), duration)
		}
	})

	// Test cache effectiveness
	t.Run("CachePerformance", func(t *testing.T) {
		query := "Track 100"

		// First search (cache miss)
		start := time.Now()
		results1, err := SearchTracks(query)
		duration1 := time.Since(start)
		if err != nil {
			t.Errorf("First search failed: %v", err)
		}

		// Second search (cache hit)
		start = time.Now()
		results2, err := SearchTracks(query)
		duration2 := time.Since(start)
		if err != nil {
			t.Errorf("Second search failed: %v", err)
		}

		if len(results1) != len(results2) {
			t.Errorf("Cache returned different results: %d vs %d", len(results1), len(results2))
		}

		// Cache should be significantly faster
		if duration2 > duration1/2 {
			t.Logf("Warning: Cache not significantly faster - First: %v, Second: %v", duration1, duration2)
		} else {
			t.Logf("Cache performance: First search: %v, Cached: %v (%.1fx speedup)",
				duration1, duration2, float64(duration1)/float64(duration2))
		}
	})
}

func TestDatabaseFallback(t *testing.T) {
	// Test behavior when database is not available
	t.Run("DatabaseNotInitialized", func(t *testing.T) {
		// Make sure database is not initialized
		CloseDatabase()

		// Try to search - should fail gracefully
		_, err := SearchTracksFromDatabase("test", nil)
		if err == nil {
			t.Error("Expected error when database not initialized")
		}
		if err.Error() != "database not initialized" {
			t.Errorf("Expected 'database not initialized' error, got: %v", err)
		}
	})
}

func TestAPICompatibility(t *testing.T) {
	// Setup test database
	testDBPath := filepath.Join(os.TempDir(), "itunes_api_test.db")
	defer os.Remove(testDBPath)

	// Override the default database path for testing
	originalPath := database.PrimaryDBPath
	database.PrimaryDBPath = testDBPath
	defer func() { database.PrimaryDBPath = originalPath }()

	// Initialize database
	err := InitDatabase()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer CloseDatabase()

	// Create test data that matches expected API format
	testTrack := database.Track{
		PersistentID: "API_TEST_001",
		Name:         "API Test Song",
		Album:        "API Test Album",
		Artist:       "API Test Artist",
		Collection:   "API Test Collection",
		Genre:        "Test Genre",
		Rating:       85,
		Starred:      true,
		Playlists:    []string{"Playlist 1", "Playlist 2"},
	}

	err = dbManager.BatchInsertTracks([]database.Track{testTrack})
	if err != nil {
		t.Fatalf("Failed to insert test track: %v", err)
	}

	// Test that API returns expected format
	t.Run("APIFormat", func(t *testing.T) {
		results, err := SearchTracks("API Test")
		if err != nil {
			t.Errorf("Search failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}

		track := results[0]

		// Verify all fields are properly populated
		if track.ID != "API_TEST_001" {
			t.Errorf("Expected ID 'API_TEST_001', got '%s'", track.ID)
		}
		if track.PersistentID != "API_TEST_001" {
			t.Errorf("Expected PersistentID 'API_TEST_001', got '%s'", track.PersistentID)
		}
		if track.Name != "API Test Song" {
			t.Errorf("Expected Name 'API Test Song', got '%s'", track.Name)
		}
		if track.Album != "API Test Album" {
			t.Errorf("Expected Album 'API Test Album', got '%s'", track.Album)
		}
		if track.Artist != "API Test Artist" {
			t.Errorf("Expected Artist 'API Test Artist', got '%s'", track.Artist)
		}
		if track.Collection != "API Test Collection" {
			t.Errorf("Expected Collection 'API Test Collection', got '%s'", track.Collection)
		}
		if track.Genre != "Test Genre" {
			t.Errorf("Expected Genre 'Test Genre', got '%s'", track.Genre)
		}
		if track.Rating != 85 {
			t.Errorf("Expected Rating 85, got %d", track.Rating)
		}
		if !track.Starred {
			t.Error("Expected Starred to be true")
		}
		if len(track.Playlists) != 2 {
			t.Errorf("Expected 2 playlists, got %d", len(track.Playlists))
		}
	})
}
