package itunes

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	Playlists    []string `json:"playlists"`            // All playlists containing this track
	Genre        string   `json:"genre,omitempty"`      // Phase 2: Track genre
	Rating       int      `json:"rating,omitempty"`     // Phase 2: Track rating (0-100)
	Starred      bool     `json:"starred,omitempty"`    // Phase 2: Loved/starred status
	IsStreaming  bool     `json:"is_streaming"`         // Streaming track detection
	Kind         string   `json:"kind,omitempty"`       // Track type (e.g., "Internet audio stream")
	StreamURL    string   `json:"stream_url,omitempty"` // Stream URL for streaming tracks
}

// PlaylistData represents playlist metadata with persistent ID (Phase 2)
type PlaylistData struct {
	ID          string `json:"id"` // Persistent ID
	Name        string `json:"name"`
	SpecialKind string `json:"special_kind"` // "none" for user playlists
	TrackCount  int    `json:"track_count"`
	Genre       string `json:"genre,omitempty"`
}

// Station represents an Apple Music radio station
type Station struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Genre       string   `json:"genre"`
	Keywords    []string `json:"keywords"`
}

// StationSearchResult represents the result of a station search
type StationSearchResult struct {
	Status   string    `json:"status"`
	Query    string    `json:"query"`
	Stations []Station `json:"stations"`
	Count    int       `json:"count"`
	Message  string    `json:"message,omitempty"`
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
	IsStreaming     bool   `json:"is_streaming"`
	Kind            string `json:"kind,omitempty"`
	StreamURL       string `json:"stream_url,omitempty"`
}

// jsNowPlayingResponse represents the raw response from JavaScript
type jsNowPlayingResponse struct {
	Status  string           `json:"status"`
	Track   *NowPlayingTrack `json:"track,omitempty"`
	Display string           `json:"display"`
	Message string           `json:"message"`
}

// StreamingTrack contains streaming track information
type StreamingTrack struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StreamURL      string `json:"stream_url"`
	Kind           string `json:"kind"`
	Elapsed        string `json:"elapsed"`
	ElapsedSeconds int    `json:"elapsed_seconds"`
}

// NowPlayingStatus represents the current playback status
type NowPlayingStatus struct {
	Status  string           `json:"status"`           // "playing", "paused", "stopped", "error", "streaming", "streaming_paused"
	Track   *NowPlayingTrack `json:"track,omitempty"`  // For local tracks
	Stream  *StreamingTrack  `json:"stream,omitempty"` // For streaming tracks
	Display string           `json:"display"`          // Formatted display string
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

	// Provide user feedback before starting the long-running script
	fmt.Printf("🎵 Extracting music library from Apple Music app")
	fmt.Printf("\n   This may take 1-3 minutes for large libraries")

	cmd := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", tempFile.Name())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start refresh script: %w", err)
	}

	// Show progress dots while script is running
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Progress indicator
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err = <-done:
			fmt.Printf(" ✅\n")
			goto scriptComplete
		case <-ticker.C:
			fmt.Printf(".")
		}
	}

