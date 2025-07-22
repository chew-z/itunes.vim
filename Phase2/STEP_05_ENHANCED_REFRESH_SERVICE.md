# Step 5: Enhanced Refresh Service with Incremental Sync

Build a standalone refresh service that efficiently syncs Apple Music changes to SQLite:

## Tasks

1. **Create `refresh-service/main.go` as standalone binary**:
   - Independent of CLI/MCP dependencies
   - Database connection management and configuration
   - Command-line options for full vs incremental refresh
   - Comprehensive logging and error reporting

2. **Implement incremental sync logic in `refresh-service/sync.go`**:
   - DetectChangedTracks() by comparing persistent IDs
   - CleanupOrphanedTracks() for removed tracks
   - SyncPlaylistMemberships() for playlist changes
   - Atomic transaction handling for consistency

3. **Add change detection in `refresh-service/diff.go`**:
   - Compare current database persistent IDs with fresh JXA data
   - Identify new tracks, removed tracks, and modified metadata
   - Build efficient update operations for minimal database impact
   - Track sync statistics and performance metrics

4. **Create scheduling support in `refresh-service/scheduler.go`**:
   - Support for cron-like scheduling
   - Lock file management to prevent concurrent refreshes
   - Health checks and service monitoring
   - Graceful shutdown handling

5. **Add comprehensive tests in `refresh-service/`**:
   - Test incremental sync with mock data changes
   - Verify change detection algorithms
   - Test concurrent access handling
   - Performance tests with large library changes

6. **Update CLAUDE.md with refresh service documentation**:
   - Build command: `go build -o bin/itunes-refresh ./refresh-service`
   - Usage examples and scheduling recommendations
   - Monitoring and troubleshooting guide

## Requirements

- Service must handle Apple Music app restarts gracefully
- Incremental sync must be significantly faster than full refresh
- Atomic transactions must ensure database consistency
- Service must be safe to run concurrently with CLI/MCP operations
- Clear logging must help troubleshoot sync issues

## Success Criteria

✅ Standalone refresh service builds and runs independently  
✅ Incremental sync detects changes efficiently  
✅ Change detection algorithms work correctly  
✅ Concurrent access is handled safely  
✅ Performance is significantly improved over full refresh