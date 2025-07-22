package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"itunes/database"
	"itunes/itunes"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestMain sets up the test database
func TestMain(m *testing.M) {
	// Create a test database
	testDBPath := filepath.Join(os.TempDir(), "itunes_test.db")

	// Clean up any existing test database
	os.Remove(testDBPath)

	// Set the test database path via environment variable AND update the package variable
	os.Setenv("ITUNES_DB_PATH", testDBPath)
	database.PrimaryDBPath = testDBPath

	// Initialize database using the iTunes package
	if err := itunes.InitDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize test database: %v\n", err)
		os.Exit(1)
	}

	// Populate test data
	if err := populateTestData(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to populate test data: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	itunes.CloseDatabase()
	os.Remove(testDBPath)

	os.Exit(code)
}

func populateTestData() error {
	// Use the same database path that was set in TestMain
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
	if err != nil {
		return err
	}
	defer dm.Close()

	// Create test playlists using PlaylistData type
	playlists := []database.PlaylistData{
		{ID: "PLAYLIST001", Name: "Jazz Favorites", Genre: "Jazz", SpecialKind: "none"},
		{ID: "PLAYLIST002", Name: "Rock Classics", Genre: "Rock", SpecialKind: "none"},
		{ID: "PLAYLIST003", Name: "Chill Vibes", Genre: "Electronic", SpecialKind: "none"},
	}

	for _, p := range playlists {
		if err := dm.UpsertPlaylist(&p); err != nil {
			return err
		}
	}

	// Create test tracks
	tracks := []database.Track{
		{
			PersistentID: "TRACK001",
			Name:         "Blue in Green",
			Artist:       "Miles Davis",
			Album:        "Kind of Blue",
			Collection:   "Jazz Favorites",
			Playlists:    []string{"Jazz Favorites"},
			Genre:        "Jazz",
			Rating:       100,
			Starred:      true,
		},
		{
			PersistentID: "TRACK002",
			Name:         "So What",
			Artist:       "Miles Davis",
			Album:        "Kind of Blue",
			Collection:   "Jazz Favorites",
			Playlists:    []string{"Jazz Favorites"},
			Genre:        "Jazz",
			Rating:       90,
			Starred:      false,
		},
		{
			PersistentID: "TRACK003",
			Name:         "Bohemian Rhapsody",
			Artist:       "Queen",
			Album:        "A Night at the Opera",
			Collection:   "Rock Classics",
			Playlists:    []string{"Rock Classics"},
			Genre:        "Rock",
			Rating:       100,
			Starred:      true,
		},
		{
			PersistentID: "TRACK004",
			Name:         "Hotel California",
			Artist:       "Eagles",
			Album:        "Hotel California",
			Collection:   "Rock Classics",
			Playlists:    []string{"Rock Classics", "Chill Vibes"},
			Genre:        "Rock",
			Rating:       95,
			Starred:      true,
		},
		{
			PersistentID: "TRACK005",
			Name:         "Midnight City",
			Artist:       "M83",
			Album:        "Hurry Up, We're Dreaming",
			Collection:   "Chill Vibes",
			Playlists:    []string{"Chill Vibes"},
			Genre:        "Electronic",
			Rating:       85,
			Starred:      false,
		},
	}

	if err := dm.BatchInsertTracks(tracks); err != nil {
		return err
	}

	// Link tracks to playlists - need to get internal IDs first
	allPlaylists, err := dm.ListPlaylists()
	if err != nil {
		return err
	}

	playlistIDMap := make(map[string]int64)
	for _, p := range allPlaylists {
		playlistIDMap[p.PersistentID] = p.ID
	}

	// Get track internal IDs
	trackIDMap := make(map[string]int64)
	for _, track := range tracks {
		dbTrack, err := dm.GetTrackByPersistentID(track.PersistentID)
		if err != nil {
			return err
		}
		trackIDMap[track.PersistentID] = dbTrack.ID
	}

	// Link tracks to playlists
	playlistTracks := []struct {
		PlaylistPersistentID string
		TrackPersistentIDs   []string
	}{
		{"PLAYLIST001", []string{"TRACK001", "TRACK002"}},
		{"PLAYLIST002", []string{"TRACK003", "TRACK004"}},
		{"PLAYLIST003", []string{"TRACK004", "TRACK005"}},
	}

	for _, pt := range playlistTracks {
		playlistID := playlistIDMap[pt.PlaylistPersistentID]
		var trackIDs []int64
		for _, tpid := range pt.TrackPersistentIDs {
			trackIDs = append(trackIDs, trackIDMap[tpid])
		}
		if err := dm.BatchInsertPlaylistTracks(playlistID, trackIDs); err != nil {
			return err
		}
	}

	return nil
}

