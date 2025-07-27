# Radio Stations Database Implementation Guide

This document provides step-by-step implementation instructions for refactoring the radio stations functionality from web scraping to a database-backed system.

## Overview

Replace the current web scraping approach in `SearchStations` with a SQLite database-backed system that provides:
- Fast, reliable station search using FTS5
- CLI-based station management 
- Pre-populated curated station database
- Integration with existing genre system

## Current State Analysis

### Files to Modify:
- `database/schema.go` - Add migration v3 for radio_stations table
- `itunes/itunes.go` - Replace SearchStations implementation 
- `itunes.go` - Add new CLI commands for station management
- `CLAUDE.md` - Update documentation

### Current SearchStations Function Location:
- File: `itunes/itunes.go`
- Function: `SearchStations(query string) (*StationSearchResult, error)`
- Current behavior: Web scraping Apple Music radio page
- Current structs: `Station`, `StationSearchResult`

## Implementation Steps

### Step 1: Database Schema Changes

#### 1.1 Add Migration v3 to `database/schema.go`

Add this migration to the `Schema` slice in `database/schema.go`:

```go
{
    Version:     3,
    Description: "Add radio stations table with genre integration",
    Up: `
    -- Radio stations table leveraging existing genres
    CREATE TABLE IF NOT EXISTS radio_stations (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        url TEXT NOT NULL UNIQUE,
        description TEXT,
        genre_id INTEGER,
        country TEXT,
        language TEXT,
        quality TEXT, -- e.g., "128k AAC", "320k MP3"
        homepage TEXT,
        verified_at DATETIME,
        is_active BOOLEAN DEFAULT TRUE,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (genre_id) REFERENCES genres(id)
    );

    -- FTS5 virtual table for radio station search
    CREATE VIRTUAL TABLE IF NOT EXISTS radio_stations_fts USING fts5(
        name,
        description,
        genre_name,
        country,
        language,
        tokenize='unicode61 remove_diacritics 2'
    );

    -- Triggers to keep FTS5 table in sync
    CREATE TRIGGER IF NOT EXISTS radio_stations_fts_insert AFTER INSERT ON radio_stations
    BEGIN
        INSERT INTO radio_stations_fts(rowid, name, description, genre_name, country, language)
        SELECT
            NEW.id,
            NEW.name,
            COALESCE(NEW.description, ''),
            COALESCE(g.name, 'Unknown'),
            COALESCE(NEW.country, ''),
            COALESCE(NEW.language, '')
        FROM genres g
        WHERE g.id = NEW.genre_id;
    END;

    CREATE TRIGGER IF NOT EXISTS radio_stations_fts_update AFTER UPDATE ON radio_stations
    BEGIN
        UPDATE radio_stations_fts
        SET name = NEW.name,
            description = COALESCE(NEW.description, ''),
            genre_name = COALESCE((SELECT name FROM genres WHERE id = NEW.genre_id), 'Unknown'),
            country = COALESCE(NEW.country, ''),
            language = COALESCE(NEW.language, '')
        WHERE rowid = NEW.id;
    END;

    CREATE TRIGGER IF NOT EXISTS radio_stations_fts_delete AFTER DELETE ON radio_stations
    BEGIN
        DELETE FROM radio_stations_fts WHERE rowid = OLD.id;
    END;

    -- Update timestamp trigger
    CREATE TRIGGER IF NOT EXISTS update_radio_stations_timestamp AFTER UPDATE ON radio_stations
    BEGIN
        UPDATE radio_stations SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

    -- Indexes for performance
    CREATE INDEX IF NOT EXISTS idx_radio_stations_genre_id ON radio_stations(genre_id);
    CREATE INDEX IF NOT EXISTS idx_radio_stations_country ON radio_stations(country);
    CREATE INDEX IF NOT EXISTS idx_radio_stations_active ON radio_stations(is_active);
    CREATE INDEX IF NOT EXISTS idx_radio_stations_verified ON radio_stations(verified_at);

    -- Update schema version
    INSERT INTO schema_migrations (version, description) VALUES (3, 'Add radio stations table with genre integration');
    `,
    Down: `
    DROP TRIGGER IF EXISTS update_radio_stations_timestamp;
    DROP TRIGGER IF EXISTS radio_stations_fts_delete;
    DROP TRIGGER IF EXISTS radio_stations_fts_update;
    DROP TRIGGER IF EXISTS radio_stations_fts_insert;
    DROP TABLE IF EXISTS radio_stations_fts;
    DROP INDEX IF EXISTS idx_radio_stations_verified;
    DROP INDEX IF EXISTS idx_radio_stations_active;
    DROP INDEX IF EXISTS idx_radio_stations_country;
    DROP INDEX IF EXISTS idx_radio_stations_genre_id;
    DROP TABLE IF EXISTS radio_stations;
    DELETE FROM schema_migrations WHERE version = 3;
    `,
},
```

