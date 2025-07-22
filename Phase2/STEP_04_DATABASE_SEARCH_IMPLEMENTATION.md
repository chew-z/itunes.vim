# Step 4: Database-Backed Search Implementation

Replace the current JSON-based search with SQLite FTS5 search while maintaining API compatibility:

## Tasks

1. **Add search methods to DatabaseManager in `database.go`**:
   - SearchTracksWithFTS() using FTS5 for text queries
   - GetTrackByPersistentID() for direct ID lookups
   - GetPlaylistTracks() with database queries
   - SearchFilters struct with genre, rating, starred options

2. **Create search optimization in `database/search.go`**:
   - Query builder for complex search filters
   - Result ranking and relevance scoring
   - Search result caching for repeated queries
   - Performance monitoring and query optimization

3. **Update `itunes/itunes.go` with database integration**:
   - Add database manager to package globals or context
   - Implement SearchTracksFromDatabase() as alternative to SearchTracksFromCache()
   - Add configuration flag to choose between JSON and database search
   - Maintain backward compatibility with existing search API

4. **Create comprehensive search tests in `database/search_test.go`**:
   - Test FTS5 queries with various search terms
   - Verify search filters work correctly (genre, rating, etc.)
   - Compare search results between JSON and database methods
   - Performance benchmarks to ensure <10ms search time

5. **Add database mode to CLI in `itunes.go`**:
   - Environment variable or command-line flag to enable database mode
   - Fallback to JSON cache if database is unavailable
   - Clear error messages for database connection issues

## Requirements

- Search API must remain identical to current JSON implementation
- FTS5 search must return results in relevance order
- Performance must meet or exceed current JSON search speed
- Graceful fallback to JSON cache if database unavailable
- All existing search functionality must work with database backend

## Success Criteria

✅ FTS5 search returns relevant results quickly  
✅ Search filters work correctly  
✅ API compatibility maintained  
✅ Performance meets <10ms target  
✅ Graceful fallback to JSON cache works