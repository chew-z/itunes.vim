package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMigrateFromJSON tests migration from JSON cache files
func TestMigrateFromJSON(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create test data
	testTracks := []JSONTrack{
		{
			ID:           "TEST001",
			PersistentID: "PERSIST001",
			Name:         "Test Track 1",
			Artist:       "Test Artist",
			Album:        "Test Album",
			Collection:   "Test Collection",
			Playlists:    []string{"Test Playlist", "Favorites"},
			Genre:        "Rock",
			Rating:       80,
			Starred:      true,
		},
		{
			ID:           "TEST002",
			PersistentID: "PERSIST002",
			Name:         "Test Track 2",
			Artist:       "Test Artist 2",
			Album:        "Test Album 2",
			Collection:   "Test Collection 2",
			Playlists:    []string{"Test Playlist"},
			Genre:        "Jazz",
			Rating:       90,
			Starred:      false,
		},
	}

	testPlaylists := []PlaylistData{
		{
			ID:          "PLAYLIST001",
			Name:        "Test Playlist",
			SpecialKind: "none",
			TrackCount:  2,
		},
		{
			ID:          "PLAYLIST002",
			Name:        "Favorites",
			SpecialKind: "none",
			TrackCount:  1,
		},
	}

	// Test Case 1: Enhanced format
	t.Run("EnhancedFormat", func(t *testing.T) {
		// Create enhanced cache file
		enhancedData := RefreshResponse{
			Status: "success",
			Data: &RefreshData{
				Tracks:    testTracks,
				Playlists: testPlaylists,
				Stats: RefreshStats{
					TrackCount:    2,
					PlaylistCount: 2,
					RefreshTime:   time.Now().Format(time.RFC3339),
				},
			},
		}

		enhancedJSON, err := json.Marshal(enhancedData)
		if err != nil {
			t.Fatalf("Failed to marshal enhanced data: %v", err)
		}

		if err := os.WriteFile(filepath.Join(cacheDir, "library_enhanced.json"), enhancedJSON, 0644); err != nil {
			t.Fatalf("Failed to write enhanced cache file: %v", err)
		}

		// Create database manager
		dm, err := NewDatabaseManager(dbPath)
		if err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}
		defer dm.Close()

		// Perform migration
		err = dm.MigrateFromJSON(cacheDir, nil)
		if err != nil {
			t.Fatalf("Migration failed: %v", err)
		}

		// Verify migration
		stats, err := dm.GetStats()
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TrackCount != 2 {
			t.Errorf("Expected 2 tracks, got %d", stats.TrackCount)
		}

		if stats.PlaylistCount != 2 {
			t.Errorf("Expected 2 playlists, got %d", stats.PlaylistCount)
		}

		// Verify track data
		track, err := dm.GetTrackByPersistentID("PERSIST001")
		if err != nil {
			t.Fatalf("Failed to get track: %v", err)
		}

		if track.Name != "Test Track 1" {
			t.Errorf("Expected track name 'Test Track 1', got '%s'", track.Name)
		}

		if track.Rating != 80 {
			t.Errorf("Expected rating 80, got %d", track.Rating)
		}

		if !track.Starred {
			t.Error("Expected track to be starred")
		}

		// Verify playlist associations
		if len(track.Playlists) != 2 {
			t.Errorf("Expected 2 playlists, got %d", len(track.Playlists))
		}
	})

	// Test Case 2: Legacy format
	t.Run("LegacyFormat", func(t *testing.T) {
		// Remove enhanced file
		os.Remove(filepath.Join(cacheDir, "library_enhanced.json"))

		// Create legacy cache file (tracks array only)
		legacyJSON, err := json.Marshal(testTracks)
		if err != nil {
			t.Fatalf("Failed to marshal legacy data: %v", err)
		}

		if err := os.WriteFile(filepath.Join(cacheDir, "library.json"), legacyJSON, 0644); err != nil {
			t.Fatalf("Failed to write legacy cache file: %v", err)
		}

		// Create new database
		dbPath2 := filepath.Join(tmpDir, "test2.db")
		dm, err := NewDatabaseManager(dbPath2)
		if err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}
		defer dm.Close()

		// Perform migration
		var callbackCalled bool
		err = dm.MigrateFromJSON(cacheDir, func(p MigrationProgress) {
			callbackCalled = true
			t.Logf("Progress: %d/%d tracks, %d/%d playlists",
				p.ProcessedTracks, p.TotalTracks,
				p.ProcessedPlaylists, p.TotalPlaylists)
		})
		if err != nil {
			t.Fatalf("Migration failed: %v", err)
		}

		if !callbackCalled {
			t.Error("Progress callback was not called")
		}

		// Verify migration
		stats, err := dm.GetStats()
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TrackCount != 2 {
			t.Errorf("Expected 2 tracks, got %d", stats.TrackCount)
		}

		// Legacy format should extract playlists from track data
		if stats.PlaylistCount != 2 {
			t.Errorf("Expected 2 playlists extracted from tracks, got %d", stats.PlaylistCount)
		}

		// Verify playlists were created
		playlists, err := dm.ListPlaylists()
		if err != nil {
			t.Fatalf("Failed to list playlists: %v", err)
		}

		playlistNames := make(map[string]bool)
		for _, p := range playlists {
			playlistNames[p.Name] = true
		}

		if !playlistNames["Test Playlist"] {
			t.Error("Expected 'Test Playlist' to be created")
		}

		if !playlistNames["Favorites"] {
			t.Error("Expected 'Favorites' to be created")
		}
	})

	// Test Case 3: Missing cache files
	t.Run("MissingCacheFiles", func(t *testing.T) {
		nonExistentDir := filepath.Join(tmpDir, "nonexistent")
		dbPath3 := filepath.Join(tmpDir, "test3.db")
		dm, err := NewDatabaseManager(dbPath3)
		if err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}
		defer dm.Close()

		err = dm.MigrateFromJSON(nonExistentDir, nil)
		if err == nil {
			t.Error("Expected error for missing cache directory")
		}
	})
}