#### 1.2 Update SchemaVersion Constant

In `database/schema.go`, update:
```go
const SchemaVersion = 3  // Change from 2 to 3
```

### Step 2: Add RadioStation Struct and Database Functions

#### 2.1 Add RadioStation struct to `database/database.go`

Add after the existing `Playlist` struct:

```go
// RadioStation represents a radio station with metadata
type RadioStation struct {
    ID          int64
    Name        string
    URL         string
    Description string
    Genre       string
    GenreID     int64
    Country     string
    Language    string
    Quality     string
    Homepage    string
    VerifiedAt  *time.Time
    IsActive    bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// RadioStationFilters contains search parameters for radio stations
type RadioStationFilters struct {
    Genre    string
    Country  string
    Language string
    Active   *bool
    Limit    int
}
```

#### 2.2 Add Database Functions to `database/database.go`

Add these functions at the end of the file:

```go
// SearchRadioStations searches for radio stations using FTS5
func (dm *DatabaseManager) SearchRadioStations(query string, filters *RadioStationFilters) ([]RadioStation, error) {
    var stations []RadioStation
    
    if filters == nil {
        filters = &RadioStationFilters{}
    }
    
    if filters.Limit == 0 {
        filters.Limit = 15
    }

    // Build search query
    searchQuery := `
        SELECT DISTINCT rs.id, rs.name, rs.url, rs.description, 
               COALESCE(g.name, '') as genre, rs.genre_id,
               COALESCE(rs.country, '') as country,
               COALESCE(rs.language, '') as language,
               COALESCE(rs.quality, '') as quality,
               COALESCE(rs.homepage, '') as homepage,
               rs.verified_at, rs.is_active, rs.created_at, rs.updated_at
        FROM radio_stations rs
        LEFT JOIN genres g ON rs.genre_id = g.id
        LEFT JOIN radio_stations_fts fts ON rs.id = fts.rowid
        WHERE rs.is_active = 1
    `
    
    args := []interface{}{}
    argIndex := 1

    // Add FTS5 search if query provided
    if query != "" {
        searchQuery += ` AND fts.radio_stations_fts MATCH ?`
        args = append(args, query)
        argIndex++
    }

    // Add filters
    if filters.Genre != "" {
        searchQuery += ` AND g.name LIKE ?`
        args = append(args, "%"+filters.Genre+"%")
        argIndex++
    }

    if filters.Country != "" {
        searchQuery += ` AND rs.country = ?`
        args = append(args, filters.Country)
        argIndex++
    }

    if filters.Language != "" {
        searchQuery += ` AND rs.language = ?`
        args = append(args, filters.Language)
        argIndex++
    }

    // Order by relevance if FTS5 search, otherwise by name
    if query != "" {
        searchQuery += ` ORDER BY bm25(fts)`
    } else {
        searchQuery += ` ORDER BY rs.name`
    }

    searchQuery += ` LIMIT ?`
    args = append(args, filters.Limit)

    rows, err := dm.DB.Query(searchQuery, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to search radio stations: %w", err)
    }
    defer rows.Close()

    for rows.Next() {
        var station RadioStation
        err := rows.Scan(
            &station.ID, &station.Name, &station.URL, &station.Description,
            &station.Genre, &station.GenreID, &station.Country, &station.Language,
            &station.Quality, &station.Homepage, &station.VerifiedAt,
            &station.IsActive, &station.CreatedAt, &station.UpdatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan radio station: %w", err)
        }
        stations = append(stations, station)
    }

    return stations, nil
}

// AddRadioStation adds a new radio station to the database
func (dm *DatabaseManager) AddRadioStation(station *RadioStation) error {
    // Get or create genre
    var genreID int64
    if station.Genre != "" {
        err := dm.DB.QueryRow("SELECT id FROM genres WHERE name = ?", station.Genre).Scan(&genreID)
        if err == sql.ErrNoRows {
            // Create new genre
            result, err := dm.DB.Exec("INSERT INTO genres (name) VALUES (?)", station.Genre)
            if err != nil {
                return fmt.Errorf("failed to create genre: %w", err)
            }
            genreID, _ = result.LastInsertId()
        } else if err != nil {
            return fmt.Errorf("failed to query genre: %w", err)
        }
    }

    query := `
        INSERT INTO radio_stations (name, url, description, genre_id, country, language, quality, homepage, is_active)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
    
    _, err := dm.DB.Exec(query, station.Name, station.URL, station.Description,
        genreID, station.Country, station.Language, station.Quality, station.Homepage, true)
    
    if err != nil {
        return fmt.Errorf("failed to add radio station: %w", err)
    }
    
    return nil
}

