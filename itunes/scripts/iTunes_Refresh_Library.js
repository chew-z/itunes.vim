#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// Enhanced iTunes Library Refresh Script with Persistent ID Extraction
// Extracts both tracks and playlists with their persistent IDs and metadata.
//
// Uses bulk property fetches (e.g. tracks.name() returning the full array in a
// single Apple Event) instead of per-track property reads. On an ~11k-track
// library this is the difference between ~0.5s and >10 minutes of extraction.
ObjC.import('stdlib')

function run(argv) {
    'use strict'
    let music = Application('Music')

    try {
        let allPlaylists = []
        let playlistMap = new Map() // trackID -> [playlist names]
        let playlists = music.playlists()

        // Phase 1: Extract playlist data and build membership map.
        // One bulk persistentID fetch per playlist instead of one per track.
        for (let i = 0; i < playlists.length; i++) {
            let playlist = playlists[i]
            try {
                let playlistName = playlist.name()
                let playlistData = {
                    id: playlist.persistentID(),
                    name: playlistName,
                    special_kind: playlist.specialKind(),
                    track_count: 0,
                    genre: '',
                }

                try {
                    if (playlist.genre.exists()) {
                        playlistData.genre = playlist.genre()
                    }
                } catch (genreError) {
                    // Some playlists don't have genres
                }

                if (playlistData.special_kind === 'none') {
                    let memberIDs = playlist.tracks.persistentID()
                    playlistData.track_count = memberIDs.length
                    for (let j = 0; j < memberIDs.length; j++) {
                        let trackID = memberIDs[j]
                        if (!trackID) continue
                        if (!playlistMap.has(trackID)) {
                            playlistMap.set(trackID, [])
                        }
                        playlistMap.get(trackID).push(playlistName)
                    }
                } else {
                    try {
                        playlistData.track_count = playlist.tracks.persistentID().length
                    } catch (e) {
                        playlistData.track_count = 0
                    }
                }

                allPlaylists.push(playlistData)
            } catch (playlistError) {
                // Skip playlists that can't be accessed
            }
        }

        // Phase 2: Bulk-fetch all track properties from the main library.
        // Each call is a single Apple Event returning a parallel array.
        let libraryPlaylist = music.libraryPlaylists[0]
        let ids = libraryPlaylist.tracks.persistentID()
        let names = libraryPlaylist.tracks.name()
        let artists = libraryPlaylist.tracks.artist()
        let albums = libraryPlaylist.tracks.album()
        let kinds = libraryPlaylist.tracks.kind()
        let genres = libraryPlaylist.tracks.genre()
        let ratings = libraryPlaylist.tracks.rating()

        // Stream URLs exist only on URL tracks; fetch them separately and map by ID.
        let streamURLMap = new Map()
        try {
            let urlIDs = libraryPlaylist.urlTracks.persistentID()
            let urlAddresses = libraryPlaylist.urlTracks.address()
            for (let i = 0; i < urlIDs.length; i++) {
                if (urlIDs[i] && urlAddresses[i]) {
                    streamURLMap.set(urlIDs[i], urlAddresses[i])
                }
            }
        } catch (e) {
            // No URL tracks in the library
        }

        let allTracks = []
        let skippedCount = 0

        for (let i = 0; i < ids.length; i++) {
            let trackName = names[i] || ''
            let artistName = artists[i] || ''

            // Skip empty tracks
            if (trackName === '' && artistName === '') {
                skippedCount++
                continue
            }

            let trackID = ids[i]
            let trackPlaylists = playlistMap.get(trackID) || []
            let albumName = albums[i] || ''
            let trackKind = kinds[i] || ''
            let isStreaming = trackKind === 'Internet audio stream'
            let rating = ratings[i] || 0

            allTracks.push({
                id: trackID,
                persistent_id: trackID, // Explicit field for clarity
                name: trackName,
                album: albumName,
                collection: trackPlaylists.length > 0 ? trackPlaylists[0] : albumName,
                artist: artistName,
                playlists: trackPlaylists,
                genre: genres[i] || '',
                rating: rating,
                // Apple Music uses 100 for "loved" tracks
                starred: rating === 100,
                is_streaming: isStreaming,
                kind: trackKind,
                stream_url: isStreaming ? streamURLMap.get(trackID) || '' : '',
            })
        }

        // Return structured response with both tracks and playlists
        return JSON.stringify({
            status: 'success',
            data: {
                tracks: allTracks,
                playlists: allPlaylists,
                stats: {
                    track_count: allTracks.length,
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