// TestPopulateFromRefreshScript tests direct population from RefreshResponse
func TestPopulateFromRefreshScript(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	// Create test response
	response := &RefreshResponse{
		Status: "success",
		Data: &RefreshData{
			Tracks: []JSONTrack{
				{
					ID:           "TRACK001",
					PersistentID: "PERSIST001",
					Name:         "Direct Track 1",
					Artist:       "Direct Artist",
					Album:        "Direct Album",
					Collection:   "Direct Collection",
					Playlists:    []string{"Direct Playlist"},
					Genre:        "Electronic",
					Rating:       100,
					Starred:      true,
				},
				{
					ID:           "TRACK002",
					PersistentID: "PERSIST002",
					Name:         "Direct Track 2",
					Artist:       "Direct Artist 2",
					Album:        "Direct Album 2",
					Collection:   "Direct Collection 2",
					Playlists:    []string{},
					Genre:        "Classical",
					Rating:       75,
					Starred:      false,
				},
			},
			Playlists: []PlaylistData{
				{
					ID:          "DIRECT_PLAYLIST001",
					Name:        "Direct Playlist",
					SpecialKind: "none",
					TrackCount:  1,
				},
			},
			Stats: RefreshStats{
				TrackCount:    2,
				PlaylistCount: 1,
				RefreshTime:   time.Now().Format(time.RFC3339),
			},
		},
	}

	// Populate database
	progressUpdates := 0
	err = dm.PopulateFromRefreshScript(response, func(p MigrationProgress) {
		progressUpdates++
		t.Logf("Progress: %d/%d tracks, %d/%d playlists, elapsed: %v",
			p.ProcessedTracks, p.TotalTracks,
			p.ProcessedPlaylists, p.TotalPlaylists,
			p.ElapsedTime)
	})

	if err != nil {
		t.Fatalf("Population failed: %v", err)
	}

	if progressUpdates == 0 {
		t.Error("No progress updates received")
	}

	// Verify data
	stats, err := dm.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TrackCount != 2 {
		t.Errorf("Expected 2 tracks, got %d", stats.TrackCount)
	}

	if stats.PlaylistCount != 1 {
		t.Errorf("Expected 1 playlist, got %d", stats.PlaylistCount)
	}

	// Verify FTS index
	tracks, err := dm.SearchTracksWithFTS("Direct", nil)
	if err != nil {
		t.Fatalf("FTS search failed: %v", err)
	}

	if len(tracks) != 2 {
		t.Errorf("Expected 2 tracks from FTS search, got %d", len(tracks))
	}
}

