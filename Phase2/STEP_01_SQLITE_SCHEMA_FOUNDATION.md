# Step 1: SQLite Schema Foundation & Performance Validation

Create the SQLite database schema foundation with performance testing:

## Tasks

1. **Create a new file `itunes/database/schema.go`** that defines:
   - SQLite schema with all tables (artists, genres, albums, tracks, playlists, playlist_tracks)
   - FTS5 virtual table for search with optimized configuration
   - All necessary indexes and triggers
   - Schema migration system with version tracking

2. **Create `itunes/database/database.go`** with:
   - DatabaseManager struct using `modernc.org/sqlite` driver
   - NewDatabaseManager() function with proper SQLite PRAGMA settings
   - InitSchema() method to create tables and indexes
   - Basic CRUD operations for testing

3. **Create comprehensive tests in `itunes/database/database_test.go`**:
   - Schema creation and migration tests
   - Basic insert/select operations for all tables
   - Performance benchmarks with realistic data (insert 1000+ tracks, measure search time)
   - Validate FTS5 search returns results in <10ms

4. **Add SQLite dependency to go.mod** and update build commands in CLAUDE.md

5. **Create a simple CLI command** `go run database_test.go` to validate schema and run benchmarks

## Requirements

- All tests must pass
- Schema creation must be idempotent (safe to run multiple times)
- Performance benchmarks must complete successfully
- Zero external dependencies beyond SQLite driver

## Success Criteria

✅ Schema can be created and migrated successfully  
✅ Basic CRUD operations work correctly  
✅ FTS5 search performs within 10ms target  
✅ All tests pass without flakes  
✅ Performance benchmarks complete successfully