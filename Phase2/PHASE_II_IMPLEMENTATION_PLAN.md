# Phase II SQLite Migration Implementation Plan

Based on the Phase II Migration Plan, I'll break this down into manageable, testable steps that build incrementally toward the SQLite + persistent ID architecture.

## Step 1: SQLite Schema Foundation & Performance Validation

```text
Create the SQLite database schema foundation with performance testing:

1. Create a new file `itunes/database/schema.go` that defines:
   - SQLite schema with all tables (artists, genres, albums, tracks, playlists, playlist_tracks)
   - FTS5 virtual table for search with optimized configuration
   - All necessary indexes and triggers
   - Schema migration system with version tracking

2. Create `itunes/database/database.go` with:
   - DatabaseManager struct using `modernc.org/sqlite` driver
   - NewDatabaseManager() function with proper SQLite PRAGMA settings
   - InitSchema() method to create tables and indexes
   - Basic CRUD operations for testing

3. Create comprehensive tests in `itunes/database/database_test.go`:
   - Schema creation and migration tests
   - Basic insert/select operations for all tables
   - Performance benchmarks with realistic data (insert 1000+ tracks, measure search time)
   - Validate FTS5 search returns results in <10ms

4. Add SQLite dependency to go.mod and update build commands in CLAUDE.md

5. Create a simple CLI command `go run database_test.go` to validate schema and run benchmarks

Requirements:
- All tests must pass
- Schema creation must be idempotent (safe to run multiple times)
- Performance benchmarks must complete successfully
- Zero external dependencies beyond SQLite driver
```

## Step 2: Enhanced JXA Script for Persistent ID Extraction

```text
Enhance the existing iTunes_Refresh_Library.js script to extract persistent IDs for both tracks and playlists:

1. Update `itunes/scripts/iTunes_Refresh_Library.js`:
   - Extract track.persistentID() for all tracks (not just id())
   - Add playlist extraction with playlist.persistentID() and playlist.specialKind()
   - Include genre information from tracks and playlists
   - Return structured JSON with separate tracks and playlists arrays
   - Add error handling and progress reporting for large libraries

2. Update the response structure in `itunes/itunes.go`:
   - Define new structs: EnhancedTrack, PlaylistData, RefreshResponse
   - Modify RefreshLibraryCache() to parse the new JSON structure
   - Update Track struct to include persistent_id field (keeping backward compatibility)

3. Create test script `test_refresh_extraction.js`:
   - Mock Apple Music objects for testing
   - Verify persistent ID extraction logic
   - Test with empty library and large library scenarios

4. Update existing tests in `itunes/itunes_test.go`:
   - Mock the enhanced JXA response format
   - Verify parsing of persistent IDs and playlist data
   - Ensure backward compatibility with existing Track structure

5. Test the enhanced script manually:
   - Run with actual iTunes library
   - Verify persistent IDs are extracted correctly
   - Confirm playlist relationships are captured

Requirements:
- Script must handle libraries with 0 tracks gracefully
- Persistent IDs must be extracted as hex strings
- All existing functionality must continue working
- JSON response must be well-formed and parseable
```

## Step 3: Database Population and Migration Tools

```text
Build tools to populate the SQLite database from the enhanced refresh script:

1. Create `itunes/database/migrate.go`:
   - MigrateFromJSON() function to convert current cache format to SQLite
   - PopulateFromRefreshScript() to insert fresh JXA data into database
   - Atomic transaction handling for safe bulk operations
   - Duplicate detection and conflict resolution using persistent IDs

2. Enhance DatabaseManager in `database.go` with batch operations:
   - BatchInsertTracks() with proper foreign key handling
   - UpsertArtist(), UpsertAlbum(), UpsertGenre() helper functions
   - SyncPlaylist() to handle playlist-track relationships
   - UpdateFTSIndex() to manually maintain the search index

3. Create migration CLI tool `cmd/migrate/main.go`:
   - Read existing JSON cache from current location
   - Convert and populate SQLite database
   - Provide progress reporting and error handling
   - Validate migration success with row counts and spot checks

4. Add comprehensive tests in `database/migrate_test.go`:
   - Test migration from sample JSON data
   - Verify foreign key relationships are created correctly
   - Confirm FTS index is populated properly
   - Test rollback scenarios and error handling

5. Update build commands in CLAUDE.md:
   - Add `go build -o bin/itunes-migrate ./cmd/migrate`
   - Document migration procedure and validation steps

Requirements:
- Migration must be atomic (all-or-nothing)
- Foreign key relationships must be maintained correctly
- FTS index must be populated and searchable after migration
- Tool must handle missing or malformed JSON gracefully
- Progress reporting must be helpful for large libraries
```

