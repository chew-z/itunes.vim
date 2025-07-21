package itunes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Track describes one track from the script's output
type Track struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Album      string `json:"album"`
	Collection string `json:"collection"`
	Artist     string `json:"artist"`
}

// getScriptPath returns the absolute path to a script file in the autoload directory
func getScriptPath(scriptName string) (string, error) {
	// Get the directory of the current source file
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current file path")
	}

	// Go up to the project root (from itunes/itunes.go to project root)
	projectRoot := filepath.Dir(filepath.Dir(currentFile))
	scriptPath := filepath.Join(projectRoot, "autoload", scriptName)

	// Check if the script file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("script file not found: %s", scriptPath)
	}

	return scriptPath, nil
}

// SearchiTunesPlaylists runs the iTunes_Search2_fzf.js script and returns found tracks.
func SearchiTunesPlaylists(query string) ([]Track, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	scriptPath, err := getScriptPath("iTunes_Search2_fzf.js")
	if err != nil {
		return nil, fmt.Errorf("failed to locate script: %w", err)
	}

	cmd := exec.CommandContext(
		ctx,
		"/usr/bin/env", "osascript", "-l", "JavaScript", scriptPath, query,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				return nil, fmt.Errorf("No tracks found. Script debug output:\n%s", stderr.String())
			}
		}
		return nil, fmt.Errorf("failed to run iTunes_Search2_fzf.js with error: %w\nScript error: %s", err, stderr.String())
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

// PlayPlaylistTrack runs iTunes_Play_Playlist_Track.js to play a playlist or track.
// If trackName is "", only the playlist will play.
func PlayPlaylistTrack(playlistName, trackName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	scriptPath, err := getScriptPath("iTunes_Play_Playlist_Track.js")
	if err != nil {
		return fmt.Errorf("failed to locate script: %w", err)
	}

	args := []string{"-l", "JavaScript", scriptPath, playlistName}
	if trackName != "" {
		args = append(args, trackName)
	}

	cmd := exec.CommandContext(ctx, "osascript", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}
