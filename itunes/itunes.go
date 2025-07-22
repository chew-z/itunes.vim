package itunes

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"itunes/database"
)

// Exported error variables for better error handling
var (
	ErrNoTracksFound = errors.New("no tracks found")
	ErrScriptFailed  = errors.New("JXA script execution failed")
)

// Track describes one track from the script's output
type Track struct {
	ID           string   `json:"id"`
	PersistentID string   `json:"persistent_id,omitempty"` // Apple Music persistent ID (Phase 2)
	Name         string   `json:"name"`
	Album        string   `json:"album"`
	Collection   string   `json:"collection"` // Primary playlist name or album if not in a playlist
	Artist       string   `json:"artist"`
	Playlists    []string `json:"playlists"`         // All playlists containing this track
	Genre        string   `json:"genre,omitempty"`   // Phase 2: Track genre
	Rating       int      `json:"rating,omitempty"`  // Phase 2: Track rating (0-100)
	Starred      bool     `json:"starred,omitempty"` // Phase 2: Loved/starred status
}

// PlaylistData represents playlist metadata with persistent ID (Phase 2)
type PlaylistData struct {
	ID          string `json:"id"` // Persistent ID
	Name        string `json:"name"`
	SpecialKind string `json:"special_kind"` // "none" for user playlists
	TrackCount  int    `json:"track_count"`
	Genre       string `json:"genre,omitempty"`
}

// RefreshStats contains statistics from a library refresh operation
type RefreshStats struct {
	TotalTracks    int `json:"total_tracks"`
	TotalPlaylists int `json:"total_playlists"`
	ProcessingTime int `json:"processing_time_ms"`
}

// RefreshData contains the tracks and playlists from a refresh operation
type RefreshData struct {
	Tracks    []Track        `json:"tracks"`
	Playlists []PlaylistData `json:"playlists"`
	Stats     RefreshStats   `json:"stats"`
}