// TestBatchOperations tests batch insert and update operations
func TestBatchOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	// Test batch insert
	tracks := make([]Track, 100)
	for i := 0; i < 100; i++ {
		tracks[i] = Track{
			PersistentID: fmt.Sprintf("BATCH_%03d", i),
			Name:         fmt.Sprintf("Batch Track %d", i),
			Artist:       fmt.Sprintf("Artist %d", i%10),
			Album:        fmt.Sprintf("Album %d", i%20),
			Collection:   "Batch Collection",
			Genre:        []string{"Rock", "Jazz", "Pop", "Classical"}[i%4],
			Rating:       (i % 5) * 20,
			Starred:      i%2 == 0,
		}
	}

	start := time.Now()
	err = dm.BatchInsertTracks(tracks)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Batch insert failed: %v", err)
	}

	t.Logf("Batch insert of 100 tracks took %v", elapsed)

	// Verify all tracks were inserted
	stats, err := dm.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TrackCount != 100 {
		t.Errorf("Expected 100 tracks, got %d", stats.TrackCount)
	}

	// Test batch update
	jsonTracks := make([]JSONTrack, 50)
	for i := 0; i < 50; i++ {
		jsonTracks[i] = JSONTrack{
			PersistentID: fmt.Sprintf("BATCH_%03d", i),
			Name:         fmt.Sprintf("Updated Batch Track %d", i),
			Artist:       fmt.Sprintf("Updated Artist %d", i%5),
			Album:        fmt.Sprintf("Updated Album %d", i%10),
			Collection:   "Updated Collection",
			Genre:        "Updated",
			Rating:       100,
			Starred:      true,
		}
	}

	start = time.Now()
	err = dm.BatchUpdateTracks(jsonTracks)
	elapsed = time.Since(start)

	if err != nil {
		t.Fatalf("Batch update failed: %v", err)
	}

	t.Logf("Batch update of 50 tracks took %v", elapsed)

	// Verify updates
	track, err := dm.GetTrackByPersistentID("BATCH_000")
	if err != nil {
		t.Fatalf("Failed to get updated track: %v", err)
	}

	if track.Name != "Updated Batch Track 0" {
		t.Errorf("Expected updated name, got '%s'", track.Name)
	}

	if track.Rating != 100 {
		t.Errorf("Expected rating 100, got %d", track.Rating)
	}
}