scriptComplete:
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

	// Parse the structured response using correct database structure
	var response database.RefreshResponse
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return fmt.Errorf("failed to parse refresh script response: %w", err)
	}

	if response.Status != "success" {
		return fmt.Errorf("refresh script failed: %s", response.Error)
	}

	// Write enhanced cache file (full structure for migration tool)
	enhancedJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal enhanced response: %w", err)
	}

	enhancedFile := filepath.Join(cacheDir, "library_enhanced.json")
	if err := os.WriteFile(enhancedFile, enhancedJSON, 0644); err != nil {
		return fmt.Errorf("failed to write enhanced cache file: %w", err)
	}

	// Write legacy cache file (tracks only) for backward compatibility
	if response.Data != nil && response.Data.Tracks != nil {
		// Convert JSONTrack to Track for legacy format
		legacyTracks := make([]Track, len(response.Data.Tracks))
		for i, jsonTrack := range response.Data.Tracks {
			legacyTracks[i] = Track{
				ID:           jsonTrack.PersistentID,
				PersistentID: jsonTrack.PersistentID,
				Name:         jsonTrack.Name,
				Album:        jsonTrack.Album,
				Collection:   jsonTrack.Collection,
				Artist:       jsonTrack.Artist,
				Playlists:    jsonTrack.Playlists,
				Genre:        jsonTrack.Genre,
				Rating:       jsonTrack.Rating,
				Starred:      jsonTrack.Starred,
				IsStreaming:  jsonTrack.IsStreaming,
				Kind:         jsonTrack.Kind,
				StreamURL:    jsonTrack.StreamURL,
			}
		}

		libraryJSON, err := json.Marshal(legacyTracks)
		if err != nil {
			return fmt.Errorf("failed to marshal legacy track data: %w", err)
		}

		cacheFile := filepath.Join(cacheDir, "library.json")
		if err := os.WriteFile(cacheFile, libraryJSON, 0644); err != nil {
			return fmt.Errorf("failed to write legacy cache file: %w", err)
		}
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

	// First parse the raw JavaScript response
	var jsResponse jsNowPlayingResponse
	if err := json.Unmarshal(responseJSON, &jsResponse); err != nil {
		return nil, fmt.Errorf("failed to parse now playing script response: %w", err)
	}

	// Convert to appropriate response structure
	status := &NowPlayingStatus{
		Display: jsResponse.Display,
		Message: jsResponse.Message,
	}

	// Handle different states based on track type
	if jsResponse.Track != nil && jsResponse.Track.IsStreaming {
		// Streaming track
		if jsResponse.Status == "playing" {
			status.Status = "streaming"
		} else if jsResponse.Status == "paused" {
			status.Status = "streaming_paused"
		} else {
			status.Status = jsResponse.Status
		}

		// Convert to StreamingTrack
		status.Stream = &StreamingTrack{
			ID:             jsResponse.Track.ID,
			Name:           jsResponse.Track.Name,
			StreamURL:      jsResponse.Track.StreamURL,
			Kind:           jsResponse.Track.Kind,
			Elapsed:        jsResponse.Track.Position,
			ElapsedSeconds: jsResponse.Track.PositionSeconds,
		}
	} else if jsResponse.Track != nil {
		// Local track
		status.Status = jsResponse.Status
		status.Track = jsResponse.Track
	} else {
		// No track (stopped, error, etc.)
		status.Status = jsResponse.Status
	}

	return status, nil
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
	if nowPlaying.Status == "streaming" && nowPlaying.Stream != nil {
		result.Message = fmt.Sprintf("Started streaming: %s", nowPlaying.Stream.Name)
	} else if nowPlaying.Status == "playing" && nowPlaying.Track != nil {
		if nowPlaying.Track.Artist != "" && nowPlaying.Track.Artist != "Unknown Artist" {
			result.Message = fmt.Sprintf("Now playing: %s by %s", nowPlaying.Track.Name, nowPlaying.Track.Artist)
		} else {
			result.Message = fmt.Sprintf("Now playing: %s", nowPlaying.Track.Name)
		}
	} else {
		result.Message = "Playback command sent successfully"
	}

	return result, nil
}

// PlayStreamURL plays a stream from an itmss:// or https://music.apple.com/ URL
func PlayStreamURL(streamURL string) (*PlayResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary file for the embedded script
	tempFile, err := os.CreateTemp("", "itunes_play_stream_*.js")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(playStreamScript); err != nil {
		return nil, fmt.Errorf("failed to write play stream script to temp file: %w", err)
	}
	tempFile.Close()

	cmd := exec.CommandContext(ctx, "osascript", tempFile.Name(), streamURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	response := strings.TrimSpace(stdout.String())

	result := &PlayResult{
		Success: err == nil && strings.HasPrefix(response, "OK:"),
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			result.Message = fmt.Sprintf("Stream playback failed: %s", response)
			return result, nil
		}
		result.Message = fmt.Sprintf("Script execution failed: %v", err)
		return result, nil
	}

	if strings.HasPrefix(response, "ERROR:") {
		result.Success = false
		result.Message = strings.TrimPrefix(response, "ERROR: ")
		return result, nil
	}

	// Give Apple Music a moment to start streaming
	time.Sleep(1 * time.Second)

	// Get current playing status
	nowPlaying, nowPlayingErr := GetNowPlaying()
	if nowPlayingErr != nil {
		// Don't fail the whole operation if we can't get now playing info
		result.Message = strings.TrimPrefix(response, "OK: ")
		return result, nil
	}

	result.NowPlaying = nowPlaying

	// Create a success message based on what's playing
	if nowPlaying.Status == "streaming" && nowPlaying.Stream != nil {
		result.Message = fmt.Sprintf("Started streaming: %s", nowPlaying.Stream.Name)
	} else {
		result.Message = strings.TrimPrefix(response, "OK: ")
	}

	return result, nil
}

