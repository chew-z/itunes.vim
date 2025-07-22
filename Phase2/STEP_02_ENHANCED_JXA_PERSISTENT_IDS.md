# Step 2: Enhanced JXA Script for Persistent ID Extraction

Enhance the existing iTunes_Refresh_Library.js script to extract persistent IDs for both tracks and playlists:

## Tasks

1. **Update `itunes/scripts/iTunes_Refresh_Library.js`**:
   - Extract track.persistentID() for all tracks (not just id())
   - Add playlist extraction with playlist.persistentID() and playlist.specialKind()
   - Include genre information from tracks and playlists
   - Return structured JSON with separate tracks and playlists arrays
   - Add error handling and progress reporting for large libraries

2. **Update the response structure in `itunes/itunes.go`**:
   - Define new structs: EnhancedTrack, PlaylistData, RefreshResponse
   - Modify RefreshLibraryCache() to parse the new JSON structure
   - Update Track struct to include persistent_id field (keeping backward compatibility)

3. **Create test script `test_refresh_extraction.js`**:
   - Mock Apple Music objects for testing
   - Verify persistent ID extraction logic
   - Test with empty library and large library scenarios

4. **Update existing tests in `itunes/itunes_test.go`**:
   - Mock the enhanced JXA response format
   - Verify parsing of persistent IDs and playlist data
   - Ensure backward compatibility with existing Track structure

5. **Test the enhanced script manually**:
   - Run with actual iTunes library
   - Verify persistent IDs are extracted correctly
   - Confirm playlist relationships are captured

## Requirements

- Script must handle libraries with 0 tracks gracefully
- Persistent IDs must be extracted as hex strings
- All existing functionality must continue working
- JSON response must be well-formed and parseable

## Success Criteria

✅ Enhanced JXA script extracts persistent IDs correctly  
✅ Playlist relationships are captured properly  
✅ Backward compatibility maintained  
✅ All tests pass with new structure  
✅ Manual testing confirms correct extraction