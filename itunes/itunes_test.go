package itunes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRefreshResponseParsing tests parsing of the enhanced refresh response
func TestRefreshResponseParsing(t *testing.T) {
	// Mock enhanced response from JXA script
	mockResponse := `{
		"status": "success",
		"data": {
			"tracks": [
				{
					"id": "ABCD1234567890EF",
					"persistent_id": "ABCD1234567890EF",
					"name": "Blue in Green",
					"album": "Kind of Blue",
					"collection": "Jazz Favorites",
					"artist": "Miles Davis",
					"playlists": ["Jazz Favorites", "Chill"],
					"genre": "Jazz",
					"rating": 100,
					"starred": true
				},
				{
					"id": "1234567890ABCDEF",
					"persistent_id": "1234567890ABCDEF",
					"name": "So What",
					"album": "Kind of Blue",
					"collection": "Kind of Blue",
					"artist": "Miles Davis",
					"playlists": [],
					"genre": "Jazz",
					"rating": 80,
					"starred": false
				}
			],
			"playlists": [
				{
					"id": "PLAYLIST1234567890",
					"name": "Jazz Favorites",
					"special_kind": "none",
					"track_count": 42,
					"genre": "Jazz"
				},
				{
					"id": "PLAYLIST0987654321",
					"name": "Recently Added",
					"special_kind": "recentlyAdded",
					"track_count": 100,
					"genre": ""
				}
			],
			"stats": {
				"track_count": 2,
				"playlist_count": 2,
				"skipped_tracks": 0,
				"refresh_time": "2025-01-22T10:00:00Z"
			}
		}
	}`

	var response RefreshResponse
	err := json.Unmarshal([]byte(mockResponse), &response)
	if err != nil {
		t.Fatalf("Failed to parse refresh response: %v", err)
	}

	// Verify response status
	if response.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", response.Status)
	}

	// Verify tracks
	if len(response.Data.Tracks) != 2 {
		t.Errorf("Expected 2 tracks, got %d", len(response.Data.Tracks))
	}

	// Verify first track with persistent ID
	track1 := response.Data.Tracks[0]
	if track1.PersistentID != "ABCD1234567890EF" {
		t.Errorf("Expected persistent ID 'ABCD1234567890EF', got '%s'", track1.PersistentID)
	}
	if track1.ID != track1.PersistentID {
		t.Errorf("Expected ID to match persistent ID, got ID='%s', PersistentID='%s'", track1.ID, track1.PersistentID)
	}
	if track1.Genre != "Jazz" {
		t.Errorf("Expected genre 'Jazz', got '%s'", track1.Genre)
	}
	if track1.Rating != 100 {
		t.Errorf("Expected rating 100, got %d", track1.Rating)
	}
	if !track1.Starred {
		t.Error("Expected track to be starred")
	}
	if len(track1.Playlists) != 2 {
		t.Errorf("Expected 2 playlists, got %d", len(track1.Playlists))
	}

	// Verify playlists
	if len(response.Data.Playlists) != 2 {
		t.Errorf("Expected 2 playlists, got %d", len(response.Data.Playlists))
	}

	playlist1 := response.Data.Playlists[0]
	if playlist1.ID != "PLAYLIST1234567890" {
		t.Errorf("Expected playlist ID 'PLAYLIST1234567890', got '%s'", playlist1.ID)
	}
	if playlist1.SpecialKind != "none" {
		t.Errorf("Expected special kind 'none', got '%s'", playlist1.SpecialKind)
	}
	if playlist1.TrackCount != 42 {
		t.Errorf("Expected track count 42, got %d", playlist1.TrackCount)
	}

	// Verify stats
	if response.Data.Stats.TrackCount != 2 {
		t.Errorf("Expected track count 2, got %d", response.Data.Stats.TrackCount)
	}
	if response.Data.Stats.PlaylistCount != 2 {
		t.Errorf("Expected playlist count 2, got %d", response.Data.Stats.PlaylistCount)
	}
}

// TestBackwardCompatibility tests that the new structure maintains backward compatibility
func TestBackwardCompatibility(t *testing.T) {
	// Create a track with minimal fields (backward compatible)
	track := Track{
		ID:         "12345",
		Name:       "Test Track",
		Album:      "Test Album",
		Collection: "Test Collection",
		Artist:     "Test Artist",
		Playlists:  []string{"Playlist1"},
	}

	// Marshal and unmarshal to verify JSON compatibility
	data, err := json.Marshal(track)
	if err != nil {
		t.Fatalf("Failed to marshal track: %v", err)
	}

	var parsedTrack Track
	err = json.Unmarshal(data, &parsedTrack)
	if err != nil {
		t.Fatalf("Failed to unmarshal track: %v", err)
	}

	// Verify fields
	if parsedTrack.ID != track.ID {
		t.Errorf("Expected ID '%s', got '%s'", track.ID, parsedTrack.ID)
	}
	if parsedTrack.PersistentID != "" {
		t.Errorf("Expected empty PersistentID for backward compatibility, got '%s'", parsedTrack.PersistentID)
	}
}

