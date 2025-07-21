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
        
        // Get the main library playlist, which contains all unique tracks
        let libraryPlaylist = music.libraryPlaylists[0];
        let libraryTracks = libraryPlaylist.tracks();
        let trackCount = libraryTracks.length;
        
        if (verbose) {
            console.log("Found " + trackCount + " unique tracks in the library")
        }
        
        // Iterate through unique tracks once (much more efficient)
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
                
                allTracks.push({
                    id: track.persistentID(),
                    name: trackName,
                    album: albumName,
                    collection: albumName, // Using album as collection for consistent, non-duplicate results
                    artist: artistName,
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