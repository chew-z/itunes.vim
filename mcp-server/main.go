package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"itunes/database"
	"itunes/itunes"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Initialize database (now default mode)
	if err := itunes.InitDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize database: %v\n", err)
		fmt.Fprintln(os.Stderr, "Please ensure the database exists by running: itunes-migrate")
		os.Exit(1)
	}
	defer itunes.CloseDatabase()

	// Create MCP server with tool and resource capabilities
	mcpServer := server.NewMCPServer(
		"itunes-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true), // subscribe and list resources
		server.WithLogging(),
	)

	// Create search tool
	searchTool := mcp.NewTool("search_itunes",
		mcp.WithDescription("Searches the local iTunes/Apple Music library for tracks, artists, or albums."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search term. Can be a track name, artist, album, or playlist."),
		),
	)

	// Create play tool
	playTool := mcp.NewTool("play_track",
		mcp.WithDescription("Plays a track, playlist, or album. To play a specific track within a playlist or album, provide both the track identifier (track_id or track) and the context (playlist or album)."),
		mcp.WithString("track_id",
			mcp.Description("The unique ID of the track from a search result. The most reliable way to play a specific track."),
		),
		mcp.WithString("playlist",
			mcp.Description("The name of the playlist to play. If a track_id is also given, starts playback from that track within the playlist."),
		),
		mcp.WithString("album",
			mcp.Description("The name of the album to play. If a track_id is also given, starts playback from that track within the album."),
		),
		mcp.WithString("track",
			mcp.Description("The name of the track. Use as a fallback if track_id is not known."),
		),
	)

	// Create refresh tool
	refreshTool := mcp.NewTool("refresh_library",
		mcp.WithDescription("Updates the local library cache from the Music app. This can take several minutes for large libraries. Run this only if your library has changed significantly."),
	)

	// Create now playing tool
	nowPlayingTool := mcp.NewTool("now_playing",
		mcp.WithDescription("Gets the current playback status and track information from Apple Music."),
	)

	// Add tools to server
	mcpServer.AddTool(searchTool, searchHandler)
	mcpServer.AddTool(playTool, playHandler)
	mcpServer.AddTool(refreshTool, refreshHandler)
	mcpServer.AddTool(nowPlayingTool, nowPlayingHandler)

	// Add MCP resources for database statistics
	dbStatsResource := mcp.NewResource(
		"itunes://database/stats",
		"Database Statistics",
		mcp.WithResourceDescription("iTunes SQLite database statistics and metadata"),
		mcp.WithMIMEType("application/json"),
	)

	mcpServer.AddResource(dbStatsResource, dbStatsHandler)

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

	// Search using database (now default)
	tracks, err := itunes.SearchTracks(query)
	if err != nil {
		if errors.Is(err, itunes.ErrNoTracksFound) {
			return mcp.NewToolResultText("No tracks found matching the query."), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	result, err := json.MarshalIndent(tracks, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

func playHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get parameters - track_id is preferred, then track name as fallback
	trackID := request.GetString("track_id", "")
	playlist := request.GetString("playlist", "")
	album := request.GetString("album", "")
	track := request.GetString("track", "")

	// Need at least one track identifier
	if playlist == "" && album == "" && track == "" && trackID == "" {
		return mcp.NewToolResultError("Either playlist, album, track, or track_id parameter must be provided"), nil
	}

	// Use the enhanced function that returns current track info
	result, err := itunes.PlayPlaylistTrackWithStatus(playlist, album, track, trackID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Playback operation failed: %v", err)), nil
	}

	if !result.Success {
		return mcp.NewToolResultError(result.Message), nil
	}

	// Return detailed result with current track info
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultText(result.Message), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func nowPlayingHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status, err := itunes.GetNowPlaying()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get current playback status: %v", err)), nil
	}

	resultJSON, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal status: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func refreshHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// First refresh the library cache (creates JSON files)
	err := itunes.RefreshLibraryCache()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Library refresh failed: %v", err)), nil
	}

	// Now migrate the refreshed data to the database
	cacheDir := filepath.Join(os.TempDir(), "itunes-cache")
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to open database: %v", err)), nil
	}
	defer dm.Close()

	// Migrate from the enhanced JSON file (or fallback to legacy)
	err = dm.MigrateFromJSON(cacheDir, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to migrate data to database: %v", err)), nil
	}

	// Get database statistics
	stats, err := dm.GetStats()
	if err != nil {
		return mcp.NewToolResultText("Library refresh and database update completed, but couldn't get statistics."), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Library refresh completed successfully!\n\n📊 **Database Statistics:**\n• **%d tracks** stored in SQLite database\n• **%d playlists** indexed\n• **%d artists** cataloged\n• **%d albums** identified\n• Database size: %.2f MB\n\n✅ You can now search for music with ultra-fast database queries (limit: %d tracks per search).",
		stats.TrackCount,
		stats.PlaylistCount,
		stats.ArtistCount,
		stats.AlbumCount,
		float64(stats.DatabaseSize)/(1024*1024),
		itunes.SearchLimit)), nil
}

// Resource handlers for cache access
func dbStatsHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	stats, err := itunes.GetDatabaseStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get database stats: %w", err)
	}

	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal database stats: %w", err)
	}

	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(statsJSON),
		},
	}, nil
}
