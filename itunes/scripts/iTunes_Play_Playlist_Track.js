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

        // PRIORITY 1: Playlist context playback
        if (playlistName !== "") {
            if (verbose) {
                console.log("Attempting playlist-based playback: " + playlistName);
            }
            
            let playlist = null;
            try {
                playlist = music.playlists.byName(playlistName);
            } catch (e) {
                return "ERROR: Playlist not found: " + playlistName;
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
                
                // Priority to ID lookup within playlist
                if (trackId !== "") {
                    try {
                        let tracksByID = playlist.tracks.whose({persistentID: trackId});
                        if (tracksByID.length > 0) {
                            foundTrack = tracksByID[0];
                        }
                    } catch (e) { /* ignore if not found */ }
                }
                
                // Fallback to name lookup within playlist
                if (!foundTrack && trackName !== "") {
                    try {
                        let tracksByName = playlist.tracks.whose({name: trackName});
                        if (tracksByName.length > 0) {
                            foundTrack = tracksByName[0];
                        }
                    } catch (e) { /* ignore if not found */ }
                }
                
                if (foundTrack) {
                    if (verbose) {
                        console.log("Found track in playlist: " + foundTrack.name());
                    }
                    // Proper sequence for context-aware playback
                    music.mute = true;
                    playlist.play(); // Play the whole playlist to set the queue
                    foundTrack.play(); // Immediately switch to the desired track
                    music.mute = false;
                    return "OK: Started playing track '" + foundTrack.name() + "' from playlist '" + playlistName + "'";
                } else {
                    return "ERROR: Track not found in playlist '" + playlistName + "'";
                }
            }
        }

        // PRIORITY 2: Album context playback
        if (albumName !== "") {
            if (verbose) {
                console.log("Attempting album-based playback: " + albumName);
            }
            
            try {
                let libraryPlaylist = music.libraryPlaylists[0];
                let targetTrack = null;

                // Find the target track first
                if (trackId !== "") {
                    let tracksByID = libraryPlaylist.tracks.whose({persistentID: trackId, album: albumName});
                    if (tracksByID.length > 0) {
                        targetTrack = tracksByID[0];
                    }
                }
                if (!targetTrack && trackName !== "") {
                    let tracksByName = libraryPlaylist.tracks.whose({name: trackName, album: albumName});
                    if (tracksByName.length > 0) {
                        targetTrack = tracksByName[0];
                    }
                }

                // If a specific track is found, play it with album context
                if (targetTrack) {
                    if (verbose) {
                        console.log("Found target track in album: " + targetTrack.name());
                    }
                    // To play a track in album context, we play the track itself.
                    // The Music app should handle the "Up Next" queue automatically.
                    targetTrack.play();
                    return "OK: Started playing track '" + targetTrack.name() + "' from album '" + albumName + "'";
                }

                // If no specific track requested, or if track wasn't found, play the whole album
                let albumTracks = libraryPlaylist.tracks.whose({album: albumName});
                if (albumTracks.length > 0) {
                    if (verbose) {
                        console.log("Playing album from beginning: " + albumName);
                    }
                    albumTracks[0].play();
                    return "OK: Started playing album: " + albumName;
                } else {
                    return "ERROR: Album not found: " + albumName;
                }
            } catch (e) {
                return "ERROR: Album playback failed: " + e.message;
            }
        }

        // PRIORITY 3: Fallback to individual track lookup without context
        if (trackId !== "" || trackName !== "") {
            if (verbose) {
                console.log("Attempting fallback individual track lookup.");
            }
            try {
                let foundTrack = null;
                if (trackId !== "") {
                    let tracksByID = music.tracks.whose({persistentID: trackId});
                    if (tracksByID.length > 0) {
                        foundTrack = tracksByID[0];
                    }
                }
                if (!foundTrack && trackName !== "") {
                    let tracksByName = music.tracks.whose({name: trackName});
                    if (tracksByName.length > 0) {
                        foundTrack = tracksByName[0];
                    }
                }

                if (foundTrack) {
                    if (verbose) {
                        console.log("Found individual track: " + foundTrack.name());
                    }
                    foundTrack.play();
                    return "OK: Started playing individual track: " + foundTrack.name();
                } else {
                    return "ERROR: Track not found in library.";
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