// TestErrorResponse tests handling of error responses
func TestErrorResponse(t *testing.T) {
	mockErrorResponse := `{
		"status": "error",
		"message": "Library refresh error: Music app not running",
		"error": "AppleScriptError",
		"data": {
			"tracks": [],
			"playlists": [],
			"stats": {
				"track_count": 0,
				"playlist_count": 0,
				"skipped_tracks": 0,
				"refresh_time": "2025-01-22T10:00:00Z"
			}
		}
	}`

	var response RefreshResponse
	err := json.Unmarshal([]byte(mockErrorResponse), &response)
	if err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if response.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", response.Status)
	}
	if response.Message == "" {
		t.Error("Expected error message, got empty string")
	}
	if len(response.Data.Tracks) != 0 {
		t.Errorf("Expected 0 tracks in error response, got %d", len(response.Data.Tracks))
	}
}

// TestEmptyLibrary tests handling of empty library
func TestEmptyLibrary(t *testing.T) {
	mockEmptyResponse := `{
		"status": "success",
		"data": {
			"tracks": [],
			"playlists": [],
			"stats": {
				"track_count": 0,
				"playlist_count": 0,
				"skipped_tracks": 0,
				"refresh_time": "2025-01-22T10:00:00Z"
			}
		}
	}`

	var response RefreshResponse
	err := json.Unmarshal([]byte(mockEmptyResponse), &response)
	if err != nil {
		t.Fatalf("Failed to parse empty library response: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", response.Status)
	}
	if len(response.Data.Tracks) != 0 {
		t.Errorf("Expected 0 tracks, got %d", len(response.Data.Tracks))
	}
	if len(response.Data.Playlists) != 0 {
		t.Errorf("Expected 0 playlists, got %d", len(response.Data.Playlists))
	}
}

// TestLargeLibraryResponse tests handling of large library with many tracks
func TestLargeLibraryResponse(t *testing.T) {
	// Create a response with many tracks
	var tracks []Track
	for i := 0; i < 1000; i++ {
		tracks = append(tracks, Track{
			ID:           generateMockPersistentID(i),
			PersistentID: generateMockPersistentID(i),
			Name:         "Track " + string(rune(i)),
			Album:        "Album " + string(rune(i/10)),
			Collection:   "Collection " + string(rune(i/100)),
			Artist:       "Artist " + string(rune(i/50)),
			Playlists:    []string{},
			Genre:        "Genre " + string(rune(i/200)),
			Rating:       (i % 5) * 20,
			Starred:      i%10 == 0,
		})
	}

	response := RefreshResponse{
		Status: "success",
		Data: struct {
			Tracks    []Track        `json:"tracks"`
			Playlists []PlaylistData `json:"playlists"`
			Stats     RefreshStats   `json:"stats"`
		}{
			Tracks:    tracks,
			Playlists: []PlaylistData{},
			Stats: RefreshStats{
				TrackCount:    1000,
				PlaylistCount: 0,
				SkippedTracks: 0,
				RefreshTime:   time.Now().Format(time.RFC3339),
			},
		},
	}

	// Test marshaling performance
	start := time.Now()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal large response: %v", err)
	}
	marshalDuration := time.Since(start)

	// Test unmarshaling performance
	start = time.Now()
	var parsedResponse RefreshResponse
	err = json.Unmarshal(data, &parsedResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal large response: %v", err)
	}
	unmarshalDuration := time.Since(start)

	// Verify data integrity
	if len(parsedResponse.Data.Tracks) != 1000 {
		t.Errorf("Expected 1000 tracks, got %d", len(parsedResponse.Data.Tracks))
	}

	// Performance checks (should be fast even with 1000 tracks)
	if marshalDuration > 100*time.Millisecond {
		t.Logf("Warning: Marshal took %v (expected < 100ms)", marshalDuration)
	}
	if unmarshalDuration > 100*time.Millisecond {
		t.Logf("Warning: Unmarshal took %v (expected < 100ms)", unmarshalDuration)
	}
}

