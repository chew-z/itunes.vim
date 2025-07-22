# Step 3: Database Population and Migration Tools

Build tools to populate the SQLite database from the enhanced refresh script:

## Tasks

1. **Create `itunes/database/migrate.go`**:
   - MigrateFromJSON() function to convert current cache format to SQLite
   - PopulateFromRefreshScript() to insert fresh JXA data into database
   - Atomic transaction handling for safe bulk operations
   - Duplicate detection and conflict resolution using persistent IDs

2. **Enhance DatabaseManager in `database.go` with batch operations**:
   - BatchInsertTracks() with proper foreign key handling
   - UpsertArtist(), UpsertAlbum(), UpsertGenre() helper functions
   - SyncPlaylist() to handle playlist-track relationships
   - UpdateFTSIndex() to manually maintain the search index

3. **Create migration CLI tool `cmd/migrate/main.go`**:
   - Read existing JSON cache from current location
   - Convert and populate SQLite database
   - Provide progress reporting and error handling
   - Validate migration success with row counts and spot checks

4. **Add comprehensive tests in `database/migrate_test.go`**:
   - Test migration from sample JSON data
   - Verify foreign key relationships are created correctly
   - Confirm FTS index is populated properly
   - Test rollback scenarios and error handling

5. **Update build commands in CLAUDE.md**:
   - Add `go build -o bin/itunes-migrate ./cmd/migrate`
   - Document migration procedure and validation steps

## Requirements

- Migration must be atomic (all-or-nothing)
- Foreign key relationships must be maintained correctly
- FTS index must be populated and searchable after migration
- Tool must handle missing or malformed JSON gracefully
- Progress reporting must be helpful for large libraries

## Success Criteria

✅ Migration tool converts JSON to SQLite successfully  
✅ Foreign key relationships are maintained  
✅ FTS index is populated and searchable  
✅ Atomic transactions work correctly  
✅ Progress reporting is helpful and accurate