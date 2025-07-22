package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"itunes/itunes"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: itunes <command> [arguments]")
		fmt.Println("Commands:")
		fmt.Println("  search <query>             - Search iTunes library for tracks")
		fmt.Println("  play <collection> [track]  - Play album/playlist (use 'collection' field from search results)")
		fmt.Println("\nEnvironment variables:")
		fmt.Println("  ITUNES_USE_DATABASE=true   - Use SQLite database instead of JSON cache")
		return
	}

	command := os.Args[1]

	// Check if database mode is enabled
	useDatabase := strings.ToLower(os.Getenv("ITUNES_USE_DATABASE")) == "true"
	if useDatabase {
		// Initialize database
		if err := itunes.InitDatabase(); err != nil {
			fmt.Printf("Warning: Failed to initialize database, falling back to JSON cache: %v\n", err)
			useDatabase = false
		} else {
			defer itunes.CloseDatabase()
			itunes.UseDatabase = true
			fmt.Println("Using SQLite database for search")
		}
	}

	// Initialize cache manager
	cacheManager := itunes.NewCacheManager()

	switch command {
	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes search <query>")
			return
		}
		query := os.Args[2]

		// Check cache first
		if cachedTracks, found := cacheManager.Get(query); found {
			fmt.Println("(Using cached results)")
			tracks := cachedTracks

			// Save to latest results file for backward compatibility
			if err := cacheManager.SaveLatestResults(tracks); err != nil {
				fmt.Printf("Warning: Could not save results file: %v\n", err)
			}

			fmt.Printf("Search results saved to %s/search_results.json\n", cacheManager.GetCacheDir())
			for _, t := range tracks {
				fmt.Printf("%s by %s [%s]\n", t.Name, t.Artist, t.Collection)
			}
			return
		}

		// Cache miss - perform actual search
		tracks, err := itunes.SearchTracks(query)
		if err != nil {
			if errors.Is(err, itunes.ErrNoTracksFound) {
				fmt.Println("No tracks found.")
			} else {
				fmt.Println("Error:", err)
			}
			return
		}

		if len(tracks) == 0 {
			fmt.Println("No tracks found.")
			return
		}

		// Cache the results
		if err := cacheManager.Set(query, tracks); err != nil {
			fmt.Printf("Warning: Could not cache results: %v\n", err)
		}

		// Save to latest results file for backward compatibility
		if err := cacheManager.SaveLatestResults(tracks); err != nil {
			fmt.Printf("Warning: Could not save results file: %v\n", err)
		}

		fmt.Printf("Search results saved to %s/search_results.json\n", cacheManager.GetCacheDir())
		for _, t := range tracks {
			fmt.Printf("%s by %s [%s]\n", t.Name, t.Artist, t.Collection)
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
