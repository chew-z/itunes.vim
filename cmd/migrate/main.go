package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"itunes/database"
	"itunes/itunes"
)

var (
	defaultCacheDir = filepath.Join(os.TempDir(), "itunes-cache")
	defaultDBPath   = "~/Music/iTunes/itunes_library.db"
)

// CLI flags
var (
	cacheDir   = flag.String("cache", defaultCacheDir, "Path to JSON cache directory")
	dbPath     = flag.String("db", defaultDBPath, "Path to SQLite database file")
	verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	dryRun     = flag.Bool("dry-run", false, "Perform a dry run without making changes")
	validate   = flag.Bool("validate", false, "Validate existing database")
	fromScript = flag.Bool("from-script", false, "Run refresh script and populate database directly")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "iTunes Library Migration Tool\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Migrate from JSON cache to SQLite\n")
		fmt.Fprintf(os.Stderr, "  %s\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Migrate with verbose output\n")
		fmt.Fprintf(os.Stderr, "  %s -verbose\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Validate existing database\n")
		fmt.Fprintf(os.Stderr, "  %s -validate\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Refresh from Apple Music and migrate\n")
		fmt.Fprintf(os.Stderr, "  %s -from-script\n", os.Args[0])
	}
	flag.Parse()

	// Configure logging
	if !*verbose {
		log.SetFlags(0)
	}

	// Handle validation mode
	if *validate {
		if err := validateDatabase(); err != nil {
			log.Fatalf("Validation failed: %v", err)
		}
		return
	}

	// Handle migration
	if *fromScript {
		if err := migrateFromScript(); err != nil {
			log.Fatalf("Migration from script failed: %v", err)
		}
	} else {
		if err := migrateFromCache(); err != nil {
			log.Fatalf("Migration from cache failed: %v", err)
		}
	}
}

