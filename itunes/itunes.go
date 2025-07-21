package itunes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Exported error variables for better error handling
var (
	ErrNoTracksFound = errors.New("no tracks found")
	ErrScriptFailed  = errors.New("JXA script execution failed")
)

// Track describes one track from the script's output
type Track struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Album      string `json:"album"`
	Collection string `json:"collection"`
	Artist     string `json:"artist"`
}

// PlayPlaylistTrack runs the embedded iTunes_Play_Playlist_Track.js script to play a playlist or track.
// If trackName is "", only the playlist will play.
func PlayPlaylistTrack(playlistName, trackName string) error {
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

	// Add track name if provided
	if trackName != "" {
		args = append(args, trackName)
	}

	cmd := exec.CommandContext(ctx, "osascript", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", ErrScriptFailed, stderr.String())
	}

	// Parse the structured response
	responseJSON := stdout.Bytes()
	if len(responseJSON) > 0 {
		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Error   string `json:"error"`
		}

		if err := json.Unmarshal(responseJSON, &response); err == nil {
			if response.Status == "error" {
				return fmt.Errorf("play script error: %s", response.Message)
			}
		}
	}

	return nil
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
			// Check partial matches in all searchable fields
			searchableText := strings.Join([]string{collectionName, trackName, artistName, albumName}, " ")
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
