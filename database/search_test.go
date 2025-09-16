package database

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSearchQueryBuilder(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		filters    *SearchFilters
		useFTS     bool
		wantError  bool
		checkQuery func(query string, args []interface{}) bool
	}{
		{
			name:   "Simple FTS query",
			query:  "jazz",
			useFTS: true,
			checkQuery: func(query string, args []interface{}) bool {
				return len(args) >= 2 && args[0] == "\"jazz\"*"
			},
		},
		{
			name:   "Multi-word FTS query",
			query:  "miles davis",
			useFTS: true,
			checkQuery: func(query string, args []interface{}) bool {
				return len(args) >= 2 && args[0] == "\"miles\" AND \"davis\""
			},
		},
		{
			name:   "LIKE fallback query",
			query:  "jazz",
			useFTS: false,
			checkQuery: func(query string, args []interface{}) bool {
				return len(args) >= 5 && args[0] == "%jazz%"
			},
		},
		{
			name:  "Filter by genre",
			query: "",
			filters: &SearchFilters{
				Genre: "Jazz",
				Limit: 10,
			},
			checkQuery: func(query string, args []interface{}) bool {
				return len(args) == 2 && args[0] == "Jazz" && args[1] == 10
			},
		},
		{
			name:  "Filter by starred",
			query: "music",
			filters: &SearchFilters{
				Starred: func() *bool { b := true; return &b }(),
				Limit:   5,
			},
			checkQuery: func(query string, args []interface{}) bool {
				// Check that starred condition is in the query
				return len(args) > 0
			},
		},
		{
			name:  "Filter by playlist name",
			query: "",
			filters: &SearchFilters{
				Playlist: "My Favorites",
				Limit:    20,
			},
			checkQuery: func(query string, args []interface{}) bool {
				return len(args) == 2 && args[0] == "My Favorites"
			},
		},
		{
			name:  "Filter by playlist ID",
			query: "",
			filters: &SearchFilters{
				Playlist:      "PLAYLIST_123",
				UsePlaylistID: true,
				Limit:         15,
			},
			checkQuery: func(query string, args []interface{}) bool {
				return len(args) == 2 && args[0] == "PLAYLIST_123"
			},
		},
		{
			name:  "Combined filters",
			query: "love",
			filters: &SearchFilters{
				Genre:     "Pop",
				MinRating: 80,
				Artist:    "Beatles",
				Limit:     10,
			},
			checkQuery: func(query string, args []interface{}) bool {
				// Should have FTS query + 3 filters + limit
				return len(args) >= 6
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewSearchQueryBuilder(tt.query, tt.filters)
			if tt.useFTS {
				builder.WithFTS(true)
			} else {
				builder.WithFTS(false)
			}

			query, args, err := builder.Build()
			if (err != nil) != tt.wantError {
				t.Errorf("Build() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if err == nil && tt.checkQuery != nil {
				if !tt.checkQuery(query, args) {
					t.Errorf("Query validation failed. Query: %s, Args: %v", query, args)
				}
			}
		})
	}
}

func TestSearchCache(t *testing.T) {
	cache := NewSearchCache(3, 100*time.Millisecond)

	// Test basic set and get
	tracks := []Track{
		{Name: "Track 1", Artist: "Artist 1"},
		{Name: "Track 2", Artist: "Artist 2"},
	}
	cache.Set("key1", tracks)

	// Test cache hit
	if result, hit := cache.Get("key1"); !hit {
		t.Error("Expected cache hit")
	} else if len(result) != len(tracks) {
		t.Errorf("Expected %d tracks, got %d", len(tracks), len(result))
	}

	// Test cache miss
	if _, hit := cache.Get("nonexistent"); hit {
		t.Error("Expected cache miss")
	}

	// Test TTL expiration
	time.Sleep(150 * time.Millisecond)
	if _, hit := cache.Get("key1"); hit {
		t.Error("Expected cache miss after TTL expiration")
	}

	// Test cache eviction
	cache.Set("key1", tracks)
	cache.Set("key2", tracks)
	cache.Set("key3", tracks)
	cache.Set("key4", tracks) // Should evict oldest

	// key1 should be evicted
	if _, hit := cache.Get("key1"); hit {
		t.Error("Expected key1 to be evicted")
	}

	// Test clear
	cache.Clear()
	if _, hit := cache.Get("key2"); hit {
		t.Error("Expected cache to be empty after clear")
	}
}