// Helper function to create CallToolRequest with arguments
func createRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// Helper function to extract text from CallToolResult
func extractTextFromResult(result *mcp.CallToolResult) (string, error) {
	if result.IsError {
		return "", fmt.Errorf("result is an error")
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("no content in result")
	}
	// Extract text from the first content item
	content := result.Content[0]
	// Use type assertion to get the text
	if textContent, ok := content.(mcp.TextContent); ok {
		return textContent.Text, nil
	}
	// Try to marshal and extract if it's a different type
	data, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	var contentMap map[string]interface{}
	if err := json.Unmarshal(data, &contentMap); err != nil {
		return "", err
	}
	if text, ok := contentMap["text"].(string); ok {
		return text, nil
	}
	return "", fmt.Errorf("could not extract text from content")
}

func TestSearchHandler(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		query         string
		expectResults bool
	}{
		{"Search for artist", "Miles", true},
		{"Search for album", "Blue", true},
		{"Search for track", "Bohemian", true},
		{"Search with no results", "NonexistentTrack", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createRequest(map[string]interface{}{
				"query": tt.query,
			})

			result, err := searchHandler(ctx, request)
			if err != nil {
				t.Fatalf("searchHandler returned error: %v", err)
			}

			if result.IsError {
				t.Errorf("Got unexpected error result")
				return
			}

			// Extract text content
			text, err := extractTextFromResult(result)
			if err != nil {
				t.Fatalf("Failed to extract text: %v", err)
			}

			// Parse the JSON response
			var tracks []itunes.Track
			if err := json.Unmarshal([]byte(text), &tracks); err != nil {
				t.Fatalf("Failed to parse tracks: %v", err)
			}

			if tt.expectResults && len(tracks) == 0 {
				t.Errorf("Expected results but got none")
			} else if !tt.expectResults && len(tracks) > 0 {
				t.Errorf("Expected no results but got %d", len(tracks))
			}

			// Verify track structure for results
			for _, track := range tracks {
				if track.ID == "" || track.Name == "" || track.Artist == "" {
					t.Errorf("Track missing required fields: %+v", track)
				}
			}
		})
	}
}

func TestSearchAdvancedHandler(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		params         map[string]interface{}
		checkCondition func([]itunes.Track) error
	}{
		{
			name: "Search with genre filter",
			params: map[string]interface{}{
				"query": "Miles",
				"genre": "Jazz",
			},
			checkCondition: func(tracks []itunes.Track) error {
				for _, track := range tracks {
					if track.Genre != "Jazz" {
						return fmt.Errorf("track %s has genre %s, expected Jazz", track.Name, track.Genre)
					}
				}
				return nil
			},
		},
		{
			name: "Search with rating filter",
			params: map[string]interface{}{
				"query":      "a",
				"min_rating": 95.0,
			},
			checkCondition: func(tracks []itunes.Track) error {
				for _, track := range tracks {
					if track.Rating < 95 {
						return fmt.Errorf("track %s has rating %d, expected >= 95", track.Name, track.Rating)
					}
				}
				return nil
			},
		},
		{
			name: "Search starred tracks only",
			params: map[string]interface{}{
				"query":   "a",
				"starred": true,
			},
			checkCondition: func(tracks []itunes.Track) error {
				for _, track := range tracks {
					if !track.Starred {
						return fmt.Errorf("track %s is not starred", track.Name)
					}
				}
				return nil
			},
		},
		{
			name: "Search with playlist filter",
			params: map[string]interface{}{
				"query":    "a",
				"playlist": "Rock Classics",
			},
			checkCondition: func(tracks []itunes.Track) error {
				for _, track := range tracks {
					found := false
					for _, pl := range track.Playlists {
						if pl == "Rock Classics" {
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("track %s not in Rock Classics playlist", track.Name)
					}
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := createRequest(tt.params)

			result, err := searchAdvancedHandler(ctx, request)
			if err != nil {
				t.Fatalf("searchAdvancedHandler returned error: %v", err)
			}

			if result.IsError {
				t.Errorf("Got error result")
				return
			}

			// Extract text content
			text, err := extractTextFromResult(result)
			if err != nil {
				t.Fatalf("Failed to extract text: %v", err)
			}

			// Parse the JSON response
			var tracks []itunes.Track
			if err := json.Unmarshal([]byte(text), &tracks); err != nil {
				t.Fatalf("Failed to parse tracks: %v", err)
			}

			if len(tracks) == 0 {
				t.Errorf("Expected results but got none")
			}

			// Check filter conditions
			if err := tt.checkCondition(tracks); err != nil {
				t.Errorf("Filter check failed: %v", err)
			}
		})
	}
}

func TestListPlaylistsHandler(t *testing.T) {
	ctx := context.Background()

	request := createRequest(map[string]interface{}{})

	result, err := listPlaylistsHandler(ctx, request)
	if err != nil {
		t.Fatalf("listPlaylistsHandler returned error: %v", err)
	}

	if result.IsError {
		t.Errorf("Got error result")
		return
	}

	// Extract text content
	text, err := extractTextFromResult(result)
	if err != nil {
		t.Fatalf("Failed to extract text: %v", err)
	}

	// Parse the JSON response
	var playlists []struct {
		Name         string `json:"name"`
		PersistentID string `json:"persistent_id"`
		TrackCount   int    `json:"track_count"`
		Genre        string `json:"genre,omitempty"`
	}

	if err := json.Unmarshal([]byte(text), &playlists); err != nil {
		t.Fatalf("Failed to parse playlists: %v", err)
	}

	if len(playlists) != 3 {
		t.Errorf("Expected 3 playlists, got %d", len(playlists))
	}

	// Verify playlist names
	expectedNames := map[string]bool{
		"Jazz Favorites": true,
		"Rock Classics":  true,
		"Chill Vibes":    true,
	}

	for _, p := range playlists {
		if !expectedNames[p.Name] {
			t.Errorf("Unexpected playlist: %s", p.Name)
		}
		if p.TrackCount != 2 {
			t.Errorf("Playlist %s: expected 2 tracks, got %d", p.Name, p.TrackCount)
		}
	}
}

func TestGetPlaylistTracksHandler(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		playlist  string
		useID     bool
		wantCount int
	}{
		{"Get tracks by playlist name", "Jazz Favorites", false, 2},
		{"Get tracks by playlist ID", "PLAYLIST002", true, 2},
		{"Playlist with multiple genres", "Chill Vibes", false, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"playlist": tt.playlist,
			}
			if tt.useID {
				args["use_id"] = tt.useID
			}

			request := createRequest(args)

			result, err := getPlaylistTracksHandler(ctx, request)
			if err != nil {
				t.Fatalf("getPlaylistTracksHandler returned error: %v", err)
			}

			if result.IsError {
				t.Errorf("Got error result")
				return
			}

			// Extract text content
			text, err := extractTextFromResult(result)
			if err != nil {
				t.Fatalf("Failed to extract text: %v", err)
			}

			// Parse the JSON response
			var tracks []itunes.Track
			if err := json.Unmarshal([]byte(text), &tracks); err != nil {
				t.Fatalf("Failed to parse tracks: %v", err)
			}

			if len(tracks) != tt.wantCount {
				t.Errorf("Expected %d tracks, got %d", tt.wantCount, len(tracks))
			}
		})
	}
}

