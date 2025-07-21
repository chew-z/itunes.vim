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
        
        // Search through all playlists to build comprehensive library cache
        let playlists = music.playlists();
        
        if (verbose) {
            console.log("Found " + playlists.length + " playlists to process")
        }
        
        for (let playlist of playlists) {
            try {
                let playlistName = playlist.name();
                let tracks = playlist.tracks();
                
                if (verbose && playlists.indexOf(playlist) % 10 === 0) {
                    console.log("Processing playlist: " + playlistName + " (" + tracks.length + " tracks)")
                }
                
                for (let track of tracks) {
                    try {
                        let trackName = track.name.exists() ? track.name() : "";
                        let artistName = track.artist.exists() ? track.artist() : "";
                        let albumName = track.album.exists() ? track.album() : "";
                        
                        // Skip empty tracks
                        if (trackName === "" && artistName === "") {
                            continue;
                        }
                        
                        allTracks.push({
                            id: String(track.id()),
                            name: trackName,
                            album: albumName,
                            collection: playlistName, // Using actual playlist name as collection
                            artist: artistName,
                        });
                    } catch (trackError) {
                        // Skip tracks that can't be accessed
                        if (verbose) {
                            console.log("Error accessing track: " + trackError);
                        }
                    }
                }
            } catch (playlistError) {
                // Skip playlists that can't be accessed
                if (verbose) {
                    console.log("Error accessing playlist: " + playlistError);
                }
            }
        }

        if (verbose) {
            console.log("Library refresh complete. Total tracks: " + allTracks.length)
        }

        return JSON.stringify(allTracks);
    } catch (e) {
        console.log("Library refresh error: " + e)
        return JSON.stringify([])
    }
}