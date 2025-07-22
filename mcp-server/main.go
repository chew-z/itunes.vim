package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"itunes/itunes"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Global cache manager for the MCP server session
var cacheManager *itunes.CacheManager

func main() {
	// Initialize global cache manager
	cacheManager = itunes.NewCacheManager()

	// Check if database mode is enabled
	useDatabase := os.Getenv("ITUNES_USE_DATABASE") == "true"
	if useDatabase {
		// Initialize database
		if err := itunes.InitDatabase(); err != nil {
			fmt.Printf("Warning: Failed to initialize database, falling back to JSON cache: %v\n", err)
		} else {
			defer itunes.CloseDatabase()
			itunes.UseDatabase = true
			fmt.Println("Using SQLite database for search")
		}
	}

	// Start periodic cleanup of expired cache files
	go func() {
		// Clean up immediately on startup
		cacheManager.CleanupExpired()

		// Then clean up every hour
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cacheManager.CleanupExpired()
		}
	}()

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

	// Add MCP resources for cache access
	cacheStatsResource := mcp.NewResource(
		"itunes://cache/stats",
		"Cache Statistics",
		mcp.WithResourceDescription("iTunes cache statistics and metadata"),
		mcp.WithMIMEType("application/json"),
	)

	cacheQueriesResource := mcp.NewResource(
		"itunes://cache/queries",
		"Cached Queries",
		mcp.WithResourceDescription("List of all cached search queries with metadata"),
		mcp.WithMIMEType("application/json"),
	)

	latestResultsResource := mcp.NewResource(
		"itunes://cache/latest",
		"Latest Search Results",
		mcp.WithResourceDescription("Most recent search results from cache"),
		mcp.WithMIMEType("application/json"),
	)

	mcpServer.AddResource(cacheStatsResource, cacheStatsHandler)
	mcpServer.AddResource(cacheQueriesResource, cacheQueriesHandler)
	mcpServer.AddResource(latestResultsResource, latestResultsHandler)

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

	// Check cache first
	if cachedTracks, found := cacheManager.Get(query); found {
		result, err := json.MarshalIndent(cachedTracks, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal cached results: %v", err)), nil
		}
		return mcp.NewToolResultText(string(result)), nil
	}

	// Cache miss - perform actual search
	tracks, err := itunes.SearchTracks(query)
	if err != nil {
		if errors.Is(err, itunes.ErrNoTracksFound) {
			return mcp.NewToolResultText("No tracks found matching the query."), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	// Cache the results
	if err := cacheManager.Set(query, tracks); err != nil {
		// Log cache error but don't fail the request
		fmt.Printf("Warning: Failed to cache search results: %v\n", err)
	}

	// Also save for 'latest' resource (backward compatibility with CLI)
	if err := cacheManager.SaveLatestResults(tracks); err != nil {
		fmt.Printf("Warning: Failed to save latest search results: %v\n", err)
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
	err := itunes.RefreshLibraryCache()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Library refresh failed: %v", err)), nil
	}

	// Count tracks and playlists in cache to report detailed success
	cacheFile := filepath.Join(os.TempDir(), "itunes-cache", "library.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return mcp.NewToolResultText("Library refresh completed, but couldn't verify cache."), nil
	}

	var tracks []itunes.Track
	if err := json.Unmarshal(data, &tracks); err != nil {
		return mcp.NewToolResultText("Library refresh completed, but couldn't parse cache."), nil
	}

	// Count unique playlists by iterating through the new Playlists field
	playlistSet := make(map[string]bool)
	for _, track := range tracks {
		for _, playlist := range track.Playlists {
			if playlist != "" {
				playlistSet[playlist] = true
			}
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf("Library refresh completed successfully!\n\n📊 **Cache Statistics:**\n• **%d tracks** cached from your iTunes library\n• **%d playlists** scanned and indexed\n• Cache location: %s\n\n✅ You can now search for music with fast, token-efficient results (max 15 tracks per search).", len(tracks), len(playlistSet), cacheFile)), nil
}

// Resource handlers for cache access
func cacheStatsHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	stats := cacheManager.GetCacheStats()
	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cache stats: %w", err)
	}

	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(statsJSON),
		},
	}, nil
}

func cacheQueriesHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	queries := cacheManager.GetAllCachedQueries()
	queriesJSON, err := json.MarshalIndent(queries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cached queries: %w", err)
	}

	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(queriesJSON),
		},
	}, nil
}

func latestResultsHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Read the latest results file
	latestFile := filepath.Join(cacheManager.GetCacheDir(), "search_results.json")
	data, err := os.ReadFile(latestFile)
	if err != nil {
		if !os.IsNotExist(err) {
			// Log other errors like permission issues
			fmt.Printf("Warning: could not read latest results file: %v\n", err)
		}
		// If no latest results, return empty array
		return []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/json",
				Text:     "[]",
			},
		}, nil
	}

	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
