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
		mcp.WithDescription("Plays a track with optional playlist context for continuous playback. Playlist context enables seamless continuation within the playlist. Album parameter helps locate tracks but does not provide playback context."),
		mcp.WithString("track_id",
			mcp.Description("The unique ID of the track from a search result. The most reliable way to play a specific track."),
		),
		mcp.WithString("playlist",
			mcp.Description("The name of the playlist to play. If a track_id is also given, starts playback from that track within the playlist."),
		),
		mcp.WithString("album",
			mcp.Description("The name of the album to help locate tracks. Note: Does not provide album playback context - individual track will play without album continuation."),
		),
		mcp.WithString("track",
			mcp.Description("The name of the track. Use as a fallback if track_id is not known."),
		),
	)

	// Create refresh tool
	refreshTool := mcp.NewTool("refresh_library",
		mcp.WithDescription("Refreshes the iTunes library database by extracting current data from Apple Music app and populating SQLite database. Takes 1-3 minutes for large libraries. Use only when library has changed significantly."),
	)

	// Create now playing tool
	nowPlayingTool := mcp.NewTool("now_playing",
		mcp.WithDescription("Gets the current playback status and track information from Apple Music."),
	)

	// Create list playlists tool
	listPlaylistsTool := mcp.NewTool("list_playlists",
		mcp.WithDescription("Lists all user playlists in the iTunes/Apple Music library with metadata."),
	)

	// Create get playlist tracks tool
	getPlaylistTracksTool := mcp.NewTool("get_playlist_tracks",
		mcp.WithDescription("Gets all tracks in a specific playlist."),
		mcp.WithString("playlist",
			mcp.Required(),
			mcp.Description("The name or persistent ID of the playlist."),
		),
		mcp.WithBoolean("use_id",
			mcp.Description("Set to true if providing a persistent ID instead of name. Default is false (use name)."),
		),
	)

	// Create advanced search tool
	searchAdvancedTool := mcp.NewTool("search_advanced",
		mcp.WithDescription("Advanced search with filters for genre, artist, album, rating, and starred status."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query for track names, artists, or albums."),
		),
		mcp.WithString("genre",
			mcp.Description("Filter by genre (partial match supported)."),
		),
		mcp.WithString("artist",
			mcp.Description("Filter by artist name (partial match supported)."),
		),
		mcp.WithString("album",
			mcp.Description("Filter by album name (partial match supported)."),
		),
		mcp.WithString("playlist",
			mcp.Description("Filter to tracks in a specific playlist."),
		),
		mcp.WithNumber("min_rating",
			mcp.Description("Minimum rating (0-100). Only returns tracks with rating >= this value."),
		),
		mcp.WithBoolean("starred",
			mcp.Description("If true, only return starred/loved tracks. If false, return all tracks."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return. Default is 15."),
		),
		mcp.WithBoolean("streaming_only",
			mcp.Description("If true, only return streaming tracks (e.g., radio stations). If false, return all tracks."),
		),
		mcp.WithBoolean("local_only",
			mcp.Description("If true, only return local (non-streaming) tracks. If false, return all tracks."),
		),
	)

	// Create stream playback tool
	playStreamTool := mcp.NewTool("play_stream",
		mcp.WithDescription("Play Apple Music stream from an itmss:// or https://music.apple.com/ URL."),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The itmss:// or https://music.apple.com/ URL to play."),
		),
	)

	// Create station search tool
	searchStationsTool := mcp.NewTool("search_stations",
		mcp.WithDescription("Search for Apple Music radio stations by genre, name, or keywords."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query for stations (e.g., 'country', 'jazz', 'rock', 'classical')."),
		),
	)

	// Add tools to server
	mcpServer.AddTool(searchTool, searchHandler)
	mcpServer.AddTool(playTool, playHandler)
	mcpServer.AddTool(refreshTool, refreshHandler)
	mcpServer.AddTool(nowPlayingTool, nowPlayingHandler)
	mcpServer.AddTool(listPlaylistsTool, listPlaylistsHandler)
	mcpServer.AddTool(getPlaylistTracksTool, getPlaylistTracksHandler)
	mcpServer.AddTool(searchAdvancedTool, searchAdvancedHandler)
	mcpServer.AddTool(playStreamTool, playStreamHandler)
	mcpServer.AddTool(searchStationsTool, searchStationsHandler)

	// Add MCP resources for database statistics
	dbStatsResource := mcp.NewResource(
		"itunes://database/stats",
		"Database Statistics",
		mcp.WithResourceDescription("iTunes SQLite database statistics and metadata"),
		mcp.WithMIMEType("application/json"),
	)

	// Add playlists resource
	playlistsResource := mcp.NewResource(
		"itunes://database/playlists",
		"Playlists List",
		mcp.WithResourceDescription("List of all playlists in the iTunes library with metadata"),
		mcp.WithMIMEType("application/json"),
	)

	mcpServer.AddResource(dbStatsResource, dbStatsHandler)
	mcpServer.AddResource(playlistsResource, playlistsHandler)

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

func listPlaylistsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playlists, err := itunes.ListPlaylists()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list playlists: %v", err)), nil
	}

	// Convert to a more user-friendly format
	type PlaylistInfo struct {
		Name         string `json:"name"`
		PersistentID string `json:"persistent_id"`
		TrackCount   int    `json:"track_count"`
		Genre        string `json:"genre,omitempty"`
		SpecialKind  string `json:"special_kind,omitempty"`
	}

	playlistInfos := make([]PlaylistInfo, len(playlists))
	for i, p := range playlists {
		playlistInfos[i] = PlaylistInfo{
			Name:         p.Name,
			PersistentID: p.PersistentID,
			TrackCount:   p.TrackCount,
			Genre:        p.Genre,
			SpecialKind:  p.SpecialKind,
		}
	}

	result, err := json.MarshalIndent(playlistInfos, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal playlists: %v", err)), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

func getPlaylistTracksHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playlist, err := request.RequireString("playlist")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid playlist parameter: %v", err)), nil
	}

	useID := request.GetBool("use_id", false)

	tracks, err := itunes.GetPlaylistTracks(playlist, useID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get playlist tracks: %v", err)), nil
	}

	if len(tracks) == 0 {
		return mcp.NewToolResultText("No tracks found in the specified playlist."), nil
	}

	result, err := json.MarshalIndent(tracks, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal tracks: %v", err)), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

func searchAdvancedHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid query parameter: %v", err)), nil
	}

	// Build search filters from parameters
	filters := &database.SearchFilters{}

	if genre := request.GetString("genre", ""); genre != "" {
		filters.Genre = genre
	}

	if artist := request.GetString("artist", ""); artist != "" {
		filters.Artist = artist
	}

	if album := request.GetString("album", ""); album != "" {
		filters.Album = album
	}

	if playlist := request.GetString("playlist", ""); playlist != "" {
		filters.Playlist = playlist
	}

	if minRating := request.GetFloat("min_rating", 0); minRating > 0 {
		filters.MinRating = int(minRating)
	}

	// Check if starred parameter was provided
	args := request.GetArguments()
	if _, hasStarred := args["starred"]; hasStarred {
		starred := request.GetBool("starred", false)
		filters.Starred = &starred
	}

	// Check if streaming_only parameter was provided
	if _, hasStreamingOnly := args["streaming_only"]; hasStreamingOnly {
		streamingOnly := request.GetBool("streaming_only", false)
		filters.StreamingOnly = &streamingOnly
	}

	// Check if local_only parameter was provided
	if _, hasLocalOnly := args["local_only"]; hasLocalOnly {
		localOnly := request.GetBool("local_only", false)
		filters.LocalOnly = &localOnly
	}

	if limit := request.GetFloat("limit", 0); limit > 0 {
		filters.Limit = int(limit)
	}

	// Search using database with filters
	tracks, err := itunes.SearchTracksFromDatabase(query, filters)
	if err != nil {
		if errors.Is(err, itunes.ErrNoTracksFound) {
			return mcp.NewToolResultText("No tracks found matching the query and filters."), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Advanced search failed: %v", err)), nil
	}

	result, err := json.MarshalIndent(tracks, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

func playStreamHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		URL string `json:"url"`
	}

	argBytes, err := json.Marshal(request.Params.Arguments)
	if err != nil {
		return mcp.NewToolResultError("Failed to marshal arguments: " + err.Error()), nil
	}

	if err := json.Unmarshal(argBytes, &params); err != nil {
		return mcp.NewToolResultError("Invalid arguments: " + err.Error()), nil
	}

	if params.URL == "" {
		return mcp.NewToolResultError("URL parameter is required"), nil
	}

	// Play the stream using the new function
	result, err := itunes.PlayStreamURL(params.URL)
	if err != nil {
		return mcp.NewToolResultError("Failed to play stream: " + err.Error()), nil
	}

	// Return the play result as JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError("Failed to format result: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func searchStationsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		Query string `json:"query"`
	}

	argBytes, err := json.Marshal(request.Params.Arguments)
	if err != nil {
		return mcp.NewToolResultError("Failed to marshal arguments: " + err.Error()), nil
	}

	if err := json.Unmarshal(argBytes, &params); err != nil {
		return mcp.NewToolResultError("Invalid arguments: " + err.Error()), nil
	}

	if params.Query == "" {
		return mcp.NewToolResultError("Query parameter is required"), nil
	}

	// Search for stations using the new function
	result, err := itunes.SearchStations(params.Query)
	if err != nil {
		return mcp.NewToolResultError("Failed to search stations: " + err.Error()), nil
	}

	// Return the search result as JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError("Failed to format result: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func playlistsHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	playlists, err := itunes.ListPlaylists()
	if err != nil {
		return nil, fmt.Errorf("failed to get playlists: %w", err)
	}

	playlistsJSON, err := json.MarshalIndent(playlists, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal playlists: %w", err)
	}

	return []mcp.ResourceContents{
		&mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(playlistsJSON),
		},
	}, nil
}