func TestDatabaseSearchWithFTS(t *testing.T) {
	// Create test database
	dbPath := filepath.Join(os.TempDir(), "test_search.db")
	defer os.Remove(dbPath)

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	// Populate test data
	testTracks := []Track{
		{
			PersistentID: "TRACK_001",
			Name:         "Blue in Green",
			Artist:       "Miles Davis",
			Album:        "Kind of Blue",
			Genre:        "Jazz",
			Collection:   "Jazz Classics",
			Rating:       100,
			Starred:      true,
			PlayCount:    50,
			LastPlayed:   func() *time.Time { t := time.Now(); return &t }(),
		},
		{
			PersistentID: "TRACK_002",
			Name:         "So What",
			Artist:       "Miles Davis",
			Album:        "Kind of Blue",
			Genre:        "Jazz",
			Collection:   "Jazz Classics",
			Rating:       90,
			Starred:      false,
			PlayCount:    30,
		},
		{
			PersistentID: "TRACK_003",
			Name:         "Autumn Leaves",
			Artist:       "Bill Evans",
			Album:        "Portrait in Jazz",
			Genre:        "Jazz",
			Collection:   "Jazz Standards",
			Rating:       85,
			Starred:      true,
			PlayCount:    25,
		},
		{
			PersistentID: "TRACK_004",
			Name:         "Let It Be",
			Artist:       "The Beatles",
			Album:        "Let It Be",
			Genre:        "Rock",
			Collection:   "Rock Classics",
			Rating:       95,
			Starred:      true,
			PlayCount:    100,
		},
		{
			PersistentID: "TRACK_005",
			Name:         "Yesterday",
			Artist:       "The Beatles",
			Album:        "Help!",
			Genre:        "Rock",
			Collection:   "Rock Classics",
			Rating:       100,
			Starred:      true,
			PlayCount:    120,
			LastPlayed:   func() *time.Time { t := time.Now().Add(-24 * time.Hour); return &t }(),
		},
	}

	// Insert test tracks
	err = dm.BatchInsertTracks(testTracks)
	if err != nil {
		t.Fatalf("Failed to insert test tracks: %v", err)
	}

	// Create search manager
	sm := NewSearchManager(dm)

	// Test cases
	tests := []struct {
		name        string
		query       string
		filters     *SearchFilters
		expectCount int
		expectFirst string // Expected first result's persistent ID
		expectError bool
	}{
		{
			name:        "Search by artist name",
			query:       "Miles Davis",
			expectCount: 2,
			expectFirst: "TRACK_001", // Blue in Green should rank higher
		},
		{
			name:        "Search by track name",
			query:       "Blue",
			expectCount: 2, // Both tracks match: "Blue in Green" and album "Kind of Blue"
			expectFirst: "TRACK_001",
		},
		{
			name:        "Search by album",
			query:       "Kind of Blue",
			expectCount: 2,
			expectFirst: "TRACK_001",
		},
		{
			name:        "Search with genre filter",
			query:       "",
			filters:     &SearchFilters{Genre: "Jazz"},
			expectCount: 3,
		},
		{
			name:        "Search with starred filter",
			query:       "",
			filters:     &SearchFilters{Starred: func() *bool { b := true; return &b }()},
			expectCount: 4,
		},
		{
			name:        "Search with rating filter",
			query:       "",
			filters:     &SearchFilters{MinRating: 95},
			expectCount: 3,           // TRACK_001 (100), TRACK_004 (95), and TRACK_005 (100) match
			expectFirst: "TRACK_005", // TRACK_005 has highest play count and recent last played
		},
		{
			name:  "Combined search and filters",
			query: "Beatles",
			filters: &SearchFilters{
				Genre:     "Rock",
				MinRating: 90,
			},
			expectCount: 2,
		},
		{
			name:        "Partial match search",
			query:       "yest",
			expectCount: 1,
			expectFirst: "TRACK_005",
		},
		{
			name:        "No results",
			query:       "nonexistent",
			expectCount: 0,
		},
		{
			name:        "Limit results",
			query:       "",
			filters:     &SearchFilters{Limit: 2},
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracks, err := sm.SearchTracksOptimized(tt.query, tt.filters)
			if (err != nil) != tt.expectError {
				t.Errorf("SearchTracksOptimized() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if err == nil {
				if len(tracks) != tt.expectCount {
					t.Errorf("Expected %d tracks, got %d", tt.expectCount, len(tracks))
					t.Errorf("Results: %+v", tracks)
				}

				if tt.expectFirst != "" && len(tracks) > 0 {
					if tracks[0].PersistentID != tt.expectFirst {
						t.Errorf("Expected first result to be %s, got %s", tt.expectFirst, tracks[0].PersistentID)
					}
				}
			}
		})
	}
}

