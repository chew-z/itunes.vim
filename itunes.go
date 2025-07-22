package main

import (
	"fmt"
	"os"
	"strconv"

	"itunes/itunes"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: itunes <command> [arguments]")
		fmt.Println("Commands:")
		fmt.Println("  search <query>             - Search iTunes library for tracks")
		fmt.Println("  play <collection> [track]  - Play album/playlist (use 'collection' field from search results)")
		fmt.Println("\nEnvironment variables:")
		fmt.Println("  ITUNES_SEARCH_LIMIT=<num>  - Set search result limit (default: 15)")
		return
	}

	command := os.Args[1]

	// Initialize database (now default mode)
	if err := itunes.InitDatabase(); err != nil {
		fmt.Printf("Error: Failed to initialize database: %v\n", err)
		fmt.Println("Please ensure the database exists by running: itunes-migrate")
		return
	}
	defer itunes.CloseDatabase()

	// Get search limit from environment
	searchLimit := 15
	if limitStr := os.Getenv("ITUNES_SEARCH_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			searchLimit = limit
		}
	}

	switch command {
	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes search <query>")
			return
		}
		query := os.Args[2]

		// Search using database
		tracks, err := itunes.SearchTracks(query)
		if err != nil {
			fmt.Printf("Error searching tracks: %v\n", err)
			return
		}

		if len(tracks) == 0 {
			fmt.Println("No tracks found")
			return
		}

		// Display results
		fmt.Printf("Found %d tracks (limit: %d):\n", len(tracks), searchLimit)
		for _, t := range tracks {
			fmt.Printf("%s by %s [%s] (ID: %s)\n", t.Name, t.Artist, t.Collection, t.ID)
		}

	case "play":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes play <playlist> [album] [track] [trackID]")
			fmt.Println("  playlist: playlist name (use empty string \"\" if not applicable)")
			fmt.Println("  album: album name for album context (optional)")
			fmt.Println("  track: track name (optional)")
			fmt.Println("  trackID: track ID from search results (recommended, most reliable)")
			return
		}

		// Parse arguments with support for empty strings
		playlist := os.Args[2]
		var album, track, trackID string

		if len(os.Args) > 3 {
			album = os.Args[3]
		}
		if len(os.Args) > 4 {
			track = os.Args[4]
		}
		if len(os.Args) > 5 {
			trackID = os.Args[5]
		}

		if err := itunes.PlayPlaylistTrack(playlist, album, track, trackID); err != nil {
			fmt.Println("Play failed:", err)
		} else {
			fmt.Println("Playback started.")
		}

	case "now-playing", "status":
		status, err := itunes.GetNowPlaying()
		if err != nil {
			fmt.Println("Failed to get current status:", err)
			return
		}

		if status.Status == "playing" && status.Track != nil {
			fmt.Printf("Status: %s\n", status.Status)
			fmt.Printf("Track: %s\n", status.Display)
			fmt.Printf("Album: %s\n", status.Track.Album)
			fmt.Printf("Position: %s / %s\n", status.Track.Position, status.Track.Duration)
			if status.Track.ID != "" {
				fmt.Printf("Track ID: %s\n", status.Track.ID)
			}
		} else {
			fmt.Printf("Status: %s\n", status.Status)
			if status.Message != "" {
				fmt.Printf("Message: %s\n", status.Message)
			}
		}

	default:
		fmt.Println("Unknown command:", command)
		fmt.Println("Available commands: search, play, now-playing, status")
	}
}