// progressReporter creates a progress callback function
func progressReporter(verbose bool) database.ProgressCallback {
	lastUpdate := time.Now()

	return func(progress database.MigrationProgress) {
		now := time.Now()
		// Update every second or when complete
		if now.Sub(lastUpdate) < time.Second &&
			progress.ProcessedTracks < progress.TotalTracks {
			return
		}
		lastUpdate = now

		// Calculate progress percentages
		trackPercent := float64(progress.ProcessedTracks) / float64(progress.TotalTracks) * 100
		playlistPercent := float64(progress.ProcessedPlaylists) / float64(progress.TotalPlaylists) * 100

		fmt.Printf("\rTracks: %d/%d (%.1f%%) | Playlists: %d/%d (%.1f%%) | Errors: %d",
			progress.ProcessedTracks, progress.TotalTracks, trackPercent,
			progress.ProcessedPlaylists, progress.TotalPlaylists, playlistPercent,
			len(progress.Errors))

		if progress.ProcessedTracks >= progress.TotalTracks {
			fmt.Println() // New line after completion

			// Print summary
			duration := time.Since(progress.StartTime)
			fmt.Printf("\nMigration completed in %s\n", duration.Round(time.Second))

			if len(progress.Errors) > 0 {
				fmt.Printf("\nEncountered %d errors during migration:\n", len(progress.Errors))
				if verbose {
					for i, err := range progress.Errors {
						if i >= 10 && !verbose {
							fmt.Printf("  ... and %d more errors\n", len(progress.Errors)-10)
							break
						}
						fmt.Printf("  - %v\n", err)
					}
				} else {
					fmt.Println("  Run with -verbose to see error details")
				}
			}
		}
	}
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// migrateFromCache migrates from JSON cache files
func migrateFromCache() error {
	log.Printf("Migrating from cache directory: %s", *cacheDir)

	// Check if cache files exist
	enhancedPath := filepath.Join(*cacheDir, "library_enhanced.json")
	libraryPath := filepath.Join(*cacheDir, "library.json")

	var trackCount int
	if data, err := os.ReadFile(enhancedPath); err == nil {
		// Try to parse enhanced format
		var enhanced database.RefreshResponse
		if err := json.Unmarshal(data, &enhanced); err == nil {
			trackCount = len(enhanced.Data.Tracks)
		}
	} else if data, err := os.ReadFile(libraryPath); err == nil {
		// Try to parse regular format
		var tracks []itunes.Track
		if err := json.Unmarshal(data, &tracks); err == nil {
			trackCount = len(tracks)
		}
	} else {
		return fmt.Errorf("no cache files found in %s", *cacheDir)
	}

	fmt.Printf("Found %d tracks in cache file\n", trackCount)

	// Analyze cache if dry run
	if *dryRun {
		return analyzeCache(*cacheDir)
	}

	// Open database
	dbManager, err := database.NewDatabaseManager(expandPath(*dbPath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dbManager.Close()

	// Run migration
	fmt.Println("Starting migration to SQLite database...")
	if err := dbManager.MigrateFromJSON(*cacheDir, progressReporter(*verbose)); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Validate migration
	fmt.Println("\nValidating migration...")
	valid, issues := dbManager.ValidateMigration(*cacheDir)
	if !valid {
		return fmt.Errorf("migration validation failed: %s", strings.Join(issues, "; "))
	}

	// Get final stats
	stats, err := dbManager.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get database stats: %w", err)
	}

	fmt.Println("\nDatabase Statistics:")
	fmt.Printf("  Tracks:    %d\n", stats.TrackCount)
	fmt.Printf("  Playlists: %d\n", stats.PlaylistCount)
	fmt.Printf("  Artists:   %d\n", stats.ArtistCount)
	fmt.Printf("  Albums:    %d\n", stats.AlbumCount)
	fmt.Printf("  Genres:    %d\n", stats.GenreCount)
	fmt.Printf("  Size:      %.2f MB\n", float64(stats.DatabaseSize)/(1024*1024))

	fmt.Println("\nMigration completed successfully!")
	return nil
}

// migrateFromScript runs the refresh script and migrates directly
func migrateFromScript() error {
	log.Println("Running library refresh script...")

	// Dry run - just show what would be done
	if *dryRun {
		fmt.Println("Would run: itunes.RefreshLibraryCache()")
		fmt.Println("Would migrate results to:", expandPath(*dbPath))
		return nil
	}

	// Run refresh script
	startTime := time.Now()
	if err := itunes.RefreshLibraryCache(); err != nil {
		return fmt.Errorf("failed to refresh library: %w", err)
	}

	fmt.Printf("Library refresh completed in %s\n", time.Since(startTime).Round(time.Second))

	// Read the enhanced cache file
	enhancedPath := filepath.Join(*cacheDir, "library_enhanced.json")
	data, err := os.ReadFile(enhancedPath)
	if err != nil {
		return fmt.Errorf("failed to read enhanced cache: %w", err)
	}

	var enhanced database.RefreshResponse
	if err := json.Unmarshal(data, &enhanced); err != nil {
		return fmt.Errorf("failed to parse refresh response: %w", err)
	}

	fmt.Printf("Loaded %d tracks and %d playlists from refresh\n",
		len(enhanced.Data.Tracks), len(enhanced.Data.Playlists))

	// Open database
	dbManager, err := database.NewDatabaseManager(expandPath(*dbPath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dbManager.Close()

	// Populate database
	fmt.Println("Populating database...")
	if err := dbManager.PopulateFromRefreshScript(&enhanced, progressReporter(*verbose)); err != nil {
		return fmt.Errorf("failed to populate database: %w", err)
	}

	// Validate
	fmt.Println("\nValidating migration...")
	valid, issues := dbManager.ValidateMigration(*cacheDir)
	if !valid {
		return fmt.Errorf("migration validation failed: %s", strings.Join(issues, "; "))
	}

	fmt.Println("\nMigration from script completed successfully!")
	return nil
}

// analyzeCache analyzes cache files without migrating
func analyzeCache(cacheDir string) error {
	fmt.Println("Analyzing cache files...")

	// Try enhanced format first
	enhancedPath := filepath.Join(cacheDir, "library_enhanced.json")
	if data, err := os.ReadFile(enhancedPath); err == nil {
		var response database.RefreshResponse
		if err := json.Unmarshal(data, &response); err == nil {
			fmt.Println("\nEnhanced Library Cache:")
			fmt.Printf("  Status: %s\n", response.Status)
			fmt.Printf("  Tracks: %d\n", len(response.Data.Tracks))
			fmt.Printf("  Playlists: %d\n", len(response.Data.Playlists))
			fmt.Printf("  Refresh Time: %s\n", response.Data.Stats.RefreshTime)

			// Sample tracks
			fmt.Println("\nSample Tracks:")
			for i, track := range response.Data.Tracks {
				if i >= 5 {
					break
				}
				fmt.Printf("  - %s by %s (ID: %s)\n", track.Name, track.Artist, track.PersistentID)
			}

			// Sample playlists
			fmt.Println("\nSample Playlists:")
			for i, playlist := range response.Data.Playlists {
				if i >= 5 {
					break
				}
				fmt.Printf("  - %s (%d tracks, ID: %s)\n", playlist.Name, playlist.TrackCount, playlist.ID)
			}

			return nil
		}
	}

	// Try regular format
	libraryPath := filepath.Join(cacheDir, "library.json")
	if data, err := os.ReadFile(libraryPath); err == nil {
		var tracks []itunes.Track
		if err := json.Unmarshal(data, &tracks); err == nil {
			fmt.Println("\nRegular Library Cache:")
			fmt.Printf("  Tracks: %d\n", len(tracks))

			// Check for persistent IDs
			withPersistentID := 0
			for _, track := range tracks {
				if track.ID != "" {
					withPersistentID++
				}
			}
			fmt.Printf("  Tracks with Persistent IDs: %d (%.1f%%)\n",
				withPersistentID, float64(withPersistentID)/float64(len(tracks))*100)

			// Sample tracks
			fmt.Println("\nSample Tracks:")
			for i, track := range tracks {
				if i >= 5 {
					break
				}
				idStr := track.PersistentID
				if idStr == "" {
					idStr = track.ID
				}
				fmt.Printf("  - %s by %s (ID: %s)\n", track.Name, track.Artist, idStr)
			}

			return nil
		}
	}

	return fmt.Errorf("no valid cache files found in %s", cacheDir)
}

// validateDatabase validates an existing database
func validateDatabase() error {
	log.Printf("Validating database: %s", *dbPath)

	// Open database
	dm, err := database.NewDatabaseManager(expandPath(*dbPath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer dm.Close()

	// Get statistics
	stats, err := dm.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get database stats: %w", err)
	}

	fmt.Println("Database Statistics:")
	fmt.Printf("  Tracks:    %d\n", stats.TrackCount)
	fmt.Printf("  Playlists: %d\n", stats.PlaylistCount)
	fmt.Printf("  Artists:   %d\n", stats.ArtistCount)
	fmt.Printf("  Albums:    %d\n", stats.AlbumCount)
	fmt.Printf("  Genres:    %d\n", stats.GenreCount)
	fmt.Printf("  Size:      %.2f MB\n", float64(stats.DatabaseSize)/(1024*1024))

	// Run validation
	fmt.Println("\nRunning validation checks...")

	// Check for tracks without persistent IDs
	var tracksWithoutID int
	err = dm.DB.QueryRow(`
		SELECT COUNT(*) FROM tracks
		WHERE persistent_id IS NULL OR persistent_id = ''
	`).Scan(&tracksWithoutID)
	if err != nil {
		return fmt.Errorf("failed to check tracks without IDs: %w", err)
	}

	if tracksWithoutID > 0 {
		fmt.Printf("  ⚠️  Found %d tracks without persistent IDs\n", tracksWithoutID)
	} else {
		fmt.Println("  ✓ All tracks have persistent IDs")
	}

	// Check FTS index
	var ftsCount int64
	err = dm.DB.QueryRow("SELECT COUNT(*) FROM tracks_fts").Scan(&ftsCount)
	if err != nil {
		return fmt.Errorf("failed to check FTS index: %w", err)
	}

	if ftsCount != stats.TrackCount {
		fmt.Printf("  ⚠️  FTS index has %d entries but there are %d tracks\n", ftsCount, stats.TrackCount)
	} else {
		fmt.Printf("  ✓ FTS index is in sync (%d entries)\n", ftsCount)
	}

	// Test search functionality
	fmt.Println("\nTesting search functionality...")
	searchTerms := []string{"love", "blue", "jazz", "rock", "classical"}

	for _, term := range searchTerms {
		tracks, err := dm.SearchTracksWithFTS(term, &database.SearchFilters{Limit: 5})
		if err != nil {
			fmt.Printf("  ✗ Search for '%s' failed: %v\n", term, err)
		} else {
			fmt.Printf("  ✓ Search for '%s' returned %d results\n", term, len(tracks))
			if *verbose && len(tracks) > 0 {
				fmt.Printf("    Sample: %s by %s\n", tracks[0].Name, tracks[0].Artist)
			}
		}
	}

	// Check playlist associations
	var orphanedAssociations int
	err = dm.DB.QueryRow(`
		SELECT COUNT(*) FROM playlist_tracks pt
		LEFT JOIN tracks t ON t.id = pt.track_id
		WHERE t.id IS NULL
	`).Scan(&orphanedAssociations)
	if err != nil {
		return fmt.Errorf("failed to check orphaned associations: %w", err)
	}

	if orphanedAssociations > 0 {
		fmt.Printf("\n  ⚠️  Found %d orphaned playlist associations\n", orphanedAssociations)
	} else {
		fmt.Println("\n  ✓ No orphaned playlist associations")
	}

	// Sample some tracks
	fmt.Println("\nSample tracks:")
	rows, err := dm.DB.Query(`
		SELECT t.name, ar.name, al.name, g.name, t.rating
		FROM tracks t
		LEFT JOIN artists ar ON ar.id = t.artist_id
		LEFT JOIN albums al ON al.id = t.album_id
		LEFT JOIN genres g ON g.id = t.genre_id
		ORDER BY RANDOM()
		LIMIT 5
	`)
	if err != nil {
		return fmt.Errorf("failed to sample tracks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var track, artist, album, genre string
		var rating int
		if err := rows.Scan(&track, &artist, &album, &genre, &rating); err != nil {
			continue
		}
		fmt.Printf("  - %s by %s (Album: %s, Genre: %s, Rating: %d)\n",
			track, artist, album, genre, rating)
	}

	fmt.Println("\nValidation completed!")
	return nil
}