## Step 4: Database-Backed Search Implementation

```text
Replace the current JSON-based search with SQLite FTS5 search while maintaining API compatibility:

1. Add search methods to DatabaseManager in `database.go`:
   - SearchTracksWithFTS() using FTS5 for text queries
   - GetTrackByPersistentID() for direct ID lookups
   - GetPlaylistTracks() with database queries
   - SearchFilters struct with genre, rating, starred options

2. Create search optimization in `database/search.go`:
   - Query builder for complex search filters
   - Result ranking and relevance scoring
   - Search result caching for repeated queries
   - Performance monitoring and query optimization

3. Update `itunes/itunes.go` with database integration:
   - Add database manager to package globals or context
   - Implement SearchTracksFromDatabase() as alternative to SearchTracksFromCache()
   - Add configuration flag to choose between JSON and database search
   - Maintain backward compatibility with existing search API

4. Create comprehensive search tests in `database/search_test.go`:
   - Test FTS5 queries with various search terms
   - Verify search filters work correctly (genre, rating, etc.)
   - Compare search results between JSON and database methods
   - Performance benchmarks to ensure <10ms search time

5. Add database mode to CLI in `itunes.go`:
   - Environment variable or command-line flag to enable database mode
   - Fallback to JSON cache if database is unavailable
   - Clear error messages for database connection issues

Requirements:
- Search API must remain identical to current JSON implementation
- FTS5 search must return results in relevance order
- Performance must meet or exceed current JSON search speed
- Graceful fallback to JSON cache if database unavailable
- All existing search functionality must work with database backend
```

## Step 5: Enhanced Refresh Service with Incremental Sync

```text
Build a standalone refresh service that efficiently syncs Apple Music changes to SQLite:

1. Create `refresh-service/main.go` as standalone binary:
   - Independent of CLI/MCP dependencies
   - Database connection management and configuration
   - Command-line options for full vs incremental refresh
   - Comprehensive logging and error reporting

2. Implement incremental sync logic in `refresh-service/sync.go`:
   - DetectChangedTracks() by comparing persistent IDs
   - CleanupOrphanedTracks() for removed tracks
   - SyncPlaylistMemberships() for playlist changes
   - Atomic transaction handling for consistency

3. Add change detection in `refresh-service/diff.go`:
   - Compare current database persistent IDs with fresh JXA data
   - Identify new tracks, removed tracks, and modified metadata
   - Build efficient update operations for minimal database impact
   - Track sync statistics and performance metrics

4. Create scheduling support in `refresh-service/scheduler.go`:
   - Support for cron-like scheduling
   - Lock file management to prevent concurrent refreshes
   - Health checks and service monitoring
   - Graceful shutdown handling

5. Add comprehensive tests in `refresh-service/`:
   - Test incremental sync with mock data changes
   - Verify change detection algorithms
   - Test concurrent access handling
   - Performance tests with large library changes

6. Update CLAUDE.md with refresh service documentation:
   - Build command: `go build -o bin/itunes-refresh ./refresh-service`
   - Usage examples and scheduling recommendations
   - Monitoring and troubleshooting guide

Requirements:
- Service must handle Apple Music app restarts gracefully
- Incremental sync must be significantly faster than full refresh
- Atomic transactions must ensure database consistency
- Service must be safe to run concurrently with CLI/MCP operations
- Clear logging must help troubleshoot sync issues
```

## Step 6: MCP Server Integration with Database Backend

