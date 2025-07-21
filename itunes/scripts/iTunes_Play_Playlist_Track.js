#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// osacompile -l JavaScript -o iTunes_Play_Tracks.scpt iTunes_Play_Tracks.js
ObjC.import('stdlib')

function run(argv) {
    'use strict'
    let music = Application('Music')
    const verbose = false

    var args = $.NSProcessInfo.processInfo.arguments // NSArray
    var argv = []
    var argc = args.count
    for (let i = 4; i < argc; i++) {
        // skip 3-word run command at top and this file's name
        if (verbose) {
            console.log($(args.objectAtIndex(i)).js)
        } // print each argument
        argv.push(ObjC.unwrap(args.objectAtIndex(i))) // collect arguments
    }
    if (verbose) {
        console.log(argv)
    } // print arguments
    
    try {
        // Updated argument parsing to support ID-based playback
        // argv[0] = playlist name (optional)
        // argv[1] = track name (optional)  
        // argv[2] = track ID (optional, takes priority over name)
        
        let playlistName = argv.length > 0 ? argv[0] : "";
        let trackName = argv.length > 1 ? argv[1] : "";
        let trackId = argv.length > 2 ? argv[2] : "";
        
        if (verbose) {
            console.log("Playlist: " + playlistName + ", Track: " + trackName + ", ID: " + trackId);
        }

        // If no arguments provided at all, that's an error
        if (playlistName === "" && trackName === "" && trackId === "") {
            return "ERROR: No playlist, track, or track ID specified. Usage: play [playlist] [track] [trackId]"
        }

        // Find the playlist by name (if playlist name provided)
        let playlist = null;
        if (playlistName !== "") {
            let playlists = music.playlists();
            
            for (let p of playlists) {
                if (p.name() === playlistName) {
                    playlist = p;
                    break;
                }
            }
        }
        
        // PRIORITY 1: Try ID-based lookup if track ID provided (most reliable)
        if (trackId !== "") {
            if (verbose) {
                console.log("Attempting ID-based track lookup: " + trackId);
            }
            
            try {
                // Direct ID-based lookup using Apple Music's persistent ID system
                let tracksByID = music.tracks.whose({persistentID: trackId});
                if (tracksByID.length > 0) {
                    let foundTrack = tracksByID[0];
                    if (verbose) {
                        console.log("Found track by ID: " + foundTrack.name());
                    }
                    foundTrack.play();
                    return "OK: Started playing track by ID: " + foundTrack.name();
                }
                
                if (verbose) {
                    console.log("No track found with ID: " + trackId);
                }
                
                // If ID lookup failed, continue to name-based fallback
            } catch (e) {
                if (verbose) {
                    console.log("ID lookup failed: " + e.message);
                }
                // Continue to fallback methods
            }
        }

        // FALLBACK: If no playlist found (either not provided or doesn't exist), try name-based track lookup
        if (!playlist) {
            if (trackName === "" && trackId === "") {
                // If we have a playlist name but no playlist found, and no track info
                if (playlistName !== "") {
                    return "ERROR: Playlist not found: " + playlistName;
                } else {
                    return "ERROR: No playlist, track name, or track ID specified";
                }
            }
            
            // Skip name-based search if we already tried ID lookup
            if (trackName !== "" && trackId === "") {
                if (verbose) {
                    console.log("No playlist found, searching for track by name: " + trackName);
                }
                
                let foundTrack = null;
                
                // Search main library playlist first (much faster than iterating all playlists)
                let libraryPlaylist = music.libraryPlaylists[0];
                let libraryTracks = libraryPlaylist.tracks();
                
                for (let i = 0; i < libraryTracks.length; i++) {
                    let track = libraryTracks[i];
                    if (track.name.exists() && track.name() === trackName) {
                        foundTrack = track;
                        break;
                    }
                }
                
                if (foundTrack) {
                    if (verbose) {
                        console.log("Found track by name: " + foundTrack.name());
                    }
                    foundTrack.play();
                    return "OK: Started playing track by name: " + trackName;
                } else {
                    return "ERROR: Track not found in library by name: " + trackName;
                }
            } else if (trackId !== "") {
                // ID lookup already failed above
                return "ERROR: Track not found by ID: " + trackId;
            }
        }
        
        // PLAYLIST CONTEXT PLAYBACK: If we have a playlist and either track name or ID
        if (playlist) {
            // If no specific track requested, play the entire playlist
            if (trackName === "" && trackId === "") {
                if (verbose) {
                    console.log("Playing entire playlist: " + playlistName);
                }
                try {
                    playlist.play();
                    return "OK: Started playing playlist: " + playlistName;
                } catch (e) {
                    return "ERROR: Failed to play playlist '" + playlistName + "': " + e.message;
                }
            }
            
            // PRIORITY 1: Try ID-based lookup within playlist
            if (trackId !== "") {
                try {
                    let playlistTracks = playlist.tracks();
                    let foundTrack = null;
                    
                    for (let track of playlistTracks) {
                        if (track.persistentID() === trackId) {
                            foundTrack = track;
                            break;
                        }
                    }
                    
                    if (foundTrack) {
                        if (verbose) {
                            console.log("Found track by ID in playlist: " + foundTrack.name());
                        }
                        playlist.reveal();
                        playlist.play();
                        foundTrack.play();
                        return "OK: Started playing track by ID '" + foundTrack.name() + "' from playlist '" + playlistName + "'";
                    }
                } catch (e) {
                    if (verbose) {
                        console.log("Playlist ID lookup failed: " + e.message);
                    }
                    // Continue to name-based fallback
                }
            }
            
            // FALLBACK: Name-based lookup within playlist
            if (trackName !== "") {
                try {
                    let tracks = playlist.tracks();
                    let foundTrack = null;
                    
                    for (let track of tracks) {
                        if (track.name.exists() && track.name() === trackName) {
                            foundTrack = track;
                            break;
                        }
                    }
                    
                    if (foundTrack) {
                        if (verbose) {
                            console.log("Found track by name in playlist: " + foundTrack.name());
                        }
                        playlist.reveal();
                        playlist.play();
                        foundTrack.play();
                        return "OK: Started playing track by name '" + trackName + "' from playlist '" + playlistName + "'";
                    } else {
                        return "ERROR: Track not found in playlist '" + playlistName + "' by name: " + trackName;
                    }
                } catch (e) {
                    return "ERROR: Failed to search playlist '" + playlistName + "': " + e.message;
                }
            }
            
            // If we have playlist but track lookup failed
            if (trackId !== "") {
                return "ERROR: Track not found in playlist '" + playlistName + "' by ID: " + trackId;
            }
        }
        
        return "ERROR: Unable to process playback request";
    } catch (e) {
        return "ERROR: Script execution failed: " + e.message;
    }
}
