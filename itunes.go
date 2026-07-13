package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"itunes/database"
	"itunes/itunes"
	"itunes/logging"

	"go.uber.org/zap"
)

var logger *zap.Logger
var client *itunes.Client

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: itunes <command> [arguments]")
		fmt.Println("Commands:")
		fmt.Println("  search [--json] <query>            - Search iTunes library for tracks")
		fmt.Println("  play <collection> [track]         - Play album/playlist")
		fmt.Println("  search-stations <query>           - Search radio stations")
		fmt.Println("  add-station [options]             - Add a new radio station")
		fmt.Println("  update-station <id> [options]     - Update radio station")
		fmt.Println("  delete-station <id>               - Delete radio station")
		fmt.Println("  import-stations <file>            - Import stations from JSON file")
		fmt.Println("  export-stations <file>            - Export stations to JSON file")
		fmt.Println("  list-stations                     - List all radio stations")
		fmt.Println("")
		fmt.Println("Add/Update Station Options:")
		fmt.Println("  --name <name>        - Station name (required for add)")
		fmt.Println("  --url <url>          - Stream URL (required for add)")
		fmt.Println("  --description <desc> - Station description")
		fmt.Println("  --genre <genre>      - Station genre")
		fmt.Println("  --homepage <url>     - Station homepage URL (https:// web URL for browser access)")
		fmt.Println("\nEnvironment variables:")
		fmt.Println("  ITUNES_SEARCH_LIMIT=<num>  - Set search result limit (default: 15)")
		return
	}

	var err error
	logger, err = logging.InitLogger("info")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }() //nolint:errcheck // stderr sync errors are expected on macOS

	command := os.Args[1]

	// Initialize database (now default mode)
	client, err = itunes.NewClient(context.Background(), logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer func() { _ = client.Close() }() //nolint:errcheck // process shutdown cleanup

	// Get search limit from environment
	searchLimit := 15
	if limitStr := os.Getenv("ITUNES_SEARCH_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			searchLimit = limit
		}
	}

	dispatchCommand(context.Background(), command, os.Args[2:], searchLimit)
}

func dispatchCommand(ctx context.Context, command string, args []string, searchLimit int) {
	handlers := commandHandlers(ctx, args, searchLimit)
	if handler, ok := handlers[command]; ok {
		handler()
		return
	}
	printUnknownCommand(command)
}

func commandHandlers(ctx context.Context, args []string, searchLimit int) map[string]func() {
	return map[string]func(){
		"search":          func() { handleSearch(ctx, args, searchLimit) },
		"play":            func() { handlePlay(ctx, args) },
		"now-playing":     func() { handleNowPlaying(ctx) },
		"status":          func() { handleNowPlaying(ctx) },
		"search-stations": func() { handleSearchStations(ctx, args) },
		"add-station": func() {
			handleStationAction(args, handleAddStation, "Error adding station", "Radio station added successfully!")
		},
		"update-station": func() { handleUpdateStationCommand(args) },
		"delete-station": func() {
			handleSingleArgumentAction(args, "Usage: itunes delete-station <id>", handleDeleteStation,
				"Error deleting station", "Radio station deleted successfully!")
		},
		"import-stations": func() {
			handleSingleArgumentAction(args, "Usage: itunes import-stations <file>", handleImportStations,
				"Error importing stations", "Radio stations imported successfully!")
		},
		"export-stations": func() {
			handleSingleArgumentAction(args, "Usage: itunes export-stations <file>", handleExportStations,
				"Error exporting stations", "Radio stations exported successfully!")
		},
		"list-stations": func() { handleListStationsCommand() },
	}
}

func printUnknownCommand(command string) {
	fmt.Println("Unknown command:", command)
	fmt.Println("Available commands: search, play, now-playing, status, search-stations, add-station, " +
		"update-station, delete-station, import-stations, export-stations, list-stations")
}

func handleSearch(ctx context.Context, args []string, searchLimit int) {
	jsonMode := false
	var queryArgs []string
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
		} else {
			queryArgs = append(queryArgs, arg)
		}
	}
	if len(queryArgs) < 1 {
		fmt.Println("Usage: itunes search [--json] <query>")
		return
	}
	query := strings.Join(queryArgs, " ")
	tracks, err := client.SearchTracks(ctx, query)
	if err != nil {
		fmt.Printf("Error searching tracks: %v\n", err)
		return
	}
	if jsonMode {
		if tracks == nil {
			tracks = []itunes.Track{}
		}
		data, err := json.Marshal(tracks)
		if err != nil {
			fmt.Printf("Error encoding JSON: %v\n", err)
			return
		}
		fmt.Println(string(data))
		return
	}
	if len(tracks) == 0 {
		fmt.Println("No tracks found")
		return
	}
	fmt.Printf("Found %d tracks (limit: %d):\n", len(tracks), searchLimit)
	for _, track := range tracks {
		fmt.Printf("%s by %s [%s] (ID: %s)\n", track.Name, track.Artist, track.Collection, track.ID)
	}
}

