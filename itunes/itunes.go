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

	"go.uber.org/zap"
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
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Genre       string   `json:"genre"`
	Homepage    string   `json:"homepage,omitempty"` // https:// web URL for browser access
	Keywords    []string `json:"keywords"`           // For backward compatibility
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

// EQStatus represents the state of the Apple Music Equalizer.
type EQStatus struct {
	Enabled          bool     `json:"enabled"`
	CurrentPreset    *string  `json:"current_preset"` // Use pointer to handle null when disabled
	AvailablePresets []string `json:"available_presets"`
	Message          string   `json:"message,omitempty"` // Optional informational message
}

// AudioOutput describes the current audio output device.
type AudioOutput struct {
	OutputType string `json:"output_type"` // "local" or "airplay"
	DeviceName string `json:"device_name"`
	Error      string `json:"error,omitempty"`
}

// AirPlayDevice represents an AirPlay-capable audio output device.
type AirPlayDevice struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Selected    bool   `json:"selected"`
	SoundVolume int    `json:"sound_volume"`
	Error       string `json:"error,omitempty"`
}

// runScript executes a JXA script and returns its standard output.
func runScript(ctx context.Context, scriptContent string, args []string) ([]byte, error) {
	tempFile, err := os.CreateTemp("", "itunes_*.js")
	if err != nil {
		return nil, &ITunesError{Op: "runScript: create temp file", Kind: ErrJXAScript, Err: err}
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(scriptContent); err != nil {
		tempFile.Close()
		return nil, &ITunesError{Op: "runScript: write to temp file", Kind: ErrJXAScript, Err: err}
	}
	tempFile.Close()

	cmdArgs := []string{"-l", "JavaScript", tempFile.Name()}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}

	cmd := exec.CommandContext(ctx, "osascript", cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, &ITunesError{
			Op:   "runScript: execute osascript",
			Kind: ErrJXAScript,
			Err:  fmt.Errorf("script failed with stderr: %s", stderr.String()),
			Context: map[string]interface{}{
				"script": filepath.Base(tempFile.Name()),
				"args":   args,
			},
		}
	}

	return stdout.Bytes(), nil
}