// TestPlaylistDataParsing tests parsing of playlist data
func TestPlaylistDataParsing(t *testing.T) {
	mockPlaylist := `{
		"id": "PLAYLIST1234567890",
		"name": "My Awesome Playlist",
		"special_kind": "none",
		"track_count": 42,
		"genre": "Mixed"
	}`

	var playlist PlaylistData
	err := json.Unmarshal([]byte(mockPlaylist), &playlist)
	if err != nil {
		t.Fatalf("Failed to parse playlist data: %v", err)
	}

	if playlist.ID != "PLAYLIST1234567890" {
		t.Errorf("Expected ID 'PLAYLIST1234567890', got '%s'", playlist.ID)
	}
	if playlist.Name != "My Awesome Playlist" {
		t.Errorf("Expected name 'My Awesome Playlist', got '%s'", playlist.Name)
	}
	if playlist.SpecialKind != "none" {
		t.Errorf("Expected special kind 'none', got '%s'", playlist.SpecialKind)
	}
	if playlist.TrackCount != 42 {
		t.Errorf("Expected track count 42, got %d", playlist.TrackCount)
	}
	if playlist.Genre != "Mixed" {
		t.Errorf("Expected genre 'Mixed', got '%s'", playlist.Genre)
	}
}

// TestTrackWithAllFields tests a track with all enhanced fields populated
func TestTrackWithAllFields(t *testing.T) {
	track := Track{
		ID:           "ABCD1234567890EF",
		PersistentID: "ABCD1234567890EF",
		Name:         "Take Five",
		Album:        "Time Out",
		Collection:   "Jazz Classics",
		Artist:       "Dave Brubeck Quartet",
		Playlists:    []string{"Jazz Classics", "Study Music", "Favorites"},
		Genre:        "Jazz",
		Rating:       100,
		Starred:      true,
	}

	// Test JSON round-trip
	data, err := json.Marshal(track)
	if err != nil {
		t.Fatalf("Failed to marshal track: %v", err)
	}

	var parsedTrack Track
	err = json.Unmarshal(data, &parsedTrack)
	if err != nil {
		t.Fatalf("Failed to unmarshal track: %v", err)
	}

	// Verify all fields
	if parsedTrack.ID != track.ID {
		t.Errorf("ID mismatch: expected '%s', got '%s'", track.ID, parsedTrack.ID)
	}
	if parsedTrack.PersistentID != track.PersistentID {
		t.Errorf("PersistentID mismatch: expected '%s', got '%s'", track.PersistentID, parsedTrack.PersistentID)
	}
	if parsedTrack.Genre != track.Genre {
		t.Errorf("Genre mismatch: expected '%s', got '%s'", track.Genre, parsedTrack.Genre)
	}
	if parsedTrack.Rating != track.Rating {
		t.Errorf("Rating mismatch: expected %d, got %d", track.Rating, parsedTrack.Rating)
	}
	if parsedTrack.Starred != track.Starred {
		t.Errorf("Starred mismatch: expected %v, got %v", track.Starred, parsedTrack.Starred)
	}
	if len(parsedTrack.Playlists) != len(track.Playlists) {
		t.Errorf("Playlists count mismatch: expected %d, got %d", len(track.Playlists), len(parsedTrack.Playlists))
	}
}

// TestCacheFileCreation tests that the cache file is created correctly
func TestCacheFileCreation(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "itunes-cache")
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create test tracks
	tracks := []Track{
		{
			ID:           "TEST123",
			PersistentID: "TEST123",
			Name:         "Test Track",
			Album:        "Test Album",
			Collection:   "Test Collection",
			Artist:       "Test Artist",
			Playlists:    []string{},
		},
	}

	// Write to cache file
	libraryJSON, err := json.Marshal(tracks)
	if err != nil {
		t.Fatalf("Failed to marshal tracks: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "library.json")
	err = os.WriteFile(cacheFile, libraryJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Verify file exists and can be read
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}

	var cachedTracks []Track
	err = json.Unmarshal(data, &cachedTracks)
	if err != nil {
		t.Fatalf("Failed to unmarshal cached tracks: %v", err)
	}

	if len(cachedTracks) != 1 {
		t.Errorf("Expected 1 cached track, got %d", len(cachedTracks))
	}
	if cachedTracks[0].ID != "TEST123" {
		t.Errorf("Expected cached track ID 'TEST123', got '%s'", cachedTracks[0].ID)
	}
}

// Helper function to generate mock persistent IDs
func generateMockPersistentID(index int) string {
	// Generate a hex-like string similar to Apple Music persistent IDs
	return "MOCK" + string(rune(1000000000000000+index))
}