// RefreshResponse represents the complete response from the refresh script
type RefreshResponse struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    *RefreshData           `json:"data"`
	Error   string                 `json:"error,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// NowPlayingTrack contains current track information with playback details
type NowPlayingTrack struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Artist          string `json:"artist"`
	Album           string `json:"album"`
	Position        string `json:"position"`
	Duration        string `json:"duration"`
	PositionSeconds int    `json:"position_seconds"`
	DurationSeconds int    `json:"duration_seconds"`
}

// NowPlayingStatus represents the current playback status
type NowPlayingStatus struct {
	Status  string           `json:"status"` // "playing", "paused", "stopped", "error"
	Track   *NowPlayingTrack `json:"track,omitempty"`
	Display string           `json:"display"` // Formatted display string
	Message string           `json:"message"`
}

// PlayResult contains the result of a play operation with current track info
type PlayResult struct {
	Success    bool              `json:"success"`
	Message    string            `json:"message"`
	NowPlaying *NowPlayingStatus `json:"now_playing,omitempty"`
}

// PlayPlaylistTrack runs the embedded iTunes_Play_Playlist_Track.js script to play a playlist, album, or track.
// If trackName is "", only the playlist/album will play. If trackID is provided, it takes priority over trackName.
// Either playlistName or albumName can be provided for context, but not both.
func PlayPlaylistTrack(playlistName, albumName, trackName, trackID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a temporary file with the embedded script
	tempFile, err := os.CreateTemp("", "itunes_play_*.js")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write the embedded script to the temp file
	if _, err := tempFile.WriteString(playScript); err != nil {
		return fmt.Errorf("failed to write script to temp file: %w", err)
	}
	tempFile.Close()

	args := []string{"-l", "JavaScript", tempFile.Name()}

	// Always pass playlist name (empty string if not provided)
	args = append(args, playlistName)

	// Always pass album name (empty string if not provided)
	args = append(args, albumName)

	// Always pass track name (empty string if not provided)
	args = append(args, trackName)

	// Always pass track ID (empty string if not provided) - script prioritizes this
	args = append(args, trackID)

	cmd := exec.CommandContext(ctx, "osascript", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", ErrScriptFailed, stderr.String())
	}

	// Parse the new structured response format
	response := strings.TrimSpace(stdout.String())
	if response == "" {
		return errors.New("play script returned no output")
	}

	// Check for structured error response
	if strings.HasPrefix(response, "ERROR:") {
		errorMsg := strings.TrimPrefix(response, "ERROR:")
		errorMsg = strings.TrimSpace(errorMsg)
		return fmt.Errorf("play script error: %s", errorMsg)
	}

	// Check for success response
	if strings.HasPrefix(response, "OK:") {
		// Success - no error
		return nil
	}

	// Fallback: try to parse old JSON format for backward compatibility
	var jsonResponse struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &jsonResponse); err == nil {
		if jsonResponse.Status == "error" {
			return fmt.Errorf("play script error: %s", jsonResponse.Message)
		}
		return nil
	}

	// Unknown response format
	return fmt.Errorf("unexpected play script response: %s", response)
}

// SearchTracksFromCache searches the iTunes library cache directly without using JavaScript.
// This is much faster than SearchiTunesPlaylists as it eliminates the osascript overhead.
func SearchTracksFromCache(query string) ([]Track, error) {
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}

	// Read the library cache file
	cacheFile := filepath.Join(os.TempDir(), "itunes-cache", "library.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("library cache not found - please run refresh_library first")
		}
		return nil, fmt.Errorf("failed to read library cache: %w", err)
	}

	var allTracks []Track
	if err := json.Unmarshal(data, &allTracks); err != nil {
		return nil, fmt.Errorf("failed to parse library cache: %w", err)
	}

	// Perform search with same logic as iTunes_Search2_fzf.js
	var exactMatches []Track
	var partialMatches []Track
	queryLower := strings.ToLower(strings.TrimSpace(query))

	for _, track := range allTracks {
		trackName := strings.ToLower(track.Name)
		artistName := strings.ToLower(track.Artist)
		albumName := strings.ToLower(track.Album)
		collectionName := strings.ToLower(track.Collection)

		// Check for exact matches first (higher priority)
		if trackName == queryLower || artistName == queryLower {
			exactMatches = append(exactMatches, track)
		} else {
			// Check partial matches in all searchable fields including track ID and playlists
			trackID := strings.ToLower(track.ID)
			playlistsStr := strings.ToLower(strings.Join(track.Playlists, " "))
			searchableText := strings.Join([]string{collectionName, trackName, artistName, albumName, trackID, playlistsStr}, " ")
			if strings.Contains(searchableText, queryLower) {
				partialMatches = append(partialMatches, track)
			}
		}
	}

	// Combine results with exact matches first, limit to 15 total
	matches := exactMatches
	if len(matches) < 15 {
		remaining := 15 - len(matches)
		if remaining > len(partialMatches) {
			remaining = len(partialMatches)
		}
		matches = append(matches, partialMatches[:remaining]...)
	}

	if len(matches) == 0 {
		return nil, ErrNoTracksFound
	}

	return matches, nil
}

// RefreshLibraryCache runs the embedded iTunes_Refresh_Library.js script to build a comprehensive library cache.
// The cache is stored as JSON in $TMPDIR/itunes-cache/library.json for fast searching.
func RefreshLibraryCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second) // 3 minutes for full library scan
	defer cancel()

	// Ensure cache directory exists
	cacheDir := filepath.Join(os.TempDir(), "itunes-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create a temporary file with the embedded refresh script
	tempFile, err := os.CreateTemp("", "itunes_refresh_*.js")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write the embedded refresh script to the temp file
	if _, err := tempFile.WriteString(refreshScript); err != nil {
		return fmt.Errorf("failed to write refresh script to temp file: %w", err)
	}
	tempFile.Close()

	cmd := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", tempFile.Name())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				return errors.New("no tracks found in library")
			}
		}
		return fmt.Errorf("%w: %s", ErrScriptFailed, stderr.String())
	}

	// Get the JSON output from the refresh script
	responseJSON := stdout.Bytes()
	if len(responseJSON) == 0 {
		return errors.New("refresh script returned no data")
	}

	// Parse the structured response
	var response struct {
		Status  string  `json:"status"`
		Data    []Track `json:"data"`
		Message string  `json:"message"`
		Error   string  `json:"error"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return fmt.Errorf("failed to parse refresh script response: %w", err)
	}

	if response.Status != "success" {
		return fmt.Errorf("refresh script failed: %s", response.Message)
	}

	// Convert track data back to JSON for cache file
	libraryJSON, err := json.Marshal(response.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal track data: %w", err)
	}

	// Write the library data to cache file
	cacheFile := filepath.Join(cacheDir, "library.json")
	if err := os.WriteFile(cacheFile, libraryJSON, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// GetNowPlaying runs the embedded iTunes_Now_Playing.js script to get current playback status
func GetNowPlaying() (*NowPlayingStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create temporary file for the embedded script
	tempFile, err := os.CreateTemp("", "itunes_now_playing_*.js")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(nowPlayingScript); err != nil {
		return nil, fmt.Errorf("failed to write now playing script to temp file: %w", err)
	}
	tempFile.Close()

	cmd := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", tempFile.Name())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrScriptFailed, stderr.String())
	}

	// Parse the JSON response
	responseJSON := stdout.Bytes()
	if len(responseJSON) == 0 {
		return nil, errors.New("now playing script returned no data")
	}

	var status NowPlayingStatus
	if err := json.Unmarshal(responseJSON, &status); err != nil {
		return nil, fmt.Errorf("failed to parse now playing script response: %w", err)
	}

	return &status, nil
}