// GetEQStatus retrieves the current equalizer status from Apple Music.
func GetEQStatus() (*EQStatus, error) {
	// First, check if audio is being routed through AirPlay.
	audioOutput, err := GetAudioOutput()
	if err != nil {
		return nil, &ITunesError{Op: "GetEQStatus: get audio output", Kind: ErrAppleMusic, Err: err}
	}

	if audioOutput.OutputType == "airplay" {
		return nil, &ITunesError{
			Op:   "GetEQStatus",
			Kind: ErrAppleMusic,
			Err:  fmt.Errorf("EQ status cannot be checked while playing to an AirPlay device ('%s')", audioOutput.DeviceName),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	output, err := runScript(ctx, getEQScript, nil)
	if err != nil {
		return nil, &ITunesError{Op: "GetEQStatus: run script", Kind: ErrJXAScript, Err: err}
	}

	var status EQStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, &ITunesError{Op: "GetEQStatus: unmarshal json", Kind: ErrJXAScript, Err: err}
	}

	return &status, nil
}

// GetAudioOutput retrieves the current audio output device information.
func GetAudioOutput() (*AudioOutput, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	output, err := runScript(ctx, getAudioOutputScript, nil)
	if err != nil {
		return nil, &ITunesError{Op: "GetAudioOutput: run script", Kind: ErrJXAScript, Err: err}
	}

	var audioOutput AudioOutput
	if err := json.Unmarshal(output, &audioOutput); err != nil {
		return nil, &ITunesError{Op: "GetAudioOutput: unmarshal json", Kind: ErrJXAScript, Err: err}
	}

	if audioOutput.Error != "" {
		return nil, &ITunesError{
			Op:   "GetAudioOutput: script error",
			Kind: ErrJXAScript,
			Err:  fmt.Errorf("%s", audioOutput.Error),
		}
	}

	return &audioOutput, nil
}

// SetEQStatus sets the equalizer state in Apple Music.
// It can enable/disable the EQ and/or set a specific preset.
func SetEQStatus(preset string, enabled *bool) (*EQStatus, error) {
	// First, check if audio is being routed through AirPlay.
	audioOutput, err := GetAudioOutput()
	if err != nil {
		return nil, &ITunesError{Op: "SetEQStatus: get audio output", Kind: ErrAppleMusic, Err: err}
	}

	if audioOutput.OutputType == "airplay" {
		// Get current EQ status instead of returning an error
		currentStatus, err := GetEQStatus()
		if err != nil {
			return nil, &ITunesError{
				Op:   "SetEQStatus: get current EQ status for AirPlay",
				Kind: ErrAppleMusic,
				Err:  err,
				Context: map[string]interface{}{
					"airplay_device": audioOutput.DeviceName,
				},
			}
		}

		// Return current status with informative message about AirPlay limitation
		return &EQStatus{
			Enabled:          currentStatus.Enabled,
			CurrentPreset:    currentStatus.CurrentPreset,
			AvailablePresets: currentStatus.AvailablePresets,
			Message: fmt.Sprintf("EQ settings cannot be changed while using AirPlay device '%s'. Current EQ status: %s", audioOutput.DeviceName, func() string {
				if currentStatus.Enabled && currentStatus.CurrentPreset != nil && *currentStatus.CurrentPreset != "" {
					return fmt.Sprintf("enabled with '%s' preset", *currentStatus.CurrentPreset)
				} else if currentStatus.Enabled {
					return "enabled with no preset"
				}
				return "disabled"
			}()),
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var args []string
	if enabled != nil {
		args = append(args, "--enabled", strconv.FormatBool(*enabled))
	}
	if preset != "" {
		args = append(args, "--preset", preset)
	}

	output, err := runScript(ctx, setEQScript, args)
	if err != nil {
		return nil, &ITunesError{Op: "SetEQStatus: run script", Kind: ErrJXAScript, Err: err}
	}

	var status EQStatus
	if err := json.Unmarshal(output, &status); err != nil {
		// Check for a script-level error response
		var scriptError struct {
			Error      string `json:"error"`
			PresetName string `json:"preset_name"`
		}
		if json.Unmarshal(output, &scriptError) == nil && scriptError.Error != "" {
			return nil, &ITunesError{
				Op:   "SetEQStatus: script error",
				Kind: ErrJXAScript,
				Err:  fmt.Errorf("script error: %s (preset: %s)", scriptError.Error, scriptError.PresetName),
			}
		}
		return nil, &ITunesError{Op: "SetEQStatus: unmarshal json", Kind: ErrJXAScript, Err: err}
	}

	return &status, nil
}

// ListAirPlayDevices retrieves a list of all available AirPlay devices.
func ListAirPlayDevices() ([]AirPlayDevice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	output, err := runScript(ctx, listAirPlayDevicesScript, nil)
	if err != nil {
		return nil, &ITunesError{Op: "ListAirPlayDevices: run script", Kind: ErrJXAScript, Err: err}
	}

	var devices []AirPlayDevice
	if err := json.Unmarshal(output, &devices); err != nil {
		// Check for a script-level error response
		var scriptError struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(output, &scriptError) == nil && scriptError.Error != "" {
			return nil, &ITunesError{
				Op:   "ListAirPlayDevices: script error",
				Kind: ErrJXAScript,
				Err:  fmt.Errorf("%s", scriptError.Error),
			}
		}
		return nil, &ITunesError{Op: "ListAirPlayDevices: unmarshal json", Kind: ErrJXAScript, Err: err}
	}

	return devices, nil
}

// SetAirPlayDevice sets the active AirPlay device.
func SetAirPlayDevice(deviceName string) (*AirPlayDevice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	output, err := runScript(ctx, setAirPlayDeviceScript, []string{deviceName})
	if err != nil {
		return nil, &ITunesError{Op: "SetAirPlayDevice: run script", Kind: ErrJXAScript, Err: err}
	}

	var device AirPlayDevice
	if err := json.Unmarshal(output, &device); err != nil {
		return nil, &ITunesError{Op: "SetAirPlayDevice: unmarshal json", Kind: ErrJXAScript, Err: err}
	}

	if device.Error != "" {
		return nil, &ITunesError{
			Op:   "SetAirPlayDevice: script error",
			Kind: ErrJXAScript,
			Err:  fmt.Errorf("%s", device.Error),
			Context: map[string]interface{}{
				"device_name": deviceName,
			},
		}
	}

	return &device, nil
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
		return &ITunesError{Op: "PlayPlaylistTrack: create temp file", Kind: ErrJXAScript, Err: err}
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write the embedded script to the temp file
	if _, err := tempFile.WriteString(playScript); err != nil {
		return &ITunesError{Op: "PlayPlaylistTrack: write to temp file", Kind: ErrJXAScript, Err: err}
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
		return &ITunesError{
			Op:   "PlayPlaylistTrack: run script",
			Kind: ErrJXAScript,
			Err:  fmt.Errorf("script failed with stderr: %s", stderr.String()),
			Context: map[string]interface{}{
				"playlist": playlistName,
				"album":    albumName,
				"track":    trackName,
				"track_id": trackID,
			},
		}
	}

	// Parse the new structured response format
	response := strings.TrimSpace(stdout.String())
	if response == "" {
		return &ITunesError{Op: "PlayPlaylistTrack: empty response", Kind: ErrJXAScript, Err: errors.New("play script returned no output")}
	}

	// Check for structured error response
	if strings.HasPrefix(response, "ERROR:") {
		errorMsg := strings.TrimPrefix(response, "ERROR:")
		errorMsg = strings.TrimSpace(errorMsg)
		return &ITunesError{Op: "PlayPlaylistTrack: script error", Kind: ErrJXAScript, Err: fmt.Errorf("%s", errorMsg)}
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
			return &ITunesError{Op: "PlayPlaylistTrack: script error (json)", Kind: ErrJXAScript, Err: fmt.Errorf("%s", jsonResponse.Message)}
		}
		return nil
	}

	// Unknown response format
	return &ITunesError{Op: "PlayPlaylistTrack: unknown response", Kind: ErrJXAScript, Err: fmt.Errorf("unexpected play script response: %s", response)}
}

// RefreshLibraryCache runs the embedded iTunes_Refresh_Library.js script to build a comprehensive library cache.
// The cache is stored as JSON in $TMPDIR/itunes-cache/library.json for fast searching.
func RefreshLibraryCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second) // 3 minutes for full library scan
	defer cancel()

	// Ensure cache directory exists
	cacheDir := filepath.Join(os.TempDir(), "itunes-cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return &ITunesError{Op: "RefreshLibraryCache: create cache dir", Kind: ErrJXAScript, Err: err}
	}

	// Create a temporary file with the embedded refresh script
	tempFile, err := os.CreateTemp("", "itunes_refresh_*.js")
	if err != nil {
		return &ITunesError{Op: "RefreshLibraryCache: create temp file", Kind: ErrJXAScript, Err: err}
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write the embedded refresh script to the temp file
	if _, err := tempFile.WriteString(refreshScript); err != nil {
		return &ITunesError{Op: "RefreshLibraryCache: write to temp file", Kind: ErrJXAScript, Err: err}
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
		return &ITunesError{Op: "RefreshLibraryCache: start script", Kind: ErrJXAScript, Err: err}
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
				return &ITunesError{Op: "RefreshLibraryCache", Kind: ErrNoTracksFound, Err: errors.New("no tracks found in library")}
			}
		}
		return &ITunesError{
			Op:   "RefreshLibraryCache: run script",
			Kind: ErrJXAScript,
			Err:  fmt.Errorf("script failed with stderr: %s", stderr.String()),
		}
	}

	// Get the JSON output from the refresh script
	responseJSON := stdout.Bytes()
	if len(responseJSON) == 0 {
		return &ITunesError{Op: "RefreshLibraryCache: empty response", Kind: ErrJXAScript, Err: errors.New("refresh script returned no data")}
	}

	// Parse the structured response using correct database structure
	var response database.RefreshResponse
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return &ITunesError{Op: "RefreshLibraryCache: unmarshal json", Kind: ErrJXAScript, Err: err}
	}

	if response.Status != "success" {
		return &ITunesError{Op: "RefreshLibraryCache: script error", Kind: ErrJXAScript, Err: fmt.Errorf("%s", response.Error)}
	}

	// Write enhanced cache file (full structure for migration tool)
	enhancedJSON, err := json.Marshal(response)
	if err != nil {
		return &ITunesError{Op: "RefreshLibraryCache: marshal enhanced json", Kind: ErrJXAScript, Err: err}
	}

	enhancedFile := filepath.Join(cacheDir, "library_enhanced.json")
	if err := os.WriteFile(enhancedFile, enhancedJSON, 0644); err != nil {
		return &ITunesError{Op: "RefreshLibraryCache: write enhanced cache", Kind: ErrJXAScript, Err: err}
	}

	// Write legacy cache file (tracks only) for backward compatibility
	if response.Data != nil && response.Data.Tracks != nil {
		// Convert JSONTrack to Track for legacy format
		legacyTracks := make([]Track, len(response.Data.Tracks))
		for i, jsonTrack := range response.Data.Tracks {
			legacyTracks[i] = Track{
				ID:           jsonTrack.PersistentID, // Use persistent ID as the main ID
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
			return &ITunesError{Op: "RefreshLibraryCache: marshal legacy json", Kind: ErrJXAScript, Err: err}
		}

		cacheFile := filepath.Join(cacheDir, "library.json")
		if err := os.WriteFile(cacheFile, libraryJSON, 0644); err != nil {
			return &ITunesError{Op: "RefreshLibraryCache: write legacy cache", Kind: ErrJXAScript, Err: err}
		}
	}

	return nil
}

