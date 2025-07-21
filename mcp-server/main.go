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
		mcp.WithDescription("Search iTunes/Apple Music library for tracks"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query for tracks in the iTunes library"),
		),
	)

	// Create play tool
	playTool := mcp.NewTool("play_track",
		mcp.WithDescription("Play a track in iTunes/Apple Music with proper context for continuous playback. Use track_id with either playlist (for user playlists) or album (for album playback). The `playlist` parameter now works with actual user-created playlists."),
		mcp.WithString("track_id",
			mcp.Description("RECOMMENDED: Use the EXACT 'id' field value from search_itunes results. Most reliable method that avoids encoding/character issues."),
		),
		mcp.WithString("playlist",
			mcp.Description("For playlist context: Use the exact 'collection' field value or a value from the 'playlists' array. Provides playlist context so playback continues with next tracks in playlist."),
		),
		mcp.WithString("album",
			mcp.Description("For album context: Use the exact 'album' field value from search_itunes results. Provides album context so playback continues with next tracks in album."),
		),
		mcp.WithString("track",
			mcp.Description("FALLBACK: Use the EXACT 'name' field value from search_itunes results. Only use if track_id unavailable. Less reliable with complex names - prefer track_id."),
		),
	)

	// Create refresh tool
	refreshTool := mcp.NewTool("refresh_library",
		mcp.WithDescription("Refresh the iTunes/Apple Music library cache. WARNING: This is a resource-intensive operation that takes 1-3 minutes for large libraries. Only use with explicit user approval and sparingly - typically only on first use or after significant library changes. Use search_itunes for normal operations as it uses cached data."),
	)

	// Add tools to server
	mcpServer.AddTool(searchTool, searchHandler)
	mcpServer.AddTool(playTool, playHandler)
	mcpServer.AddTool(refreshTool, refreshHandler)

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
	tracks, err := itunes.SearchTracksFromCache(query)
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

	err := itunes.PlayPlaylistTrack(playlist, album, track, trackID)
	if err != nil {
		if errors.Is(err, itunes.ErrScriptFailed) {
			return mcp.NewToolResultError(fmt.Sprintf("Unable to control Apple Music: %v", err)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Playback failed: %v", err)), nil
	}

	// Build success message based on what was used
	if trackID != "" {
		if playlist != "" {
			return mcp.NewToolResultText(fmt.Sprintf("Started playing track ID '%s' from playlist '%s'", trackID, playlist)), nil
		} else if album != "" {
			return mcp.NewToolResultText(fmt.Sprintf("Started playing track ID '%s' from album '%s'", trackID, album)), nil
		} else {
			return mcp.NewToolResultText(fmt.Sprintf("Started playing track by ID: %s", trackID)), nil
		}
	} else if track != "" {
		if playlist != "" {
			return mcp.NewToolResultText(fmt.Sprintf("Started playing track '%s' from playlist '%s'", track, playlist)), nil
		} else if album != "" {
			return mcp.NewToolResultText(fmt.Sprintf("Started playing track '%s' from album '%s'", track, album)), nil
		} else {
			return mcp.NewToolResultText(fmt.Sprintf("Started playing track: %s", track)), nil
		}
	} else if playlist != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Started playing playlist '%s'", playlist)), nil
	} else if album != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Started playing album '%s'", album)), nil
	}

	return mcp.NewToolResultText("Playback started"), nil
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