// TestPlaylistOperations tests playlist-specific operations
func TestPlaylistOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	// Insert test tracks
	testTracks := []Track{
		{PersistentID: "TRACK001", Name: "Song 1", Artist: "Artist", Album: "Album"},
		{PersistentID: "TRACK002", Name: "Song 2", Artist: "Artist", Album: "Album"},
		{PersistentID: "TRACK003", Name: "Song 3", Artist: "Artist", Album: "Album"},
	}

	err = dm.BatchInsertTracks(testTracks)
	if err != nil {
		t.Fatalf("Failed to insert test tracks: %v", err)
	}

	// Get track IDs
	var trackIDs []int64
	for _, track := range testTracks {
		dbTrack, _ := dm.GetTrackByPersistentID(track.PersistentID)
		trackIDs = append(trackIDs, dbTrack.ID)
	}

	// Create playlist
	playlist := &PlaylistData{
		ID:          "PLAYLIST001",
		Name:        "Test Playlist",
		SpecialKind: "none",
		TrackCount:  3,
	}

	err = dm.UpsertPlaylist(playlist)
	if err != nil {
		t.Fatalf("Failed to create playlist: %v", err)
	}

	// Get playlist from database
	dbPlaylist, err := dm.GetPlaylistByPersistentID("PLAYLIST001")
	if err != nil {
		t.Fatalf("Failed to get playlist: %v", err)
	}

	// Associate tracks with playlist
	err = dm.BatchInsertPlaylistTracks(dbPlaylist.ID, trackIDs)
	if err != nil {
		t.Fatalf("Failed to associate tracks with playlist: %v", err)
	}

	// Get playlist tracks
	tracks, err := dm.GetPlaylistTracks("PLAYLIST001", true)
	if err != nil {
		t.Fatalf("Failed to get playlist tracks: %v", err)
	}

	if len(tracks) != 3 {
		t.Errorf("Expected 3 tracks in playlist, got %d", len(tracks))
	}

	// Verify track order
	for i, track := range tracks {
		expectedName := fmt.Sprintf("Song %d", i+1)
		if track.Name != expectedName {
			t.Errorf("Expected track %d to be '%s', got '%s'", i, expectedName, track.Name)
		}
	}

	// Test SyncPlaylist
	// Remove one track and add a new one
	newTrack := Track{PersistentID: "TRACK004", Name: "Song 4", Artist: "Artist", Album: "Album"}
	err = dm.BatchInsertTracks([]Track{newTrack})
	if err != nil {
		t.Fatalf("Failed to insert new track: %v", err)
	}

	newTrackPersistentIDs := []string{"TRACK001", "TRACK003", "TRACK004"} // Remove track 2, add track 4

	err = dm.SyncPlaylist("PLAYLIST001", newTrackPersistentIDs)
	if err != nil {
		t.Fatalf("Failed to sync playlist: %v", err)
	}

	// Verify sync
	tracks, err = dm.GetPlaylistTracks("PLAYLIST001", true)
	if err != nil {
		t.Fatalf("Failed to get playlist tracks after sync: %v", err)
	}

	if len(tracks) != 3 {
		t.Errorf("Expected 3 tracks after sync, got %d", len(tracks))
	}

	// Check that track 2 is gone and track 4 is added
	foundTrack2 := false
	foundTrack4 := false
	for _, track := range tracks {
		if track.Name == "Song 2" {
			foundTrack2 = true
		}
		if track.Name == "Song 4" {
			foundTrack4 = true
		}
	}

	if foundTrack2 {
		t.Error("Track 2 should have been removed from playlist")
	}

	if !foundTrack4 {
		t.Error("Track 4 should have been added to playlist")
	}
}

// TestMigrationValidation tests the validation function
func TestMigrationValidation(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create test data with unicode and special characters
	testTracks := []JSONTrack{
		{
			ID:           "UNICODE001",
			PersistentID: "PERSIST_UNI001",
			Name:         "Café français – Test",
			Artist:       "Björk",
			Album:        "Homogénic",
			Collection:   "Ñoño's Collection",
			Playlists:    []string{"日本語", "Русский"},
			Genre:        "Électronique",
			Rating:       95,
			Starred:      true,
		},
	}

	// Create cache file
	trackJSON, err := json.Marshal(testTracks)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(filepath.Join(cacheDir, "library.json"), trackJSON, 0644); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Create database and migrate
	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	err = dm.MigrateFromJSON(cacheDir, nil)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Validate migration
	valid, issues := dm.ValidateMigration(cacheDir)
	if !valid {
		t.Errorf("Migration validation failed: %v", issues)
	}

	// Verify unicode data
	track, err := dm.GetTrackByPersistentID("PERSIST_UNI001")
	if err != nil {
		t.Fatalf("Failed to get unicode track: %v", err)
	}

	if track.Name != "Café français – Test" {
		t.Errorf("Unicode track name corrupted: got '%s'", track.Name)
	}

	if track.Artist != "Björk" {
		t.Errorf("Unicode artist name corrupted: got '%s'", track.Artist)
	}
}

// BenchmarkBatchInsert benchmarks batch insert performance
func BenchmarkBatchInsert(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	dm, err := NewDatabaseManager(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer dm.Close()

	// Create test tracks
	tracks := make([]Track, 100)
	for i := 0; i < 100; i++ {
		tracks[i] = Track{
			PersistentID: fmt.Sprintf("BENCH_%06d", i),
			Name:         fmt.Sprintf("Track %d", i),
			Artist:       fmt.Sprintf("Artist %d", i%20),
			Album:        fmt.Sprintf("Album %d", i%30),
			Genre:        []string{"Rock", "Jazz", "Pop"}[i%3],
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Modify persistent IDs to avoid duplicates
		for j := range tracks {
			tracks[j].PersistentID = fmt.Sprintf("BENCH_%d_%06d", i, j)
		}

		if err := dm.BatchInsertTracks(tracks); err != nil {
			b.Fatalf("Batch insert failed: %v", err)
		}
	}
}