// GetNowPlaying runs the embedded iTunes_Now_Playing.js script to get current playback status
func GetNowPlaying() (*NowPlayingStatus, error) {
	var status *NowPlayingStatus
	var err error

	err = retryWithBackoff(func() error {
		status, err = getNowPlayingAttempt()
		return err
	}, 3)

	if err != nil {
		return nil, err
	}

	return status, nil
}

func getNowPlayingAttempt() (*NowPlayingStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create temporary file for the embedded script
	tempFile, err := os.CreateTemp("", "itunes_now_playing_*.js")
	if err != nil {
		return nil, &ITunesError{Op: "GetNowPlaying: create temp file", Kind: ErrJXAScript, Err: err}
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(nowPlayingScript); err != nil {
		return nil, &ITunesError{Op: "GetNowPlaying: write to temp file", Kind: ErrJXAScript, Err: err}
	}
	tempFile.Close()

	cmd := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", tempFile.Name())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return nil, &ITunesError{
			Op:   "GetNowPlaying: run script",
			Kind: ErrJXAScript,
			Err:  fmt.Errorf("script failed with stderr: %s", stderr.String()),
		}
	}

	// Parse the JSON response
	responseJSON := stdout.Bytes()
	if len(responseJSON) == 0 {
		return nil, &ITunesError{Op: "GetNowPlaying: empty response", Kind: ErrJXAScript, Err: errors.New("now playing script returned no data")}
	}

	// First parse the raw JavaScript response
	var jsResponse jsNowPlayingResponse
	if err := json.Unmarshal(responseJSON, &jsResponse); err != nil {
		return nil, &ITunesError{Op: "GetNowPlaying: unmarshal json", Kind: ErrJXAScript, Err: err}
	}

	// Convert to appropriate response structure
	status := &NowPlayingStatus{
		Display: jsResponse.Display,
		Message: jsResponse.Message,
	}

	// Handle different states based on track type
	if jsResponse.Track != nil && jsResponse.Track.IsStreaming {
		// Streaming track
		switch jsResponse.Status {
		case "playing":
			status.Status = "streaming"
		case "paused":
			status.Status = "streaming_paused"
		default:
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
		return result, &ITunesError{Op: "PlayPlaylistTrackWithStatus: playback failed", Kind: ErrAppleMusic, Err: err}
	}

	// Give Apple Music a moment to start playing
	time.Sleep(1 * time.Second)

	// Get current playing status
	nowPlaying, nowPlayingErr := GetNowPlaying()
	if nowPlayingErr != nil {
		// Don't fail the whole operation if we can't get now playing info
		result.Message = "Playback started, but could not get current track info"
		return result, &ITunesError{Op: "PlayPlaylistTrackWithStatus: get now playing", Kind: ErrAppleMusic, Err: nowPlayingErr}
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

// PlayStreamURL plays a stream from any supported streaming URL (itmss://, https://, http://, etc.)
func PlayStreamURL(streamURL string) (*PlayResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary file for the embedded script
	tempFile, err := os.CreateTemp("", "itunes_play_stream_*.js")
	if err != nil {
		return nil, &ITunesError{Op: "PlayStreamURL: create temp file", Kind: ErrJXAScript, Err: err}
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(playStreamScript); err != nil {
		return nil, &ITunesError{Op: "PlayStreamURL: write to temp file", Kind: ErrJXAScript, Err: err}
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
			return result, &ITunesError{Op: "PlayStreamURL: playback failed", Kind: ErrAppleMusic, Err: fmt.Errorf("%s", result.Message)}
		}
		result.Message = fmt.Sprintf("Script execution failed: %v", err)
		return result, &ITunesError{Op: "PlayStreamURL: run script", Kind: ErrJXAScript, Err: err}
	}

	if strings.HasPrefix(response, "ERROR:") {
		result.Success = false
		result.Message = strings.TrimPrefix(response, "ERROR: ")
		return result, &ITunesError{Op: "PlayStreamURL: script error", Kind: ErrJXAScript, Err: fmt.Errorf("%s", result.Message)}
	}

	// Give Apple Music more time to process the new stream URL
	time.Sleep(3 * time.Second)

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
	DBManager     *database.DatabaseManager
	SearchManager *database.SearchManager
	SearchLimit   = 15 // Default search limit, can be overridden by ITUNES_SEARCH_LIMIT env var
)

func retryWithBackoff(fn func() error, maxRetries int) error {
	backoff := 100 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		if err := fn(); err == nil {
			return nil
		}
		time.Sleep(backoff)
		backoff *= 2
	}
	return fmt.Errorf("failed after %d retries", maxRetries)
}

// InitDatabase initializes the SQLite database connection
func InitDatabase(logger *zap.Logger) error {
	// Get search limit from environment if set
	if limitStr := os.Getenv("ITUNES_SEARCH_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			SearchLimit = limit
		}
	}

	// Initialize database manager
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath, logger)
	if err != nil {
		return &ITunesError{Op: "InitDatabase: new database manager", Kind: ErrDatabase, Err: err}
	}

	// Run migrations to ensure schema is up to date
	if err := dm.RunMigrations(); err != nil {
		dm.Close()
		return &ITunesError{Op: "InitDatabase: run migrations", Kind: ErrDatabase, Err: err}
	}

	DBManager = dm
	SearchManager = database.NewSearchManager(dm)
	return nil
}