// PlayPlaylistTrackWithStatus runs PlayPlaylistTrack and returns the result with current track info
func PlayPlaylistTrackWithStatus(playlistName, albumName, trackName, trackID string) (*PlayResult, error) {
	// First, attempt to play the track
	err := PlayPlaylistTrack(playlistName, albumName, trackName, trackID)

	result := &PlayResult{
		Success: err == nil,
	}

	if err != nil {
		result.Message = fmt.Sprintf("Playback failed: %v", err)
		return result, nil
	}

	// Give Apple Music a moment to start playing
	time.Sleep(1 * time.Second)

	// Get current playing status
	nowPlaying, nowPlayingErr := GetNowPlaying()
	if nowPlayingErr != nil {
		// Don't fail the whole operation if we can't get now playing info
		result.Message = "Playback started, but could not get current track info"
		return result, nil
	}

	result.NowPlaying = nowPlaying

	// Create a success message based on what's playing
	if nowPlaying.Status == "playing" && nowPlaying.Track != nil {
		result.Message = fmt.Sprintf("Now playing: %s", nowPlaying.Display)
	} else {
		result.Message = "Playback command sent successfully"
	}

	return result, nil
}

// Database integration variables
var (
	dbManager     *database.DatabaseManager
	searchManager *database.SearchManager
	UseDatabase   = true // Database mode is now default
	SearchLimit   = 15   // Default search limit, can be overridden by ITUNES_SEARCH_LIMIT env var
)

// InitDatabase initializes the SQLite database connection
func InitDatabase() error {
	// Get search limit from environment if set
	if limitStr := os.Getenv("ITUNES_SEARCH_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			SearchLimit = limit
		}
	}

	// Initialize database manager
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run migrations to ensure schema is up to date
	if err := dm.RunMigrations(); err != nil {
		dm.Close()
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	dbManager = dm
	searchManager = database.NewSearchManager(dm)
	return nil
}

// CloseDatabase closes the database connection
func CloseDatabase() {
	if dbManager != nil {
		dbManager.Close()
		dbManager = nil
		searchManager = nil
	}
}