func handlePlay(ctx context.Context, args []string) {
	if len(args) < 1 {
		printPlayUsage()
		return
	}
	playlist := args[0]
	var album, track, trackID string
	if len(args) > 1 {
		album = args[1]
	}
	if len(args) > 2 {
		track = args[2]
	}
	if len(args) > 3 {
		trackID = args[3]
	}
	if err := itunes.PlayPlaylistTrack(ctx, playlist, album, track, trackID); err != nil {
		fmt.Println("Play failed:", err)
	} else {
		fmt.Println("Playback started.")
	}
}

func printPlayUsage() {
	fmt.Println("Usage: itunes play <playlist> [album] [track] [trackID]")
	fmt.Println("  playlist: playlist name (use empty string \"\" if not applicable)")
	fmt.Println("  album: album name for album context (optional)")
	fmt.Println("  track: track name (optional)")
	fmt.Println("  trackID: track ID from search results (recommended, most reliable)")
}

func handleNowPlaying(ctx context.Context) {
	status, err := itunes.GetNowPlaying(ctx)
	if err != nil {
		fmt.Println("Failed to get current status:", err)
		return
	}
	fmt.Printf("Status: %s\n", status.Status)
	if status.Status == "playing" && status.Track != nil {
		fmt.Printf("Track: %s\n", status.Display)
		fmt.Printf("Album: %s\n", status.Track.Album)
		fmt.Printf("Position: %s / %s\n", status.Track.Position, status.Track.Duration)
		if status.Track.ID != "" {
			fmt.Printf("Track ID: %s\n", status.Track.ID)
		}
	} else if status.Message != "" {
		fmt.Printf("Message: %s\n", status.Message)
	}
}

func handleSearchStations(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: itunes search-stations <query>")
		return
	}
	query := args[0]
	result, err := client.SearchStations(ctx, query)
	if err != nil {
		fmt.Printf("Error searching stations: %v\n", err)
		return
	}
	if result.Count == 0 {
		fmt.Printf("No radio stations found for query: %s\n", query)
		if result.Message != "" {
			fmt.Printf("Hint: %s\n", result.Message)
		}
		return
	}
	fmt.Printf("Found %d radio station(s) for '%s':\n\n", result.Count, query)
	for _, station := range result.Stations {
		printStationSearchResult(station)
	}
}

func printStationSearchResult(station itunes.Station) {
	fmt.Printf("ID: %d\nName: %s\nURL: %s\n", station.ID, station.Name, station.URL)
	if station.Description != "" {
		fmt.Printf("Description: %s\n", station.Description)
	}
	if station.Genre != "" {
		fmt.Printf("Genre: %s\n", station.Genre)
	}
	if station.Homepage != "" {
		fmt.Printf("Homepage: %s\n", station.Homepage)
	}
	fmt.Println("---")
}

func handleStationAction(args []string, action func([]string) error, errorPrefix, success string) {
	if err := action(args); err != nil {
		fmt.Printf("%s: %v\n", errorPrefix, err)
		return
	}
	fmt.Println(success)
}

func handleUpdateStationCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: itunes update-station <id> [options]")
		return
	}
	handleStationAction(args, handleUpdateStation, "Error updating station", "Radio station updated successfully!")
}

func handleSingleArgumentAction(args []string, usage string, action func(string) error, errorPrefix, success string) {
	if len(args) < 1 {
		fmt.Println(usage)
		return
	}
	if err := action(args[0]); err != nil {
		fmt.Printf("%s: %v\n", errorPrefix, err)
		return
	}
	fmt.Println(success)
}

func handleListStationsCommand() {
	if err := handleListStations(); err != nil {
		fmt.Printf("Error listing stations: %v\n", err)
	}
}

// parseFlags parses command line flags for station management
func parseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for i := 0; i < len(args); i++ {
		if i+1 >= len(args) || !isStationFlag(args[i]) {
			continue
		}
		flags[strings.TrimPrefix(args[i], "--")] = args[i+1]
		i++
	}
	return flags
}

func isStationFlag(argument string) bool {
	switch argument {
	case "--name", "--url", "--description", "--genre", "--homepage":
		return true
	}
	return false
}