// CloseDatabase closes the database connection
func CloseDatabase() {
	if DBManager != nil {
		DBManager.Close()
		DBManager = nil
		SearchManager = nil
	}
}

// SearchTracksFromDatabase searches tracks using the SQLite database with FTS5
func SearchTracksFromDatabase(query string, filters *database.SearchFilters) ([]Track, error) {
	if DBManager == nil {
		return nil, &ITunesError{Op: "SearchTracksFromDatabase", Kind: ErrDatabase, Err: errors.New("database not initialized")}
	}

	// Apply default search limit if not specified in filters
	if filters == nil {
		filters = &database.SearchFilters{Limit: SearchLimit}
	} else if filters.Limit <= 0 {
		filters.Limit = SearchLimit
	}

	// Use search manager for cached search
	dbTracks, err := SearchManager.SearchWithCache(query, filters)
	if err != nil {
		return nil, &ITunesError{Op: "SearchTracksFromDatabase: search with cache", Kind: ErrDatabase, Err: err}
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
	if DBManager == nil {
		return nil, &ITunesError{Op: "GetTrackByPersistentID", Kind: ErrDatabase, Err: errors.New("database not initialized")}
	}

	dbTrack, err := DBManager.GetTrackByPersistentID(persistentID)
	if err != nil {
		return nil, &ITunesError{Op: "GetTrackByPersistentID: get from db", Kind: ErrDatabase, Err: err}
	}

	if dbTrack == nil {
		return nil, &ITunesError{Op: "GetTrackByPersistentID", Kind: ErrNoTracksFound, Err: fmt.Errorf("track with persistent ID '%s' not found", persistentID)}
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
	if DBManager == nil {
		return nil, &ITunesError{Op: "GetPlaylistTracks", Kind: ErrDatabase, Err: errors.New("database not initialized")}
	}

	dbTracks, err := DBManager.GetPlaylistTracks(playlistIdentifier, usePlaylistID)
	if err != nil {
		return nil, &ITunesError{Op: "GetPlaylistTracks: get from db", Kind: ErrDatabase, Err: err}
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
	if DBManager == nil {
		return nil, &ITunesError{Op: "GetDatabaseStats", Kind: ErrDatabase, Err: errors.New("database not initialized")}
	}
	stats, err := DBManager.GetStats()
	if err != nil {
		return nil, &ITunesError{Op: "GetDatabaseStats: get from db", Kind: ErrDatabase, Err: err}
	}
	return stats, nil
}

// ListPlaylists returns all user playlists from the database
func ListPlaylists() ([]database.Playlist, error) {
	if DBManager == nil {
		return nil, &ITunesError{Op: "ListPlaylists", Kind: ErrDatabase, Err: errors.New("database not initialized")}
	}
	playlists, err := DBManager.ListPlaylists()
	if err != nil {
		return nil, &ITunesError{Op: "ListPlaylists: get from db", Kind: ErrDatabase, Err: err}
	}
	return playlists, nil
}

// SearchTracks is the main search function using database
func SearchTracks(query string) ([]Track, error) {
	if DBManager == nil {
		return nil, &ITunesError{Op: "SearchTracks", Kind: ErrDatabase, Err: errors.New("database not initialized - please run InitDatabase() first")}
	}
	return SearchTracksFromDatabase(query, nil)
}

// SearchStations searches for radio stations in the database
func SearchStations(query string) (*StationSearchResult, error) {
	if DBManager == nil {
		return &StationSearchResult{
			Status:  "error",
			Query:   query,
			Count:   0,
			Message: "Database not initialized - please run InitDatabase() first or import stations with 'itunes import-stations stations.json'",
		}, &ITunesError{Op: "SearchStations", Kind: ErrDatabase, Err: errors.New("database not initialized - please run InitDatabase() first")}
	}

	filters := &database.RadioStationFilters{
		Limit: 15, // Default limit, can be made configurable
	}

	stations, err := DBManager.SearchRadioStations(query, filters)
	if err != nil {
		return &StationSearchResult{
			Status:  "error",
			Query:   query,
			Count:   0,
			Message: fmt.Sprintf("Failed to search radio stations: %v", err),
		}, &ITunesError{Op: "SearchStations: search in db", Kind: ErrDatabase, Err: err}
	}

	// Convert database stations to API stations
	var apiStations []Station
	for _, dbStation := range stations {
		apiStation := Station{
			ID:          dbStation.ID,
			Name:        dbStation.Name,
			Description: dbStation.Description,
			URL:         dbStation.URL,
			Genre:       dbStation.Genre,
			Homepage:    dbStation.Homepage,
			Keywords:    []string{}, // Legacy field for compatibility
		}
		apiStations = append(apiStations, apiStation)
	}

	result := &StationSearchResult{
		Status:   "success",
		Query:    query,
		Stations: apiStations,
		Count:    len(apiStations),
	}

	if len(apiStations) == 0 {
		result.Status = "no_results"
		result.Message = "No radio stations found matching the query. Use 'itunes import-stations stations.json' to add a curated list."
	}

	return result, nil
}
