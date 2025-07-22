# Step 7: CLI Enhancement and Database Mode Integration

Update the CLI to support database mode with enhanced functionality while maintaining backward compatibility:

## Tasks

1. **Update main CLI in `itunes.go`**:
   - Add --database flag to enable SQLite backend
   - Initialize DatabaseManager when in database mode
   - Implement graceful fallback to cache mode with clear messaging
   - Add database status information to CLI output

2. **Enhance search command with database features**:
   - Add optional filter flags: --genre, --artist, --album, --starred, --min-rating
   - Support for playlist-specific search with --playlist flag
   - Enhanced output formatting with additional metadata (genre, rating)
   - Performance timing display for database vs cache comparison

3. **Enhance play command with persistent ID support**:
   - Accept persistent IDs directly as alternative to names
   - Add playlist lookup by persistent ID for improved reliability
   - Better error messages when tracks/playlists not found
   - Maintain backward compatibility with existing play syntax

4. **Add new CLI commands for database management**:
   - `itunes migrate` - convert JSON cache to SQLite database
   - `itunes refresh-db` - run database refresh service once
   - `itunes db-stats` - show database statistics and health
   - `itunes db-vacuum` - optimize database performance

5. **Update help text and usage examples**:
   - Document all new flags and commands
   - Provide examples of database mode usage
   - Clear guidance on when to use database vs cache mode
   - Troubleshooting section for database issues

6. **Create CLI integration tests**:
   - Test all commands in both cache and database modes
   - Verify flag parsing and error handling
   - Test database mode fallback scenarios
   - Performance comparison tests between modes

## Requirements

- Default behavior must remain unchanged (cache mode)
- Database mode must be opt-in via explicit flag
- All existing CLI functionality must work in database mode
- Error messages must clearly indicate which mode is active
- Help text must be comprehensive and accurate

## Success Criteria

✅ CLI supports both cache and database modes  
✅ New database commands work correctly  
✅ Enhanced search filters function properly  
✅ Backward compatibility maintained  
✅ Help text is comprehensive and clear