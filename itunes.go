package main

import (
	"encoding/json"
	"fmt"
	"os"

	"itunes/itunes"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: itunes <command> [arguments]")
		fmt.Println("Commands: search <query>, play <playlist> [track]")
		return
	}

	command := os.Args[1]

	switch command {
	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes search <query>")
			return
		}
		query := os.Args[2]
		tracks, err := itunes.SearchiTunesPlaylists(query)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		if len(tracks) == 0 {
			fmt.Println("No tracks found.")
			// remove search results file if it exists
			if _, err := os.Stat("itunes_search_results.json"); err == nil {
				os.Remove("itunes_search_results.json")
			}
			return
		}

		// Store results in a file
		file, err := os.Create("itunes_search_results.json")
		if err != nil {
			fmt.Println("Error creating search results file:", err)
			return
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(tracks); err != nil {
			fmt.Println("Error writing search results:", err)
		}

		fmt.Println("Search results saved to itunes_search_results.json")
		for _, t := range tracks {
			fmt.Printf("%s by %s [%s]\n", t.Name, t.Artist, t.Collection)
		}

	case "play":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes play <playlist> [track]")
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
