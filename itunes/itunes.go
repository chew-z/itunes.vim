package itunes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// SearchiTunesPlaylists runs the embedded iTunes_Search2_fzf.js script and returns found tracks.
func SearchiTunesPlaylists(query string) ([]Track, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a temporary file with the embedded script
	tempFile, err := os.CreateTemp("", "itunes_search_*.js")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write the embedded script to the temp file
	if _, err := tempFile.WriteString(searchScript); err != nil {
		return nil, fmt.Errorf("failed to write script to temp file: %w", err)
	}
	tempFile.Close()

	cmd := exec.CommandContext(
		ctx,
		"/usr/bin/env", "osascript", "-l", "JavaScript", tempFile.Name(), query,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				return nil, ErrNoTracksFound
			}
		}
		return nil, fmt.Errorf("%w: %s", ErrScriptFailed, stderr.String())
	}

	out := stdout.Bytes()
	if len(out) == 0 {
		return []Track{}, nil
	}

	var tracks []Track
	if err := json.Unmarshal(out, &tracks); err != nil {
		return nil, fmt.Errorf("invalid JSON output: %w", err)
	}
	return tracks, nil
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

	args := []string{"-l", "JavaScript", tempFile.Name(), playlistName}
	if trackName != "" {
		args = append(args, trackName)
	}

	cmd := exec.CommandContext(ctx, "osascript", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", ErrScriptFailed, stderr.String())
	}

	return nil
}
