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
            console.log('Usage: play [ track ]')
        }
        $.exit(1)
    }
    try {
        let trackName = argv.join(' ');
        let foundTracks = music.search(music.libraryPlaylists[0], { for: trackName });

        if (foundTracks.length > 0) {
            if (verbose) {
                console.log("Found track, playing: " + foundTracks[0].name())
            }
            foundTracks[0].play();
        } else {
            if (verbose) {
                console.log("Track not found in library")
            }
            $.exit(1)
        }
    } catch (e) {
        console.log(e)
        $.exit(2)
    }
}
