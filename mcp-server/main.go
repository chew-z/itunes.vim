package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"itunes/itunes"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create MCP server
	mcpServer := server.NewMCPServer(
		"itunes-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	// Create search tool
	searchTool := mcp.NewTool("search_itunes",
		mcp.WithDescription("Search iTunes/Apple Music library for tracks"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query for tracks in the iTunes library"),
		),
	)

	// Create play tool
	playTool := mcp.NewTool("play_track",
		mcp.WithDescription("Play a playlist or specific track in iTunes/Apple Music"),
		mcp.WithString("playlist",
			mcp.Required(),
			mcp.Description("Name of the playlist to play"),
		),
		mcp.WithString("track",
			mcp.Description("Optional specific track name to play within the playlist"),
		),
	)

	// Add tools to server
	mcpServer.AddTool(searchTool, searchHandler)
	mcpServer.AddTool(playTool, playHandler)

	// Start stdio server
	if err := server.ServeStdio(mcpServer); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

func searchHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid query parameter: %v", err)), nil
	}

	tracks, err := itunes.SearchiTunesPlaylists(query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	if len(tracks) == 0 {
		return mcp.NewToolResultText("No tracks found matching the query."), nil
	}

	result, err := json.MarshalIndent(tracks, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

func playHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playlist, err := request.RequireString("playlist")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid playlist parameter: %v", err)), nil
	}

	// Track is optional, so we use a different method to get it
	args := request.GetArguments()
	track := ""
	if trackVal, exists := args["track"]; exists {
		if trackStr, ok := trackVal.(string); ok {
			track = trackStr
		}
	}

	err = itunes.PlayPlaylistTrack(playlist, track)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Playback failed: %v", err)), nil
	}

	if track != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Started playing track '%s' from playlist '%s'", track, playlist)), nil
	} else {
		return mcp.NewToolResultText(fmt.Sprintf("Started playing playlist '%s'", playlist)), nil
	}
}
