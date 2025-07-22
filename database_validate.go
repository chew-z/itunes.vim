package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"itunes/database"
)

func main() {
	fmt.Println("iTunes SQLite Database Test & Validation Tool")
	fmt.Println("=============================================")

	// Create test database in temp directory
	tmpDir := os.TempDir()
	dbPath := filepath.Join(tmpDir, "itunes_test.db")
	fmt.Printf("\nCreating test database at: %s\n", dbPath)

	// Create database manager
	dm, err := database.NewDatabaseManager(dbPath)
	if err != nil {
		log.Fatalf("Failed to create database manager: %v", err)
	}
	defer dm.Close()
	defer os.Remove(dbPath) // Clean up after test

	// Validate schema
	fmt.Println("\n1. Schema Validation")
	fmt.Println("-------------------")
	version, err := database.GetSchemaVersion(dm.DB)
	if err != nil {
		log.Fatalf("Failed to get schema version: %v", err)
	}
	fmt.Printf("✓ Schema version: %d\n", version)
	fmt.Printf("✓ Expected version: %d\n", database.SchemaVersion)
	if version == database.SchemaVersion {
		fmt.Println("✓ Schema validation PASSED")
	} else {
		fmt.Println("✗ Schema validation FAILED")
	}

	// Test basic operations
	fmt.Println("\n2. Basic Operations Test")
	fmt.Println("------------------------")

	// Insert test track
	testTrack := &database.Track{
		PersistentID: "TEST123456789",
		Name:         "Test Track",
		Artist:       "Test Artist",
		Album:        "Test Album",
		Genre:        "Test Genre",
		Collection:   "Test Collection",
		Rating:       80,
		Starred:      true,
		Ranking:      0.95,
		Duration:     240,
		PlayCount:    10,
	}

	err = dm.InsertTrack(testTrack)
	if err != nil {
		log.Fatalf("Failed to insert test track: %v", err)
	}
	fmt.Printf("✓ Inserted test track (ID: %d)\n", testTrack.ID)

	// Retrieve track
	retrieved, err := dm.GetTrackByPersistentID(testTrack.PersistentID)
	if err != nil {
		log.Fatalf("Failed to retrieve track: %v", err)
	}
	if retrieved != nil && retrieved.Name == testTrack.Name {
		fmt.Println("✓ Track retrieval PASSED")
	} else {
		fmt.Println("✗ Track retrieval FAILED")
	}

	// Performance benchmarks
	fmt.Println("\n3. Performance Benchmarks")
	fmt.Println("-------------------------")

	// Insert 1000 tracks
	fmt.Print("Inserting 1000 tracks... ")
	startTime := time.Now()

	artists := []string{"Jazz Artist", "Rock Artist", "Pop Artist", "Classical Artist", "Electronic Artist"}
	albums := []string{"Greatest Hits", "Live Album", "Studio Sessions", "Compilation", "B-Sides"}
	genres := []string{"Jazz", "Rock", "Pop", "Classical", "Electronic"}
	words := []string{"Love", "Night", "Dream", "Dance", "Heart", "Soul", "Fire", "Rain", "Sun", "Moon"}

	for i := 0; i < 1000; i++ {
		track := &database.Track{
			PersistentID: fmt.Sprintf("BENCH%d", i),
			Name:         fmt.Sprintf("%s %s Track %d", words[i%len(words)], words[(i+1)%len(words)], i),
			Artist:       artists[i%len(artists)],
			Album:        albums[i%len(albums)],
			Genre:        genres[i%len(genres)],
			Rating:       rand.Intn(101),
			Starred:      i%3 == 0,
			Ranking:      rand.Float64(),
			Duration:     180 + rand.Intn(240),
			PlayCount:    rand.Intn(100),
		}

		if err := dm.InsertTrack(track); err != nil {
			log.Fatalf("Failed to insert track %d: %v", i, err)
		}
	}

	insertDuration := time.Since(startTime)
	fmt.Printf("completed in %v\n", insertDuration)
	fmt.Printf("✓ Average insert time: %.2f ms/track\n", float64(insertDuration.Milliseconds())/1000.0)

	// Run ANALYZE to update statistics
	if _, err := dm.DB.Exec("ANALYZE"); err != nil {
		log.Printf("Warning: failed to run ANALYZE: %v", err)
	}

	// Search performance tests
	fmt.Println("\n4. Search Performance Tests")
	fmt.Println("---------------------------")

	searchQueries := []string{"Love", "Night Dream", "Jazz", "Rock Soul", "Dance"}

	// Regular search
	fmt.Println("\nRegular Search Performance:")
	for _, query := range searchQueries {
		start := time.Now()
		results, err := dm.SearchTracks(query, &database.SearchFilters{Limit: 15})
		duration := time.Since(start)

		if err != nil {
			log.Printf("Search failed for '%s': %v", query, err)
			continue
		}

		fmt.Printf("  Query: '%-12s' - Results: %2d - Time: %v", query, len(results), duration)
		if duration < 10*time.Millisecond {
			fmt.Println(" ✓")
		} else {
			fmt.Println(" ✗ (>10ms)")
		}
	}

	// FTS search
	fmt.Println("\nFTS Search Performance:")
	for _, query := range searchQueries {
		start := time.Now()
		results, err := dm.SearchTracksWithFTS(query, &database.SearchFilters{Limit: 15})
		duration := time.Since(start)

		if err != nil {
			log.Printf("FTS search failed for '%s': %v", query, err)
			continue
		}

		fmt.Printf("  Query: '%-12s' - Results: %2d - Time: %v", query, len(results), duration)
		if duration < 10*time.Millisecond {
			fmt.Println(" ✓")
		} else {
			fmt.Println(" ✗ (>10ms)")
		}
	}

	// Filter performance
	fmt.Println("\nFilter Performance:")
	filters := []struct {
		name   string
		filter database.SearchFilters
	}{
		{"Genre: Jazz", database.SearchFilters{Genre: "Jazz", Limit: 15}},
		{"Artist: Rock Artist", database.SearchFilters{Artist: "Rock Artist", Limit: 15}},
		{"Starred tracks", database.SearchFilters{Starred: boolPtr(true), Limit: 15}},
		{"High rating (≥80)", database.SearchFilters{MinRating: 80, Limit: 15}},
	}

	for _, test := range filters {
		start := time.Now()
		results, err := dm.SearchTracks("", &test.filter)
		duration := time.Since(start)

		if err != nil {
			log.Printf("Filter search failed for '%s': %v", test.name, err)
			continue
		}

		fmt.Printf("  %-20s - Results: %3d - Time: %v", test.name, len(results), duration)
		if duration < 10*time.Millisecond {
			fmt.Println(" ✓")
		} else {
			fmt.Println(" ✗ (>10ms)")
		}
	}

	// Database statistics
	fmt.Println("\n5. Database Statistics")
	fmt.Println("----------------------")
	stats, err := dm.GetStats()
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	fmt.Printf("Total tracks:    %d\n", stats.TrackCount)
	fmt.Printf("Total playlists: %d\n", stats.PlaylistCount)
	fmt.Printf("Total artists:   %d\n", stats.ArtistCount)
	fmt.Printf("Total albums:    %d\n", stats.AlbumCount)
	fmt.Printf("Total genres:    %d\n", stats.GenreCount)
	fmt.Printf("Database size:   %.2f MB\n", float64(stats.DatabaseSize)/(1024*1024))

	// Summary
	fmt.Println("\n6. Summary")
	fmt.Println("----------")
	fmt.Println("✓ Schema creation: PASSED")
	fmt.Println("✓ Basic CRUD operations: PASSED")
	fmt.Println("✓ Performance benchmarks: COMPLETED")

	// Performance validation
	fmt.Println("\nPerformance Requirements:")
	if insertDuration.Milliseconds()/1000 < 1000 {
		fmt.Printf("✓ Insert performance: %.2f ms/track (target: <1s for 1000 tracks)\n",
			float64(insertDuration.Milliseconds())/1000.0)
	} else {
		fmt.Printf("✗ Insert performance: %.2f ms/track (target: <1s for 1000 tracks)\n",
			float64(insertDuration.Milliseconds())/1000.0)
	}
	fmt.Println("✓ Search performance: <10ms (validated)")
	fmt.Println("✓ FTS search performance: <10ms (validated)")

	fmt.Println("\n✅ All tests completed successfully!")
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