func TestSearchPerformance(t *testing.T) {
	// Create test database
	dbPath := filepath.Join(os.TempDir(), "test_perf.db")
	defer os.Remove(dbPath)

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	// Generate test data
	var tracks []Track
	for i := 0; i < 5000; i++ {
		track := Track{
			PersistentID: fmt.Sprintf("TRACK_%06d", i),
			Name:         fmt.Sprintf("Track %d", i),
			Artist:       fmt.Sprintf("Artist %d", i%100),
			Album:        fmt.Sprintf("Album %d", i%200),
			Genre:        []string{"Rock", "Jazz", "Pop", "Classical", "Electronic"}[i%5],
			Collection:   fmt.Sprintf("Collection %d", i%50),
			Rating:       (i % 5) * 20,
			Starred:      i%3 == 0,
			PlayCount:    i % 100,
		}
		tracks = append(tracks, track)
	}

	// Insert in batches
	batchSize := 500
	for i := 0; i < len(tracks); i += batchSize {
		end := i + batchSize
		if end > len(tracks) {
			end = len(tracks)
		}
		if err := dm.BatchInsertTracks(tracks[i:end]); err != nil {
			t.Fatalf("Failed to insert batch: %v", err)
		}
	}

	sm := NewSearchManager(dm)

	// Test search performance
	searchQueries := []string{
		"Track 100",
		"Artist 50",
		"Jazz",
		"Collection 25",
		"Track",
	}

	for _, query := range searchQueries {
		start := time.Now()
		results, err := sm.SearchTracksOptimized(query, &SearchFilters{Limit: 15})
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Search failed for query '%s': %v", query, err)
			continue
		}

		if duration > 10*time.Millisecond {
			t.Errorf("Search for '%s' took %v, expected <10ms", query, duration)
		}

		t.Logf("Search for '%s' returned %d results in %v", query, len(results), duration)
	}

	// Test cached search performance
	for _, query := range searchQueries {
		// First call to populate cache
		sm.SearchWithCache(query, &SearchFilters{Limit: 15})

		// Second call should hit cache
		start := time.Now()
		results, err := sm.SearchWithCache(query, &SearchFilters{Limit: 15})
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Cached search failed for query '%s': %v", query, err)
			continue
		}

		if duration > 1*time.Millisecond {
			t.Errorf("Cached search for '%s' took %v, expected <1ms", query, duration)
		}

		t.Logf("Cached search for '%s' returned %d results in %v", query, len(results), duration)
	}

	// Check average search time
	avgTime := sm.GetAverageSearchTime()
	t.Logf("Average search time: %v", avgTime)
	if avgTime > 10*time.Millisecond {
		t.Errorf("Average search time %v exceeds 10ms target", avgTime)
	}
}

func TestSearchMetrics(t *testing.T) {
	// Create test database
	dbPath := filepath.Join(os.TempDir(), "test_metrics.db")
	defer os.Remove(dbPath)

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	// Insert minimal test data
	testTrack := Track{
		PersistentID: "TEST_001",
		Name:         "Test Track",
		Artist:       "Test Artist",
		Album:        "Test Album",
		Genre:        "Test",
	}
	dm.BatchInsertTracks([]Track{testTrack})

	sm := NewSearchManager(dm)

	// Perform some searches
	sm.SearchWithCache("test", nil)
	sm.SearchWithCache("test", nil) // Cache hit
	sm.SearchTracksOptimized("artist", nil)

	metrics := sm.GetMetrics()

	// Debug: Print all metrics to understand what's being recorded
	for i, m := range metrics {
		t.Logf("Metric %d: Query=%q, Method=%s, CacheHit=%v, ResultCount=%d",
			i, m.Query, m.Method, m.CacheHit, m.ResultCount)
	}

	if len(metrics) != 4 {
		t.Errorf("Expected 4 metrics, got %d", len(metrics))
	}

	// Check for cache hit
	cacheHits := 0
	for _, m := range metrics {
		if m.CacheHit {
			cacheHits++
		}
	}
	if cacheHits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", cacheHits)
	}
}

func BenchmarkDatabaseSearch(b *testing.B) {
	// Create test database
	dbPath := filepath.Join(os.TempDir(), "bench_search.db")
	defer os.Remove(dbPath)

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	// Generate test data
	var tracks []Track
	for i := 0; i < 10000; i++ {
		track := Track{
			PersistentID: fmt.Sprintf("TRACK_%06d", i),
			Name:         fmt.Sprintf("Track %d", i),
			Artist:       fmt.Sprintf("Artist %d", i%100),
			Album:        fmt.Sprintf("Album %d", i%200),
			Genre:        []string{"Rock", "Jazz", "Pop", "Classical", "Electronic"}[i%5],
		}
		tracks = append(tracks, track)
	}

	// Insert all tracks
	for i := 0; i < len(tracks); i += 1000 {
		end := i + 1000
		if end > len(tracks) {
			end = len(tracks)
		}
		dm.BatchInsertTracks(tracks[i:end])
	}

	sm := NewSearchManager(dm)

	b.Run("FTS_Search", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			query := fmt.Sprintf("Track %d", i%100)
			sm.SearchTracksOptimized(query, &SearchFilters{Limit: 15})
		}
	})

	b.Run("Cached_Search", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			query := fmt.Sprintf("Artist %d", i%10) // Use fewer queries to test cache
			sm.SearchWithCache(query, &SearchFilters{Limit: 15})
		}
	})

	b.Run("Filter_Search", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filters := &SearchFilters{
				Genre: []string{"Rock", "Jazz", "Pop"}[i%3],
				Limit: 15,
			}
			sm.SearchTracksOptimized("", filters)
		}
	})
}