// validateStationURL validates that a station URL uses the correct format for Apple Music
func validateStationURL(url string) error {
	if url == "" {
		return nil // Empty URL is handled by required field validation
	}

	// Check for Apple Music station URLs
	if strings.Contains(url, "music.apple.com/") {
		// Apple Music URLs should use itmss:// protocol for proper playback
		if !strings.HasPrefix(url, "itmss://") {
			if strings.HasPrefix(url, "https://music.apple.com/") {
				return fmt.Errorf("Apple Music station URLs should use 'itmss://' protocol instead of 'https://' for proper playback. Example: %s",
					strings.Replace(url, "https://", "itmss://", 1)+"?app=music")
			}
			return fmt.Errorf("Apple Music station URLs should use 'itmss://' protocol for proper playback")
		}

		// Suggest adding ?app=music parameter if not present
		if !strings.Contains(url, "?app=music") && !strings.Contains(url, "&app=music") {
			return fmt.Errorf("Apple Music station URLs should include '?app=music' parameter for optimal compatibility. Example: %s?app=music", url)
		}
	}

	// Check for supported protocols
	supportedProtocols := []string{"http://", "https://", "itmss://"}
	isSupported := false
	for _, protocol := range supportedProtocols {
		if strings.HasPrefix(url, protocol) {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return fmt.Errorf("unsupported URL protocol. Supported protocols: %s", strings.Join(supportedProtocols, ", "))
	}

	return nil
}

func handleAddStation(args []string) error {
	flags := parseFlags(args)

	name := flags["name"]
	url := flags["url"]
	genre := flags["genre"]

	if name == "" {
		return fmt.Errorf("missing required flag: --name")
	}
	if url == "" {
		return fmt.Errorf("missing required flag: --url")
	}
	if genre == "" {
		return fmt.Errorf("missing required flag: --genre (and it cannot be empty)")
	}

	// Optional flags cannot be empty if provided
	if description, ok := flags["description"]; ok && description == "" {
		return fmt.Errorf("--description flag cannot be empty when provided")
	}
	if homepage, ok := flags["homepage"]; ok && homepage == "" {
		return fmt.Errorf("--homepage flag cannot be empty when provided")
	}

	// Validate URL format
	if err := validateStationURL(url); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	station := &database.RadioStation{
		Name:        name,
		URL:         url,
		Description: flags["description"],
		Genre:       genre,
		Homepage:    flags["homepage"],
	}

	return client.AddRadioStation(context.Background(), station)
}

func handleUpdateStation(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("station ID required")
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid station ID: %v", err)
	}

	flags := parseFlags(args[1:])

	// Check for empty flags
	for flag, value := range flags {
		if value == "" {
			return fmt.Errorf("--%s flag cannot be empty when updating", flag)
		}
	}

	// Get existing station
	existingStation, err := client.GetRadioStationByID(context.Background(), id)
	if err != nil {
		return fmt.Errorf("failed to get station: %w", err)
	}

	if err := applyStationUpdates(existingStation, flags); err != nil {
		return err
	}

	return client.UpdateRadioStation(context.Background(), id, existingStation)
}

func applyStationUpdates(station *database.RadioStation, flags map[string]string) error {
	if url, ok := flags["url"]; ok {
		if err := validateStationURL(url); err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		station.URL = url
	}
	if name, ok := flags["name"]; ok {
		station.Name = name
	}
	if description, ok := flags["description"]; ok {
		station.Description = description
	}
	if genre, ok := flags["genre"]; ok {
		station.Genre = genre
	}
	if homepage, ok := flags["homepage"]; ok {
		station.Homepage = homepage
	}
	return nil
}

func handleDeleteStation(idStr string) error {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid station ID: %v", err)
	}

	return client.DeleteRadioStation(context.Background(), id)
}

func handleImportStations(filename string) error {
	// Read JSON file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var stations []database.RadioStation
	if err := json.Unmarshal(data, &stations); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate stations before import
	for _, station := range stations {
		if err := validateStationURL(station.URL); err != nil {
			return fmt.Errorf("invalid station '%s': %w", station.Name, err)
		}
		if err := validateStationURL(station.Homepage); err != nil {
			return fmt.Errorf("invalid station '%s': %w", station.Name, err)
		}
	}

	return client.ImportRadioStations(context.Background(), stations)
}

func handleExportStations(filename string) error {
	// Get all stations
	stations, err := client.SearchRadioStations(context.Background(), "", &database.RadioStationFilters{Limit: 1000})
	if err != nil {
		return fmt.Errorf("failed to get stations: %w", err)
	}

	// Convert to JSON
	data, err := json.MarshalIndent(stations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	return os.WriteFile(filename, data, 0644)
}

func handleListStations() error {
	// Get all stations
	stations, err := client.SearchRadioStations(context.Background(), "", &database.RadioStationFilters{Limit: 100})
	if err != nil {
		return fmt.Errorf("failed to get stations: %w", err)
	}

	if len(stations) == 0 {
		fmt.Println("No radio stations found in database.")
		fmt.Println("Use 'itunes import-stations stations.json' to add a curated list.")
		return nil
	}

	fmt.Printf("Found %d radio station(s):\n\n", len(stations))
	for _, station := range stations {
		fmt.Printf("ID: %d\n", station.ID)
		fmt.Printf("Name: %s\n", station.Name)
		fmt.Printf("URL: %s\n", station.URL)
		if station.Description != "" {
			fmt.Printf("Description: %s\n", station.Description)
		}
		if station.Genre != "" {
			fmt.Printf("Genre: %s\n", station.Genre)
		}
		if station.Homepage != "" {
			fmt.Printf("Homepage: %s\n", station.Homepage)
		}
		fmt.Println("---")
	}

	return nil
}
