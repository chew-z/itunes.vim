package main

import (
	"errors"
	"fmt"
	"os"

	"itunes/itunes"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: itunes <command> [arguments]")
		fmt.Println("Commands:")
		fmt.Println("  search <query>             - Search iTunes library for tracks")
		fmt.Println("  play <collection> [track]  - Play album/playlist (use 'collection' field from search results)")
		return
	}

	command := os.Args[1]

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
		tracks, err := itunes.SearchTracksFromCache(query)
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
			fmt.Println("Usage: itunes play <collection> [track]")
			fmt.Println("Use the 'collection' field from search results as the collection name")
			return
		}
		playlist := os.Args[2]
		var track string
		if len(os.Args) > 3 {
			track = os.Args[3]
		}

		if err := itunes.PlayPlaylistTrack(playlist, track); err != nil {
			fmt.Println("Play failed:", err)
		} else {
			fmt.Println("Playback started.")
		}

	default:
		fmt.Println("Unknown command:", command)
	}
}