```text
Update the MCP server to use the SQLite database while maintaining full API compatibility:

1. Update `mcp-server/main.go` with database integration:
   - Initialize DatabaseManager alongside existing CacheManager
   - Add configuration to choose between database and cache modes
   - Implement graceful fallback to cache if database unavailable
   - Add database statistics to MCP resources

2. Enhance search tool in MCP server:
   - Update searchHandler to use database search when available
   - Add support for enhanced search filters (genre, rating, starred)
   - Maintain backward compatibility with existing MCP tool interface
   - Add performance metrics to search responses

3. Enhance play tool with persistent ID support:
   - Update playHandler to accept persistent IDs directly
   - Add playlist lookup by persistent ID for reliable context
   - Improve error messages for better debugging
   - Maintain compatibility with existing track/album/playlist name parameters

4. Add new MCP tools for enhanced functionality:
   - `list_playlists` tool to browse available playlists with metadata
   - `get_playlist_tracks` tool for playlist exploration
   - `search_advanced` tool with explicit filter parameters
   - Database status and statistics tools

5. Update MCP resources with database information:
   - Add `itunes://database/stats` resource for database metadata
   - Enhance existing cache resources to show database vs cache status
   - Add `itunes://database/playlists` resource for playlist browsing

6. Create MCP integration tests:
   - Test all tools with database backend
   - Verify backward compatibility with cache-based responses
   - Test graceful fallback when database unavailable
   - Performance tests to ensure MCP response times remain fast

Requirements:
- All existing MCP tools must work identically with database backend
- New tools must follow existing MCP naming and response conventions
- Fallback to cache must be seamless and logged appropriately
- Database mode should provide enhanced functionality without breaking changes
- Performance must remain within acceptable bounds for MCP usage
```

## Step 7: CLI Enhancement and Database Mode Integration

```text
Update the CLI to support database mode with enhanced functionality while maintaining backward compatibility:

1. Update main CLI in `itunes.go`:
   - Add --database flag to enable SQLite backend
   - Initialize DatabaseManager when in database mode
   - Implement graceful fallback to cache mode with clear messaging
   - Add database status information to CLI output

2. Enhance search command with database features:
   - Add optional filter flags: --genre, --artist, --album, --starred, --min-rating
   - Support for playlist-specific search with --playlist flag
   - Enhanced output formatting with additional metadata (genre, rating)
   - Performance timing display for database vs cache comparison

3. Enhance play command with persistent ID support:
   - Accept persistent IDs directly as alternative to names
   - Add playlist lookup by persistent ID for improved reliability
   - Better error messages when tracks/playlists not found
   - Maintain backward compatibility with existing play syntax

4. Add new CLI commands for database management:
   - `itunes migrate` - convert JSON cache to SQLite database
   - `itunes refresh-db` - run database refresh service once
   - `itunes db-stats` - show database statistics and health
   - `itunes db-vacuum` - optimize database performance

5. Update help text and usage examples:
   - Document all new flags and commands
   - Provide examples of database mode usage
   - Clear guidance on when to use database vs cache mode
   - Troubleshooting section for database issues

6. Create CLI integration tests:
   - Test all commands in both cache and database modes
   - Verify flag parsing and error handling
   - Test database mode fallback scenarios
   - Performance comparison tests between modes

Requirements:
- Default behavior must remain unchanged (cache mode)
- Database mode must be opt-in via explicit flag
- All existing CLI functionality must work in database mode
- Error messages must clearly indicate which mode is active
- Help text must be comprehensive and accurate
```

## Step 8: Integration Testing and Performance Validation

```text
Create comprehensive integration tests and validate the complete system meets performance requirements:

1. Create system integration tests in `integration_test.go`:
   - End-to-end workflow: refresh → search → play with database backend
   - Cross-component testing: CLI, MCP server, and refresh service working together
   - Error scenario testing: database corruption, connection failures, etc.
   - Concurrent usage testing: multiple CLI/MCP operations simultaneously

2. Create performance benchmark suite in `benchmark/`:
   - Search performance comparison: JSON cache vs SQLite FTS5
   - Database refresh performance with various library sizes
   - MCP server response time validation under load
   - Memory usage profiling for long-running processes

3. Create realistic test data generator in `testdata/`:
   - Generate sample iTunes libraries with 10k, 50k, 100k+ tracks
   - Include diverse genres, artists, albums, and playlist relationships
   - Create edge cases: special characters, empty fields, large playlists
   - Mock Apple Music persistent IDs for consistent testing

