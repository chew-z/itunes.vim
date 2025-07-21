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
    if (argv.length == 0) {
        if (verbose) {
            console.log('Usage: play playlist [ track ]')
        }
        return JSON.stringify({ status: "error", message: "No playlist specified. Usage: play playlist [ track ]" })
    }
    try {
        let playlistName = argv[0];
        let trackName = argv.length > 1 ? argv.slice(1).join(' ') : "";
        
        if (verbose) {
            console.log("Playlist: " + playlistName + ", Track: " + trackName);
        }

        // Find the playlist by name
        let playlist = null;
        let playlists = music.playlists();
        
        for (let p of playlists) {
            if (p.name() === playlistName) {
                playlist = p;
                break;
            }
        }
        
        if (!playlist) {
            if (verbose) {
                console.log("Playlist not found: " + playlistName);
            }
            return JSON.stringify({ status: "error", message: "Playlist not found: " + playlistName })
        }
        
        // If no specific track is requested, play the entire playlist
        if (trackName === "") {
            if (verbose) {
                console.log("Playing entire playlist: " + playlistName);
            }
            playlist.play();
            return JSON.stringify({ status: "success", message: "Started playing playlist: " + playlistName })
        } else {
            // Find the specific track within the playlist
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
                    console.log("Found track in playlist, playing: " + foundTrack.name());
                }
                // Play the playlist first to set context, then play the specific track
                playlist.reveal();
                playlist.play();
                foundTrack.play();
                return JSON.stringify({ status: "success", message: "Started playing track '" + trackName + "' from playlist '" + playlistName + "'" })
            } else {
                if (verbose) {
                    console.log("Track not found in playlist: " + trackName);
                }
                return JSON.stringify({ status: "error", message: "Track not found in playlist: " + trackName })
            }
        }
    } catch (e) {
        return JSON.stringify({ status: "error", message: "Script error: " + e.message, error: e.name })
    }
}
