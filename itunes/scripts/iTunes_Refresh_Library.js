#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// Enhanced iTunes Library Refresh Script with Persistent ID Extraction
// Extracts both tracks and playlists with their persistent IDs and metadata
ObjC.import('stdlib')

function run(argv) {
    'use strict'
    let music = Application('Music')
    const verbose = false
    const progressInterval = 1000 // Report progress every N tracks

    if (verbose) {
        console.log('Starting enhanced iTunes library refresh...')
    }

    try {
        let allTracks = []
        let allPlaylists = []
        let playlistMap = new Map() // trackID -> [playlist names]
        let playlistPersistentIDMap = new Map() // playlist name -> persistent ID
        let playlists = music.playlists()

        if (verbose) {
            console.log('Phase 1: Extracting playlist data and building membership map...')
        }

        // Phase 1: Extract playlist data and build membership map
        for (let i = 0; i < playlists.length; i++) {
            let playlist = playlists[i]
            try {
                let playlistName = playlist.name()
                let playlistPersistentID = playlist.persistentID()
                let specialKind = playlist.specialKind()

                // Extract playlist metadata
                let playlistData = {
                    id: playlistPersistentID,
                    name: playlistName,
                    special_kind: specialKind,
                    track_count: 0,
                    genre: '',
                }

                // Try to get playlist genre if it exists
                try {
                    if (playlist.genre.exists()) {
                        playlistData.genre = playlist.genre()
                    }
                } catch (genreError) {
                    // Some playlists don't have genres
                }

                // Process tracks for user-created playlists
                if (specialKind === 'none') {
                    playlistPersistentIDMap.set(playlistName, playlistPersistentID)
                    let playlistTracks = playlist.tracks()
                    playlistData.track_count = playlistTracks.length

                    for (let j = 0; j < playlistTracks.length; j++) {
                        let track = playlistTracks[j]
                        if (track.persistentID.exists()) {
                            let trackID = track.persistentID()
                            if (!playlistMap.has(trackID)) {
                                playlistMap.set(trackID, [])
                            }
                            playlistMap.get(trackID).push(playlistName)
                        }
                    }
                } else {
                    // For special playlists, just get the count
                    try {
                        playlistData.track_count = playlist.tracks().length
                    } catch (e) {
                        playlistData.track_count = 0
                    }
                }

                allPlaylists.push(playlistData)
            } catch (playlistError) {
                if (verbose) {
                    console.log('Could not process playlist at index ' + i + ': ' + playlistError)
                }
            }
        }

        if (verbose) {
            console.log('Phase 1 complete. Found ' + allPlaylists.length + ' playlists.')
            console.log('Phase 2: Building enhanced track data from main library...')
        }

        // Phase 2: Build enhanced track data from the main library
        let libraryPlaylist = music.libraryPlaylists[0]
        let libraryTracks = libraryPlaylist.tracks()
        let trackCount = libraryTracks.length
        let processedCount = 0
        let skippedCount = 0

        if (verbose) {
            console.log('Found ' + trackCount + ' tracks in the library')
        }

        // Iterate through all tracks
        for (let i = 0; i < trackCount; i++) {
            let track = libraryTracks[i]
            try {
                // Extract basic track information
                let trackName = track.name.exists() ? track.name() : ''
                let artistName = track.artist.exists() ? track.artist() : ''
                let albumName = track.album.exists() ? track.album() : ''

                // Skip empty tracks
                if (trackName === '' && artistName === '') {
                    skippedCount++
                    continue
                }

                let trackPersistentID = track.persistentID()
                let trackPlaylists = playlistMap.get(trackPersistentID) || []

                // Build enhanced track object
                let trackData = {
                    id: trackPersistentID,
                    persistent_id: trackPersistentID, // Explicit field for clarity
                    name: trackName,
                    album: albumName,
                    collection: trackPlaylists.length > 0 ? trackPlaylists[0] : albumName,
                    artist: artistName,
                    playlists: trackPlaylists,
                    genre: '',
                    rating: 0,
                    starred: false,
                }

                // Extract additional metadata
                try {
                    if (track.genre.exists()) {
                        trackData.genre = track.genre()
                    }
                } catch (e) {}

                try {
                    if (track.rating.exists()) {
                        trackData.rating = track.rating()
                        // Apple Music uses 100 for "loved" tracks
                        trackData.starred = track.rating() === 100
                    }
                } catch (e) {}

                allTracks.push(trackData)
                processedCount++

                // Progress indicator for large libraries
                if (verbose && i > 0 && i % progressInterval === 0) {
                    console.log('Processed ' + i + ' of ' + trackCount + ' tracks...')
                }
            } catch (trackError) {
                // Skip tracks that can't be accessed
                skippedCount++
                if (verbose) {
                    console.log('Error accessing track at index ' + i + ': ' + trackError)
                }
            }
        }

        if (verbose) {
            console.log('Library refresh complete.')
            console.log('Total tracks processed: ' + processedCount)
            console.log('Tracks skipped: ' + skippedCount)
            console.log('Playlists found: ' + allPlaylists.length)
        }

        // Return structured response with both tracks and playlists
        return JSON.stringify({
            status: 'success',
            data: {
                tracks: allTracks,
                playlists: allPlaylists,
                stats: {
                    track_count: processedCount,
                    playlist_count: allPlaylists.length,
                    skipped_tracks: skippedCount,
                    refresh_time: new Date().toISOString(),
                },
            },
        })
    } catch (e) {
        return JSON.stringify({
            status: 'error',
            message: 'Library refresh error: ' + e.message,
            error: e.name,
            data: {
                tracks: [],
                playlists: [],
                stats: {
                    track_count: 0,
                    playlist_count: 0,
                    skipped_tracks: 0,
                    refresh_time: new Date().toISOString(),
                },
            },
        })
    }
}
