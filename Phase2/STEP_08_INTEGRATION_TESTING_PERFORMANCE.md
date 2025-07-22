# Step 8: Integration Testing and Performance Validation

Create comprehensive integration tests and validate the complete system meets performance requirements:

## Tasks

1. **Create system integration tests in `integration_test.go`**:
   - End-to-end workflow: refresh → search → play with database backend
   - Cross-component testing: CLI, MCP server, and refresh service working together
   - Error scenario testing: database corruption, connection failures, etc.
   - Concurrent usage testing: multiple CLI/MCP operations simultaneously

2. **Create performance benchmark suite in `benchmark/`**:
   - Search performance comparison: JSON cache vs SQLite FTS5
   - Database refresh performance with various library sizes
   - MCP server response time validation under load
   - Memory usage profiling for long-running processes

3. **Create realistic test data generator in `testdata/`**:
   - Generate sample iTunes libraries with 10k, 50k, 100k+ tracks
   - Include diverse genres, artists, albums, and playlist relationships
   - Create edge cases: special characters, empty fields, large playlists
   - Mock Apple Music persistent IDs for consistent testing

4. **Add stress testing for database operations**:
   - Concurrent read/write scenarios
   - Large batch operations (inserting 50k+ tracks)
   - Database recovery after forced shutdowns
   - FTS5 index rebuild and optimization

5. **Create migration validation tools**:
   - Compare JSON cache vs database search results for consistency
   - Verify all tracks/playlists migrated correctly
   - Validate foreign key relationships and data integrity
   - Performance regression detection

6. **Add monitoring and observability**:
   - Database query performance logging
   - Search result quality metrics
   - Error rate monitoring across all components
   - Health check endpoints for system monitoring

## Requirements

- Search performance must meet 1-5ms target with realistic data
- Integration tests must pass with zero flakes
- System must handle 100k+ track libraries without performance degradation
- Memory usage must remain reasonable for long-running processes
- All error scenarios must be handled gracefully with helpful messages

## Success Criteria

✅ Integration tests pass consistently without flakes  
✅ Performance meets 1-5ms search target  
✅ System handles large libraries (100k+ tracks)  
✅ Stress testing passes all scenarios  
✅ Migration validation confirms data integrity