func TestDatabaseResources(t *testing.T) {
	ctx := context.Background()

	t.Run("Database stats resource", func(t *testing.T) {
		request := mcp.ReadResourceRequest{
			Params: mcp.ReadResourceParams{
				URI: "itunes://database/stats",
			},
		}

		contents, err := dbStatsHandler(ctx, request)
		if err != nil {
			t.Fatalf("dbStatsHandler returned error: %v", err)
		}

		if len(contents) != 1 {
			t.Errorf("Expected 1 resource content, got %d", len(contents))
			return
		}

		textContent, ok := contents[0].(*mcp.TextResourceContents)
		if !ok {
			t.Errorf("Expected TextResourceContents, got %T", contents[0])
			return
		}

		var stats database.DatabaseStats
		if err := json.Unmarshal([]byte(textContent.Text), &stats); err != nil {
			t.Fatalf("Failed to parse stats: %v", err)
		}

		if stats.TrackCount != 5 {
			t.Errorf("Expected 5 tracks, got %d", stats.TrackCount)
		}

		if stats.PlaylistCount != 3 {
			t.Errorf("Expected 3 playlists, got %d", stats.PlaylistCount)
		}
	})

	t.Run("Playlists resource", func(t *testing.T) {
		request := mcp.ReadResourceRequest{
			Params: mcp.ReadResourceParams{
				URI: "itunes://database/playlists",
			},
		}

		contents, err := playlistsHandler(ctx, request)
		if err != nil {
			t.Fatalf("playlistsHandler returned error: %v", err)
		}

		if len(contents) != 1 {
			t.Errorf("Expected 1 resource content, got %d", len(contents))
			return
		}

		textContent, ok := contents[0].(*mcp.TextResourceContents)
		if !ok {
			t.Errorf("Expected TextResourceContents, got %T", contents[0])
			return
		}

		var playlists []database.Playlist
		if err := json.Unmarshal([]byte(textContent.Text), &playlists); err != nil {
			t.Fatalf("Failed to parse playlists: %v", err)
		}

		if len(playlists) != 3 {
			t.Errorf("Expected 3 playlists, got %d", len(playlists))
		}
	})
}
