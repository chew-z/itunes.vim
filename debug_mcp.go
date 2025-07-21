package main

import (
	"fmt"
	"itunes/itunes"
)

func main() {
	// Test the exact call that MCP server makes
	playlist := ""
	track := "SomaFM: Sonic Universe (#1): Transcending the world of jazz with eclectic, avant-garde takes on tradition."

	fmt.Printf("=== MCP Server Debug Test ===\n")
	fmt.Printf("Playlist: '%s'\n", playlist)
	fmt.Printf("Track: '%s'\n", track)
	fmt.Printf("Track length: %d chars\n", len(track))

	err := itunes.PlayPlaylistTrack(playlist, track)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("SUCCESS: Track played successfully\n")
	}
}