// Database integration variables
var (
	dbManager     *database.DatabaseManager
	searchManager *database.SearchManager
	SearchLimit   = 15 // Default search limit, can be overridden by ITUNES_SEARCH_LIMIT env var
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
			IsStreaming:  dbTrack.IsStreaming,
			Kind:         dbTrack.Kind,
			StreamURL:    dbTrack.StreamURL,
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
		IsStreaming:  dbTrack.IsStreaming,
		Kind:         dbTrack.Kind,
		StreamURL:    dbTrack.StreamURL,
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
			IsStreaming:  dbTrack.IsStreaming,
			Kind:         dbTrack.Kind,
			StreamURL:    dbTrack.StreamURL,
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

// SearchTracks is the main search function using database
func SearchTracks(query string) ([]Track, error) {
	if dbManager == nil {
		return nil, errors.New("database not initialized - please run InitDatabase() first")
	}
	return SearchTracksFromDatabase(query, nil)
}

// SearchStations searches for Apple Music radio stations by scraping the web interface
func SearchStations(query string) (*StationSearchResult, error) {
	stations, err := scrapeAppleMusicStations()
	if err != nil {
		return nil, fmt.Errorf("failed to scrape stations: %w", err)
	}

	// Filter stations based on search query
	var matches []Station
	queryLower := strings.ToLower(query)

	for _, station := range stations {
		score := 0

		// Check name match
		if strings.Contains(strings.ToLower(station.Name), queryLower) {
			score += 10
		}

		// Check genre match
		if strings.Contains(strings.ToLower(station.Genre), queryLower) {
			score += 8
		}

		// Check keywords match
		for _, keyword := range station.Keywords {
			if strings.Contains(strings.ToLower(keyword), queryLower) {
				score += 5
				break
			}
		}

		// Check description match
		if strings.Contains(strings.ToLower(station.Description), queryLower) {
			score += 3
		}

		if score > 0 {
			matches = append(matches, station)
		}
	}

	return &StationSearchResult{
		Status:   "success",
		Query:    query,
		Stations: matches,
		Count:    len(matches),
	}, nil
}

// scrapeAppleMusicStations scrapes the Apple Music radio page to get current stations
func scrapeAppleMusicStations() ([]Station, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Get("https://music.apple.com/us/radio")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Apple Music radio page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)

	// Extract station URLs using regex
	urlRegex := regexp.MustCompile(`https://music\.apple\.com/us/station/([^/"]+)/ra\.([0-9]+)`)
	urlMatches := urlRegex.FindAllStringSubmatch(content, -1)

	// Extract station titles and descriptions
	titleRegex := regexp.MustCompile(`data-testid="title"[^>]*>([^<]+)`)
	titleMatches := titleRegex.FindAllStringSubmatch(content, -1)

	headlineRegex := regexp.MustCompile(`data-testid="headline"[^>]*>([^<]+)`)
	headlineMatches := headlineRegex.FindAllStringSubmatch(content, -1)

	subtitleRegex := regexp.MustCompile(`data-testid="subtitle"[^>]*>([^<]+)`)
	subtitleMatches := subtitleRegex.FindAllStringSubmatch(content, -1)

	var stations []Station

	// Process the extracted data
	for i, urlMatch := range urlMatches {
		if len(urlMatch) < 3 {
			continue
		}

		stationSlug := urlMatch[1]
		stationURL := urlMatch[0]

		// Clean up station name from slug
		stationName := strings.ReplaceAll(stationSlug, "-", " ")
		stationName = strings.Title(strings.ToLower(stationName))

		// Try to get title and description if available
		if i < len(titleMatches) && len(titleMatches[i]) > 1 {
			stationName = strings.TrimSpace(titleMatches[i][1])
		}

		description := ""
		if i < len(subtitleMatches) && len(subtitleMatches[i]) > 1 {
			description = strings.TrimSpace(subtitleMatches[i][1])
		}

		genre := "radio"
		if i < len(headlineMatches) && len(headlineMatches[i]) > 1 {
			headline := strings.TrimSpace(headlineMatches[i][1])
			if strings.Contains(strings.ToLower(headline), "country") {
				genre = "country"
			} else if strings.Contains(strings.ToLower(headline), "jazz") {
				genre = "jazz"
			} else if strings.Contains(strings.ToLower(headline), "rock") {
				genre = "rock"
			} else if strings.Contains(strings.ToLower(headline), "hip") {
				genre = "hip-hop"
			} else if strings.Contains(strings.ToLower(headline), "electronic") {
				genre = "electronic"
			} else if strings.Contains(strings.ToLower(headline), "pop") {
				genre = "pop"
			} else if strings.Contains(strings.ToLower(headline), "classical") {
				genre = "classical"
			}
		}

		// Generate keywords from name and description
		keywords := []string{
			strings.ToLower(stationName),
			genre,
			"radio",
			"station",
		}

		// Avoid duplicate stations
		exists := false
		for _, existing := range stations {
			if existing.URL == stationURL {
				exists = true
				break
			}
		}

		if !exists {
			stations = append(stations, Station{
				Name:        stationName,
				Description: description,
				URL:         stationURL,
				Genre:       genre,
				Keywords:    keywords,
			})
		}
	}

	return stations, nil
}
