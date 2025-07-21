#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// osacompile -l JavaScript -o iTunes_Refresh_Library.scpt iTunes_Refresh_Library.js
ObjC.import('stdlib')

function run(argv) {
    'use strict'
    let music = Application('Music')
    const verbose = false

    if (verbose) {
        console.log("Starting iTunes library refresh...")
    }

    try {
        let allTracks = [];
        let playlistMap = new Map(); // trackID -> [playlist names]
        let playlists = music.playlists();
        
        if (verbose) {
            console.log("Phase 1: Building playlist membership map...")
        }
        
        // Phase 1: Build playlist membership map from user-created playlists
        for (let i = 0; i < playlists.length; i++) {
            let playlist = playlists[i];
            try {
                // Process only user-created playlists
                if (playlist.specialKind() === "none") {
                    let playlistName = playlist.name();
                    let playlistTracks = playlist.tracks();
                    for (let j = 0; j < playlistTracks.length; j++) {
                        let track = playlistTracks[j];
                        if (track.persistentID.exists()) {
                            let trackID = track.persistentID();
                            if (!playlistMap.has(trackID)) {
                                playlistMap.set(trackID, []);
                            }
                            playlistMap.get(trackID).push(playlistName);
                        }
                    }
                }
            } catch (playlistError) {
                if (verbose) {
                    console.log("Could not process playlist at index " + i + ": " + playlistError);
                }
            }
        }
        
        if (verbose) {
            console.log("Phase 1 complete. Found " + playlistMap.size + " tracks in user playlists.")
            console.log("Phase 2: Building enhanced track data from main library...")
        }

        // Phase 2: Build enhanced track data from the main library
        let libraryPlaylist = music.libraryPlaylists[0];
        let libraryTracks = libraryPlaylist.tracks();
        let trackCount = libraryTracks.length;
        
        if (verbose) {
            console.log("Found " + trackCount + " unique tracks in the library")
        }
        
        // Iterate through unique tracks once
        for (let i = 0; i < trackCount; i++) {
            let track = libraryTracks[i];
            try {
                let trackName = track.name.exists() ? track.name() : "";
                let artistName = track.artist.exists() ? track.artist() : "";
                let albumName = track.album.exists() ? track.album() : "";
                
                // Skip empty tracks
                if (trackName === "" && artistName === "") {
                    continue;
                }
                
                let trackID = track.persistentID();
                let trackPlaylists = playlistMap.get(trackID) || [];
                
                allTracks.push({
                    id: trackID,
                    name: trackName,
                    album: albumName,
                    collection: trackPlaylists.length > 0 ? trackPlaylists[0] : albumName,
                    artist: artistName,
                    playlists: trackPlaylists
                });
                
                // Progress indicator for large libraries
                if (verbose && i > 0 && i % 1000 === 0) {
                    console.log("Processed " + i + " of " + trackCount + " tracks...")
                }
            } catch (trackError) {
                // Skip tracks that can't be accessed
                if (verbose) {
                    console.log("Error accessing track at index " + i + ": " + trackError);
                }
            }
        }

        if (verbose) {
            console.log("Library refresh complete. Total unique tracks: " + allTracks.length)
        }

        return JSON.stringify({ status: "success", data: allTracks });
    } catch (e) {
        return JSON.stringify({ status: "error", message: "Library refresh error: " + e.message, error: e.name })
    }
}