// UpdateRadioStation updates an existing radio station
func (dm *DatabaseManager) UpdateRadioStation(id int64, station *RadioStation) error {
    // Get or create genre if provided
    var genreID *int64
    if station.Genre != "" {
        var gID int64
        err := dm.DB.QueryRow("SELECT id FROM genres WHERE name = ?", station.Genre).Scan(&gID)
        if err == sql.ErrNoRows {
            // Create new genre
            result, err := dm.DB.Exec("INSERT INTO genres (name) VALUES (?)", station.Genre)
            if err != nil {
                return fmt.Errorf("failed to create genre: %w", err)
            }
            gID, _ = result.LastInsertId()
        } else if err != nil {
            return fmt.Errorf("failed to query genre: %w", err)
        }
        genreID = &gID
    }

    query := `
        UPDATE radio_stations 
        SET name = ?, url = ?, description = ?, genre_id = ?, country = ?, 
            language = ?, quality = ?, homepage = ?, updated_at = CURRENT_TIMESTAMP
        WHERE id = ?
    `
    
    _, err := dm.DB.Exec(query, station.Name, station.URL, station.Description,
        genreID, station.Country, station.Language, station.Quality, station.Homepage, id)
    
    if err != nil {
        return fmt.Errorf("failed to update radio station: %w", err)
    }
    
    return nil
}

// DeleteRadioStation removes a radio station
func (dm *DatabaseManager) DeleteRadioStation(id int64) error {
    _, err := dm.DB.Exec("DELETE FROM radio_stations WHERE id = ?", id)
    if err != nil {
        return fmt.Errorf("failed to delete radio station: %w", err)
    }
    return nil
}

// GetRadioStationByID retrieves a radio station by ID
func (dm *DatabaseManager) GetRadioStationByID(id int64) (*RadioStation, error) {
    var station RadioStation
    query := `
        SELECT rs.id, rs.name, rs.url, rs.description, 
               COALESCE(g.name, '') as genre, rs.genre_id,
               COALESCE(rs.country, '') as country,
               COALESCE(rs.language, '') as language,
               COALESCE(rs.quality, '') as quality,
               COALESCE(rs.homepage, '') as homepage,
               rs.verified_at, rs.is_active, rs.created_at, rs.updated_at
        FROM radio_stations rs
        LEFT JOIN genres g ON rs.genre_id = g.id
        WHERE rs.id = ?
    `
    
    err := dm.DB.QueryRow(query, id).Scan(
        &station.ID, &station.Name, &station.URL, &station.Description,
        &station.Genre, &station.GenreID, &station.Country, &station.Language,
        &station.Quality, &station.Homepage, &station.VerifiedAt,
        &station.IsActive, &station.CreatedAt, &station.UpdatedAt,
    )
    
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("radio station not found")
        }
        return nil, fmt.Errorf("failed to get radio station: %w", err)
    }
    
    return &station, nil
}

