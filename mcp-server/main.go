package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"itunes/itunes"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Global cache manager for the MCP server session
var cacheManager *itunes.CacheManager

func main() {
	// Initialize global cache manager
	cacheManager = itunes.NewCacheManager()

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
	tracks, err := itunes.SearchiTunesPlaylists(query)
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
		if errors.Is(err, itunes.ErrScriptFailed) {
			return mcp.NewToolResultError(fmt.Sprintf("Unable to control Apple Music: %v", err)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Playback failed: %v", err)), nil
	}

	if track != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Started playing track '%s' from playlist '%s'", track, playlist)), nil
	} else {
		return mcp.NewToolResultText(fmt.Sprintf("Started playing playlist '%s'", playlist)), nil
	}
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