// SearchTracksFromDatabase searches tracks using the SQLite database with FTS5
func SearchTracksFromDatabase(query string, filters *database.SearchFilters) ([]Track, error) {
	if dbManager == nil {
		return nil, errors.New("database not initialized")
	}

	// Apply default search limit if not specified in filters
	if filters == nil {
		filters = &database.SearchFilters{Limit: SearchLimit}
	} else if filters.Limit <= 0 {
		filters.Limit = SearchLimit
	}

	// Use search manager for cached search
	dbTracks, err := searchManager.SearchWithCache(query, filters)
	if err != nil {
		return nil, fmt.Errorf("database search failed: %w", err)
	}

	// Convert database tracks to API tracks
	tracks := make([]Track, len(dbTracks))
	for i, dbTrack := range dbTracks {
		tracks[i] = Track{
			ID:           dbTrack.PersistentID, // Use persistent ID as the main ID
			PersistentID: dbTrack.PersistentID,
			Name:         dbTrack.Name,
			Album:        dbTrack.Album,
			Collection:   dbTrack.Collection,
			Artist:       dbTrack.Artist,
			Playlists:    dbTrack.Playlists,
			Genre:        dbTrack.Genre,
			Rating:       dbTrack.Rating,
			Starred:      dbTrack.Starred,
		}
	}

	return tracks, nil
}

// GetTrackByPersistentID retrieves a single track by its persistent ID
func GetTrackByPersistentID(persistentID string) (*Track, error) {
	if dbManager == nil {
		return nil, errors.New("database not initialized")
	}

	dbTrack, err := dbManager.GetTrackByPersistentID(persistentID)
	if err != nil {
		return nil, err
	}

	track := &Track{
		ID:           dbTrack.PersistentID,
		PersistentID: dbTrack.PersistentID,
		Name:         dbTrack.Name,
		Album:        dbTrack.Album,
		Collection:   dbTrack.Collection,
		Artist:       dbTrack.Artist,
		Playlists:    dbTrack.Playlists,
		Genre:        dbTrack.Genre,
		Rating:       dbTrack.Rating,
		Starred:      dbTrack.Starred,
	}

	return track, nil
}

// GetPlaylistTracks retrieves all tracks in a playlist
func GetPlaylistTracks(playlistIdentifier string, usePlaylistID bool) ([]Track, error) {
	if dbManager == nil {
		return nil, errors.New("database not initialized")
	}

	dbTracks, err := dbManager.GetPlaylistTracks(playlistIdentifier, usePlaylistID)
	if err != nil {
		return nil, err
	}

	tracks := make([]Track, len(dbTracks))
	for i, dbTrack := range dbTracks {
		tracks[i] = Track{
			ID:           dbTrack.PersistentID,
			PersistentID: dbTrack.PersistentID,
			Name:         dbTrack.Name,
			Album:        dbTrack.Album,
			Collection:   dbTrack.Collection,
			Artist:       dbTrack.Artist,
			Playlists:    dbTrack.Playlists,
			Genre:        dbTrack.Genre,
			Rating:       dbTrack.Rating,
			Starred:      dbTrack.Starred,
		}
	}

	return tracks, nil
}

// GetDatabaseStats returns database statistics
func GetDatabaseStats() (*database.DatabaseStats, error) {
	if dbManager == nil {
		return nil, errors.New("database not initialized")
	}
	return dbManager.GetStats()
}

// ListPlaylists returns all user playlists from the database
func ListPlaylists() ([]database.Playlist, error) {
	if dbManager == nil {
		return nil, errors.New("database not initialized")
	}
	return dbManager.ListPlaylists()
}

// SearchTracks is the main search function that uses database by default
func SearchTracks(query string) ([]Track, error) {
	if UseDatabase && dbManager != nil {
		return SearchTracksFromDatabase(query, nil)
	}
	// Fallback to cache-based search (will be deprecated)
	return SearchTracksFromCache(query)
}