4. Add stress testing for database operations:
   - Concurrent read/write scenarios
   - Large batch operations (inserting 50k+ tracks)
   - Database recovery after forced shutdowns
   - FTS5 index rebuild and optimization

5. Create migration validation tools:
   - Compare JSON cache vs database search results for consistency
   - Verify all tracks/playlists migrated correctly
   - Validate foreign key relationships and data integrity
   - Performance regression detection

6. Add monitoring and observability:
   - Database query performance logging
   - Search result quality metrics
   - Error rate monitoring across all components
   - Health check endpoints for system monitoring

Requirements:
- Search performance must meet 1-5ms target with realistic data
- Integration tests must pass with zero flakes
- System must handle 100k+ track libraries without performance degradation
- Memory usage must remain reasonable for long-running processes
- All error scenarios must be handled gracefully with helpful messages
```

## Step 9: Production Deployment and Migration Strategy

```text
Implement production deployment strategy with safe migration path and rollback capabilities:

1. Create deployment configuration in `deploy/`:
   - Environment-specific configuration files
   - Database backup and restore scripts
   - Migration runbooks with validation steps
   - Rollback procedures for each component

2. Add feature flag system in `config/`:
   - Runtime configuration for enabling database mode
   - Gradual rollout controls (percentage-based enabling)
   - A/B testing framework for performance comparison
   - Configuration validation and hot-reload support

3. Implement backup and recovery in `backup/`:
   - Automated SQLite database backup scheduling
   - Point-in-time recovery capabilities
   - Export/import tools for database portability
   - Validation tools for backup integrity

4. Add monitoring and alerting in `monitoring/`:
   - Database health check endpoints
   - Performance metric collection and reporting
   - Error rate alerting for critical failures
   - Usage analytics and performance trending

5. Create migration tools for production in `migration/`:
   - Safe migration orchestrator with validation steps
   - Parallel operation support (database + cache running simultaneously)
   - Migration progress monitoring and reporting
   - Automatic rollback triggers for performance degradation

6. Add comprehensive documentation:
   - Update CLAUDE.md with production deployment guide
   - Create troubleshooting runbook for common issues
   - Document performance tuning recommendations
   - Add operational procedures for database maintenance

Requirements:
- Migration must be reversible at any point
- System must support running in hybrid mode (cache + database)
- Performance monitoring must detect degradation automatically
- Rollback procedures must be tested and validated
- Documentation must be complete and accurate for production operations
```

## Step 10: Final Integration and Documentation

```text
Complete the integration, update all documentation, and ensure the system is production-ready:

1. Final integration cleanup in all components:
   - Remove any temporary or debug code
   - Ensure consistent error handling across all components
   - Validate all configuration options work correctly
   - Clean up unused imports and dependencies

2. Complete documentation update:
   - Update CLAUDE.md with complete feature documentation
   - Add performance benchmarks and comparison data
   - Document all new CLI commands and MCP tools
   - Create migration guide for existing users

3. Add final validation suite:
   - End-to-end system validation tests
   - Performance regression testing
   - Security vulnerability scanning
   - Code quality metrics and linting

4. Create release preparation:
   - Version tagging and changelog generation
   - Binary build automation for multiple platforms
   - Package distribution setup
   - Release notes with migration instructions

5. Add ongoing maintenance tools:
   - Database optimization and maintenance scripts
   - Performance monitoring dashboards
   - Automated testing pipeline for future changes
   - User feedback collection and issue tracking

6. Final testing with real user scenarios:
   - Test with various iTunes library sizes and configurations
   - Validate performance on different macOS versions
   - Test with complex playlist structures and edge cases
   - Confirm backward compatibility with existing workflows

Requirements:
- System must be completely production-ready
- All tests must pass consistently
- Documentation must be comprehensive and user-friendly
- Performance must meet or exceed original specifications
- Migration path must be clear and well-tested
- System must be maintainable for future development
```

---

This implementation plan breaks down the Phase II migration into 10 manageable steps, each building on the previous work. The steps prioritize testing, performance validation, and backward compatibility while incrementally adding the SQLite database functionality with Apple Music persistent ID support.