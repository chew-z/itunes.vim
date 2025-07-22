# Step 6: MCP Server Integration with Database Backend

Update the MCP server to use the SQLite database while maintaining full API compatibility:

## Tasks

1. **Update `mcp-server/main.go` with database integration**:
   - Initialize DatabaseManager alongside existing CacheManager
   - Add configuration to choose between database and cache modes
   - Implement graceful fallback to cache if database unavailable
   - Add database statistics to MCP resources

2. **Enhance search tool in MCP server**:
   - Update searchHandler to use database search when available
   - Add support for enhanced search filters (genre, rating, starred)
   - Maintain backward compatibility with existing MCP tool interface
   - Add performance metrics to search responses

3. **Enhance play tool with persistent ID support**:
   - Update playHandler to accept persistent IDs directly
   - Add playlist lookup by persistent ID for reliable context
   - Improve error messages for better debugging
   - Maintain compatibility with existing track/album/playlist name parameters

4. **Add new MCP tools for enhanced functionality**:
   - `list_playlists` tool to browse available playlists with metadata
   - `get_playlist_tracks` tool for playlist exploration
   - `search_advanced` tool with explicit filter parameters
   - Database status and statistics tools

5. **Update MCP resources with database information**:
   - Add `itunes://database/stats` resource for database metadata
   - Enhance existing cache resources to show database vs cache status
   - Add `itunes://database/playlists` resource for playlist browsing

6. **Create MCP integration tests**:
   - Test all tools with database backend
   - Verify backward compatibility with cache-based responses
   - Test graceful fallback when database unavailable
   - Performance tests to ensure MCP response times remain fast

## Requirements

- All existing MCP tools must work identically with database backend
- New tools must follow existing MCP naming and response conventions
- Fallback to cache must be seamless and logged appropriately
- Database mode should provide enhanced functionality without breaking changes
- Performance must remain within acceptable bounds for MCP usage

## Success Criteria

✅ MCP server works with both database and cache backends  
✅ All existing tools maintain backward compatibility  
✅ New tools provide enhanced functionality  
✅ Graceful fallback works correctly  
✅ Performance remains acceptable for MCP usage