// ImportRadioStations bulk imports radio stations from a slice
func (dm *DatabaseManager) ImportRadioStations(stations []RadioStation) error {
    tx, err := dm.DB.Begin()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    for _, station := range stations {
        // Get or create genre
        var genreID *int64
        if station.Genre != "" {
            var gID int64
            err := tx.QueryRow("SELECT id FROM genres WHERE name = ?", station.Genre).Scan(&gID)
            if err == sql.ErrNoRows {
                result, err := tx.Exec("INSERT INTO genres (name) VALUES (?)", station.Genre)
                if err != nil {
                    return fmt.Errorf("failed to create genre %s: %w", station.Genre, err)
                }
                gID, _ = result.LastInsertId()
            } else if err != nil {
                return fmt.Errorf("failed to query genre %s: %w", station.Genre, err)
            }
            genreID = &gID
        }

        // Insert station
        _, err := tx.Exec(`
            INSERT OR IGNORE INTO radio_stations (name, url, description, genre_id, country, language, quality, homepage, is_active)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        `, station.Name, station.URL, station.Description, genreID, station.Country, station.Language, station.Quality, station.Homepage, true)
        
        if err != nil {
            return fmt.Errorf("failed to insert station %s: %w", station.Name, err)
        }
    }

    return tx.Commit()
}
```

### Step 3: Replace SearchStations Function in itunes/itunes.go

#### 3.1 Update Station struct (if needed)

Ensure the `Station` struct in `itunes/itunes.go` includes all necessary fields:

```go
// Station represents an Apple Music radio station
type Station struct {
    ID          int64    `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    URL         string   `json:"url"`
    Genre       string   `json:"genre"`
    Country     string   `json:"country,omitempty"`
    Language    string   `json:"language,omitempty"`
    Quality     string   `json:"quality,omitempty"`
    Homepage    string   `json:"homepage,omitempty"`
    Keywords    []string `json:"keywords"` // For backward compatibility
}
```

#### 3.2 Replace SearchStations Function

Replace the entire `SearchStations` function and the `scrapeAppleMusicStations` function with:

```go
// SearchStations searches for radio stations in the database
func SearchStations(query string) (*StationSearchResult, error) {
    if dbManager == nil {
        return nil, errors.New("database not initialized - please run InitDatabase() first")
    }

    filters := &database.RadioStationFilters{
        Limit: 15, // Default limit, can be made configurable
    }

    stations, err := dbManager.SearchRadioStations(query, filters)
    if err != nil {
        return nil, fmt.Errorf("failed to search radio stations: %w", err)
    }

    // Convert database stations to API stations
    var apiStations []Station
    for _, dbStation := range stations {
        apiStation := Station{
            ID:          dbStation.ID,
            Name:        dbStation.Name,
            Description: dbStation.Description,
            URL:         dbStation.URL,
            Genre:       dbStation.Genre,
            Country:     dbStation.Country,
            Language:    dbStation.Language,
            Quality:     dbStation.Quality,
            Homepage:    dbStation.Homepage,
            Keywords:    []string{}, // Legacy field for compatibility
        }
        apiStations = append(apiStations, apiStation)
    }

    result := &StationSearchResult{
        Status:   "success",
        Query:    query,
        Stations: apiStations,
        Count:    len(apiStations),
    }

    if len(apiStations) == 0 {
        result.Status = "no_results"
        result.Message = "No radio stations found matching the query"
    }

    return result, nil
}
```

#### 3.3 Remove Web Scraping Function

Delete the entire `scrapeAppleMusicStations` function and all its dependencies.

### Step 4: Add CLI Commands to itunes.go

#### 4.1 Update Usage Information

In the main function of `itunes.go`, update the usage information:

```go
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
    fmt.Println("  --country <country>  - Country code (e.g., US, UK)")
    fmt.Println("  --language <lang>    - Language (e.g., English, Spanish)")
    fmt.Println("  --quality <quality>  - Stream quality (e.g., 128k AAC)")
    fmt.Println("  --homepage <url>     - Station homepage URL")
    fmt.Println("\nEnvironment variables:")
    fmt.Println("  ITUNES_SEARCH_LIMIT=<num>  - Set search result limit (default: 15)")
    return
}
```

#### 4.2 Add Station Management Commands

Add these cases to the switch statement in main():

```go
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
        if station.Country != "" {
            fmt.Printf("Country: %s\n", station.Country)
        }
        if station.Quality != "" {
            fmt.Printf("Quality: %s\n", station.Quality)
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
```

#### 4.3 Add CLI Helper Functions

Add these functions at the end of `itunes.go`:

```go
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
        } else if args[i] == "--country" && i+1 < len(args) {
            flags["country"] = args[i+1]
            i++
        } else if args[i] == "--language" && i+1 < len(args) {
            flags["language"] = args[i+1]
            i++
        } else if args[i] == "--quality" && i+1 < len(args) {
            flags["quality"] = args[i+1]
            i++
        } else if args[i] == "--homepage" && i+1 < len(args) {
            flags["homepage"] = args[i+1]
            i++
        }
    }
    return flags
}

func handleAddStation(args []string) error {
    flags := parseFlags(args)
    
    name := flags["name"]
    url := flags["url"]
    
    if name == "" || url == "" {
        return fmt.Errorf("--name and --url are required")
    }
    
    // Get database manager
    dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }
    defer dm.Close()
    
    station := &database.RadioStation{
        Name:        name,
        URL:         url,
        Description: flags["description"],
        Genre:       flags["genre"],
        Country:     flags["country"],
        Language:    flags["language"],
        Quality:     flags["quality"],
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
    
    // Get database manager
    dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
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
    if name := flags["name"]; name != "" {
        existingStation.Name = name
    }
    if url := flags["url"]; url != "" {
        existingStation.URL = url
    }
    if description := flags["description"]; description != "" {
        existingStation.Description = description
    }
    if genre := flags["genre"]; genre != "" {
        existingStation.Genre = genre
    }
    if country := flags["country"]; country != "" {
        existingStation.Country = country
    }
    if language := flags["language"]; language != "" {
        existingStation.Language = language
    }
    if quality := flags["quality"]; quality != "" {
        existingStation.Quality = quality
    }
    if homepage := flags["homepage"]; homepage != "" {
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
    dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
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
    dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }
    defer dm.Close()
    
    return dm.ImportRadioStations(stations)
}

func handleExportStations(filename string) error {
    // Get database manager
    dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
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
    dm, err := database.NewDatabaseManager(database.PrimaryDBPath)
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
        if station.Country != "" {
            fmt.Printf("Country: %s\n", station.Country)
        }
        if station.Quality != "" {
            fmt.Printf("Quality: %s\n", station.Quality)
        }
        fmt.Println("---")
    }
    
    return nil
}
```

### Step 5: Create Initial Station Data

#### 5.1 Create `stations.json` in Project Root

Create a file named `stations.json` with curated radio stations:

```json
[
  {
    "name": "SomaFM - Groove Salad",
    "url": "http://ice6.somafm.com/groovesalad-128-aac",
    "description": "A nicely chilled plate of ambient downtempo beats and grooves",
    "genre": "Electronic",
    "country": "US",
    "language": "English",
    "quality": "128k AAC",
    "homepage": "https://somafm.com/groovesalad/"
  },
  {
    "name": "SomaFM - Deep Space One",
    "url": "http://ice6.somafm.com/deepspaceone-128-aac",
    "description": "Deep ambient electronic, experimental and space music",
    "genre": "Ambient",
    "country": "US",
    "language": "English",
    "quality": "128k AAC",
    "homepage": "https://somafm.com/deepspaceone/"
  },
  {
    "name": "Radio Paradise",
    "url": "http://stream.radioparadise.com/aac-320",
    "description": "DJ-mixed modern & classic rock, world, electronica & more",
    "genre": "Alternative",
    "country": "US",
    "language": "English",
    "quality": "320k AAC",
    "homepage": "https://radioparadise.com/"
  },
  {
    "name": "KEXP 90.3 FM",
    "url": "http://live-aacplus-64.kexp.org/kexp64.aac",
    "description": "Seattle's Music Discovery",
    "genre": "Alternative",
    "country": "US",
    "language": "English",
    "quality": "64k AAC",
    "homepage": "https://kexp.org/"
  },
  {
    "name": "BBC Radio 6 Music",
    "url": "http://stream.live.vc.bbcmedia.co.uk/bbc_6music",
    "description": "Alternative music from the BBC",
    "genre": "Alternative",
    "country": "UK",
    "language": "English",
    "quality": "128k AAC",
    "homepage": "https://www.bbc.co.uk/6music"
  },
  {
    "name": "Jazz24",
    "url": "http://jazz24-32.streamguys1.com:9000/live",
    "description": "24/7 jazz music from around the world",
    "genre": "Jazz",
    "country": "US",
    "language": "English",
    "quality": "128k MP3",
    "homepage": "https://jazz24.org/"
  },
  {
    "name": "Radio Swiss Jazz",
    "url": "http://stream.srg-ssr.ch/m/rsj/mp3_128",
    "description": "The best of jazz music",
    "genre": "Jazz",
    "country": "CH",
    "language": "English",
    "quality": "128k MP3",
    "homepage": "https://www.radioswissjazz.ch/"
  },
  {
    "name": "Chillhop Radio",
    "url": "http://streaming.chillhop.com/chillhop.m3u8",
    "description": "Chill hop beats to study and relax to",
    "genre": "Hip-Hop",
    "country": "NL",
    "language": "English",
    "quality": "128k AAC",
    "homepage": "https://chillhop.com/"
  },
  {
    "name": "Classical KUSC",
    "url": "http://kjazz.streamguys1.com/kjazzmix-128-mp3",
    "description": "Classical music from Southern California",
    "genre": "Classical",
    "country": "US",
    "language": "English",
    "quality": "128k MP3",
    "homepage": "https://www.kusc.org/"
  },
  {
    "name": "FIP",
    "url": "http://direct.fipradio.fr/live/fip-midfi.mp3",
    "description": "Eclectic music selection from Radio France",
    "genre": "World",
    "country": "FR",
    "language": "French",
    "quality": "128k MP3",
    "homepage": "https://www.fip.fr/"
  }
]
```

### Step 6: Update Documentation

#### 6.1 Update CLAUDE.md

Add to the MCP Tools section:

```markdown
### `search_stations`
- **Description**: Search for radio stations by genre, name, or keywords using database instead of web scraping
- **Parameters**: `query` (string, required) - Search query for stations (e.g., 'jazz', 'ambient', 'electronic')
- **Returns**: JSON object with matching stations including enhanced metadata (ID, name, description, URL, genre, country, quality)
- **Note**: Now uses fast SQLite database search instead of web scraping for better performance and more comprehensive results
```

Add to the Build and Development Commands section:

```bash
# Radio Station Management
./bin/itunes search-stations "jazz"                    # Search radio stations
./bin/itunes add-station --name "My Radio" --url "http://example.com/stream" --genre "Rock"
./bin/itunes update-station 123 --description "Updated description"
./bin/itunes delete-station 123
./bin/itunes import-stations stations.json             # Import from JSON file
./bin/itunes export-stations backup.json               # Export to JSON file
./bin/itunes list-stations                             # List all stations
```

### Step 7: Testing and Verification

#### 7.1 Test Database Migration

```bash
# Build and test the migration
go build -o bin/itunes itunes.go
go build -o bin/mcp-itunes ./mcp-server

# The migration should automatically run when the database is initialized
./bin/itunes search "test"  # This should trigger migration v3
```

#### 7.2 Test Station Management

```bash
# Import initial stations
./bin/itunes import-stations stations.json

# Search stations
./bin/itunes search-stations "jazz"
./bin/itunes search-stations "ambient"

# Add a custom station
./bin/itunes add-station --name "Test Station" --url "http://example.com/stream" --genre "Test" --country "US"

# List all stations
./bin/itunes list-stations

# Test MCP functionality
./bin/mcp-itunes
# In another terminal, test the search_stations MCP tool
```

#### 7.3 Verify MCP Integration

The existing `search_stations` MCP tool should now use the database backend automatically without any changes to the MCP interface.

### Step 8: Add Import Command to Usage

Add this to the build commands documentation:

```bash
# Initial setup after database migration
./bin/itunes import-stations stations.json   # Import curated radio stations
```

## Implementation Checklist

- [ ] Add migration v3 to `database/schema.go`
- [ ] Update SchemaVersion constant to 3
- [ ] Add RadioStation struct and functions to `database/database.go`
- [ ] Replace SearchStations function in `itunes/itunes.go`
- [ ] Remove scrapeAppleMusicStations function
- [ ] Add CLI commands to `itunes.go`
- [ ] Add CLI helper functions to `itunes.go`
- [ ] Create `stations.json` with initial data
- [ ] Update CLAUDE.md documentation
- [ ] Test database migration
- [ ] Test CLI commands
- [ ] Verify MCP integration
- [ ] Import initial station data

## Expected Results

After implementation:
- Fast radio station search using SQLite FTS5 (<5ms queries)
- No dependency on web scraping
- Rich station metadata with genres, countries, quality info
- CLI-based station management
- Bulk import/export functionality
- Backward compatible MCP interface
- Extensible system for adding new stations

This refactor transforms the radio station functionality from a limited web scraping approach into a comprehensive, fast, and maintainable database-backed system.