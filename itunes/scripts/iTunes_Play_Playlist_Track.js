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
        // Updated argument parsing to support playlist/album distinction
        // argv[0] = playlist name (optional)
        // argv[1] = album name (optional)
        // argv[2] = track name (optional)  
        // argv[3] = track ID (optional, takes priority over name)
        
        let playlistName = argv.length > 0 ? argv[0] : "";
        let albumName = argv.length > 1 ? argv[1] : "";
        let trackName = argv.length > 2 ? argv[2] : "";
        let trackId = argv.length > 3 ? argv[3] : "";
        
        if (verbose) {
            console.log("Playlist: " + playlistName + ", Album: " + albumName + ", Track: " + trackName + ", ID: " + trackId);
        }

        // If no arguments provided at all, that's an error
        if (playlistName === "" && albumName === "" && trackName === "" && trackId === "") {
            return "ERROR: No playlist, album, track, or track ID specified. Usage: play [playlist] [album] [track] [trackId]"
        }

        // PRIORITY 1: Direct ID-based track lookup (works everywhere)
        if (trackId !== "") {
            if (verbose) {
                console.log("Attempting direct ID-based track lookup: " + trackId);
            }
            
            try {
                let tracksByID = music.tracks.whose({persistentID: trackId});
                if (tracksByID.length > 0) {
                    let foundTrack = tracksByID[0];
                    if (verbose) {
                        console.log("Found track by ID: " + foundTrack.name());
                    }
                    foundTrack.play();
                    return "OK: Started playing track by ID: " + foundTrack.name();
                }
            } catch (e) {
                if (verbose) {
                    console.log("Direct ID lookup failed: " + e.message);
                }
            }
        }

        // PRIORITY 2: Playlist context playback
        if (playlistName !== "") {
            if (verbose) {
                console.log("Attempting playlist-based playback: " + playlistName);
            }
            
            let playlist = null;
            let playlists = music.playlists();
            
            for (let p of playlists) {
                if (p.name() === playlistName) {
                    playlist = p;
                    break;
                }
            }
            
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
                
                // Try to find and play specific track within playlist
                let foundTrack = null;
                let playlistTracks = playlist.tracks();
                
                // Priority to ID lookup within playlist
                if (trackId !== "") {
                    for (let track of playlistTracks) {
                        if (track.persistentID() === trackId) {
                            foundTrack = track;
                            break;
                        }
                    }
                }
                
                // Fallback to name lookup within playlist
                if (!foundTrack && trackName !== "") {
                    for (let track of playlistTracks) {
                        if (track.name.exists() && track.name() === trackName) {
                            foundTrack = track;
                            break;
                        }
                    }
                }
                
                if (foundTrack) {
                    if (verbose) {
                        console.log("Found track in playlist: " + foundTrack.name());
                    }
                    // Proper sequence for context-aware playback
                    music.mute = true;
                    playlist.reveal();
                    playlist.play();
                    foundTrack.play();
                    music.mute = false;
                    return "OK: Started playing track '" + foundTrack.name() + "' from playlist '" + playlistName + "'";
                } else {
                    return "ERROR: Track not found in playlist '" + playlistName + "'";
                }
            } else {
                return "ERROR: Playlist not found: " + playlistName;
            }
        }

        // PRIORITY 3: Album context playback
        if (albumName !== "") {
            if (verbose) {
                console.log("Attempting album-based playback: " + albumName);
            }
            
            try {
                let libraryPlaylist = music.libraryPlaylists[0];
                let libraryTracks = libraryPlaylist.tracks();
                let albumTracks = [];
                let targetTrack = null;
                
                // Collect all tracks from this album
                for (let i = 0; i < libraryTracks.length; i++) {
                    let track = libraryTracks[i];
                    if (track.album.exists() && track.album() === albumName) {
                        albumTracks.push(track);
                        
                        // Check if this is our target track
                        if (trackId !== "" && track.persistentID() === trackId) {
                            targetTrack = track;
                        } else if (trackName !== "" && track.name.exists() && track.name() === trackName) {
                            targetTrack = track;
                        }
                    }
                }
                
                if (albumTracks.length === 0) {
                    return "ERROR: Album not found: " + albumName;
                }
                
                // If no specific track requested, play first track of album
                if (trackName === "" && trackId === "") {
                    if (verbose) {
                        console.log("Playing album from beginning: " + albumName);
                    }
                    albumTracks[0].play();
                    return "OK: Started playing album: " + albumName;
                }
                
                if (targetTrack) {
                    if (verbose) {
                        console.log("Found " + albumTracks.length + " tracks in album, playing: " + targetTrack.name());
                    }
                    // Proper sequence for album context playback
                    music.mute = true;
                    // Start with first track of album to establish context
                    albumTracks[0].play();
                    // Then jump to target track
                    targetTrack.play();
                    music.mute = false;
                    return "OK: Started playing track '" + targetTrack.name() + "' from album '" + albumName + "'";
                } else {
                    return "ERROR: Track not found in album '" + albumName + "'";
                }
            } catch (e) {
                return "ERROR: Album playback failed: " + e.message;
            }
        }

        // FALLBACK: Individual track lookup without context
        if (trackName !== "" || trackId !== "") {
            try {
                let libraryPlaylist = music.libraryPlaylists[0];
                let libraryTracks = libraryPlaylist.tracks();
                
                if (trackName !== "" && trackId === "") {
                    // Name-based individual track search
                    for (let i = 0; i < libraryTracks.length; i++) {
                        let track = libraryTracks[i];
                        if (track.name.exists() && track.name() === trackName) {
                            if (verbose) {
                                console.log("Found individual track by name: " + trackName);
                            }
                            track.play();
                            return "OK: Started playing individual track: " + trackName;
                        }
                    }
                    return "ERROR: Track not found in library by name: " + trackName;
                } else if (trackId !== "") {
                    // ID lookup already failed above - this shouldn't happen
                    return "ERROR: Track not found by ID: " + trackId;
                }
            } catch (e) {
                return "ERROR: Individual track search failed: " + e.message;
            }
        }
        
        return "ERROR: No valid playback parameters provided";
    } catch (e) {
        return "ERROR: Script execution failed: " + e.message;
    }
}
