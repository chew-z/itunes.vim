package main

import (
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

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: itunes <command> [arguments]")
		fmt.Println("Commands:")
		fmt.Println("  search <query>                    - Search iTunes library for tracks")
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
	defer logger.Sync()

	command := os.Args[1]

	// Initialize database (now default mode)
	if err := itunes.InitDatabase(logger); err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer itunes.CloseDatabase()

	// Get search limit from environment
	searchLimit := 15
	if limitStr := os.Getenv("ITUNES_SEARCH_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			searchLimit = limit
		}
	}

	switch command {
	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes search <query>")
			return
		}
		query := os.Args[2]

		// Search using database
		tracks, err := itunes.SearchTracks(query)
		if err != nil {
			fmt.Printf("Error searching tracks: %v\n", err)
			return
		}

		if len(tracks) == 0 {
			fmt.Println("No tracks found")
			return
		}

		// Display results
		fmt.Printf("Found %d tracks (limit: %d):\n", len(tracks), searchLimit)
		for _, t := range tracks {
			fmt.Printf("%s by %s [%s] (ID: %s)\n", t.Name, t.Artist, t.Collection, t.ID)
		}

	case "play":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes play <playlist> [album] [track] [trackID]")
			fmt.Println("  playlist: playlist name (use empty string \"\" if not applicable)")
			fmt.Println("  album: album name for album context (optional)")
			fmt.Println("  track: track name (optional)")
			fmt.Println("  trackID: track ID from search results (recommended, most reliable)")
			return
		}

		// Parse arguments with support for empty strings
		playlist := os.Args[2]
		var album, track, trackID string

		if len(os.Args) > 3 {
			album = os.Args[3]
		}
		if len(os.Args) > 4 {
			track = os.Args[4]
		}
		if len(os.Args) > 5 {
			trackID = os.Args[5]
		}

		if err := itunes.PlayPlaylistTrack(playlist, album, track, trackID); err != nil {
			fmt.Println("Play failed:", err)
		} else {
			fmt.Println("Playback started.")
		}

	case "now-playing", "status":
		status, err := itunes.GetNowPlaying()
		if err != nil {
			fmt.Println("Failed to get current status:", err)
			return
		}

		if status.Status == "playing" && status.Track != nil {
			fmt.Printf("Status: %s\n", status.Status)
			fmt.Printf("Track: %s\n", status.Display)
			fmt.Printf("Album: %s\n", status.Track.Album)
			fmt.Printf("Position: %s / %s\n", status.Track.Position, status.Track.Duration)
			if status.Track.ID != "" {
				fmt.Printf("Track ID: %s\n", status.Track.ID)
			}
		} else {
			fmt.Printf("Status: %s\n", status.Status)
			if status.Message != "" {
				fmt.Printf("Message: %s\n", status.Message)
			}
		}

	case "search-stations":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes search-stations <query>")
			return
		}
		query := os.Args[2]

		result, err := itunes.SearchStations(query)
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

	case "add-station":
		err := handleAddStation(os.Args[2:])
		if err != nil {
			fmt.Printf("Error adding station: %v\n", err)
			return
		}
		fmt.Println("Radio station added successfully!")

	case "update-station":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes update-station <id> [options]")
			return
		}
		err := handleUpdateStation(os.Args[2:])
		if err != nil {
			fmt.Printf("Error updating station: %v\n", err)
			return
		}
		fmt.Println("Radio station updated successfully!")

	case "delete-station":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes delete-station <id>")
			return
		}
		err := handleDeleteStation(os.Args[2])
		if err != nil {
			fmt.Printf("Error deleting station: %v\n", err)
			return
		}
		fmt.Println("Radio station deleted successfully!")

	case "import-stations":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes import-stations <file>")
			return
		}
		err := handleImportStations(os.Args[2])
		if err != nil {
			fmt.Printf("Error importing stations: %v\n", err)
			return
		}
		fmt.Println("Radio stations imported successfully!")

	case "export-stations":
		if len(os.Args) < 3 {
			fmt.Println("Usage: itunes export-stations <file>")
			return
		}
		err := handleExportStations(os.Args[2])
		if err != nil {
			fmt.Printf("Error exporting stations: %v\n", err)
			return
		}
		fmt.Println("Radio stations exported successfully!")

	case "list-stations":
		err := handleListStations()
		if err != nil {
			fmt.Printf("Error listing stations: %v\n", err)
			return
		}

	default:
		fmt.Println("Unknown command:", command)
		fmt.Println("Available commands: search, play, now-playing, status, search-stations, add-station, update-station, delete-station, import-stations, export-stations, list-stations")
	}
}

// parseFlags parses command line flags for station management
func parseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for i := 0; i < len(args); i++ {
		if args[i] == "--name" && i+1 < len(args) {
			flags["name"] = args[i+1]
			i++
		} else if args[i] == "--url" && i+1 < len(args) {
			flags["url"] = args[i+1]
			i++
		} else if args[i] == "--description" && i+1 < len(args) {
			flags["description"] = args[i+1]
			i++
		} else if args[i] == "--genre" && i+1 < len(args) {
			flags["genre"] = args[i+1]
			i++
		} else if args[i] == "--homepage" && i+1 < len(args) {
			flags["homepage"] = args[i+1]
			i++
		}
	}
	return flags
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

	// Get database manager
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath, logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dm.Close()

	station := &database.RadioStation{
		Name:        name,
		URL:         url,
		Description: flags["description"],
		Genre:       genre,
		Homepage:    flags["homepage"],
	}

	return dm.AddRadioStation(station)
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

	// Get database manager
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath, logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dm.Close()

	// Get existing station
	existingStation, err := dm.GetRadioStationByID(id)
	if err != nil {
		return fmt.Errorf("failed to get station: %w", err)
	}

	// Update fields if provided
	if name, ok := flags["name"]; ok {
		existingStation.Name = name
	}
	if url, ok := flags["url"]; ok {
		if err := validateStationURL(url); err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		existingStation.URL = url
	}
	if description, ok := flags["description"]; ok {
		existingStation.Description = description
	}
	if genre, ok := flags["genre"]; ok {
		existingStation.Genre = genre
	}
	if homepage, ok := flags["homepage"]; ok {
		existingStation.Homepage = homepage
	}

	return dm.UpdateRadioStation(id, existingStation)
}

func handleDeleteStation(idStr string) error {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid station ID: %v", err)
	}

	// Get database manager
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath, logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dm.Close()

	return dm.DeleteRadioStation(id)
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

	// Get database manager
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath, logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dm.Close()

	return dm.ImportRadioStations(stations)
}

func handleExportStations(filename string) error {
	// Get database manager
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath, logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dm.Close()

	// Get all stations
	stations, err := dm.SearchRadioStations("", &database.RadioStationFilters{Limit: 1000})
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
	// Get database manager
	dm, err := database.NewDatabaseManager(database.PrimaryDBPath, logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dm.Close()

	// Get all stations
	stations, err := dm.SearchRadioStations("", &database.RadioStationFilters{Limit: 100})
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
