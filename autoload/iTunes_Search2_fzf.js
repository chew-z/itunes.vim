#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// osacompile -l JavaScript -o iTunes_Search2_fzf.scpt iTunes_Search2_fzf.js
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
        console.log("Arguments: " + argv)
    } // print arguments
    let searchQuery = argv.join(' ')
    if (verbose) {
        console.log("Search Query: " + searchQuery)
    }

    try {
        let foundTracks = music.search(music.libraryPlaylists[0], { for: searchQuery });

        if (foundTracks.length === 0) {
            if (verbose) {
                console.log("No tracks found matching query.")
            }
            $.exit(1)
        }

        let tr = foundTracks.map(t => {
            return {
                id: String(t.id()),
                name: t.name.exists() ? t.name() : "",
                album: t.album.exists() ? t.album() : "",
                collection: t.album.exists() ? t.album() : "", // Using album as collection
                artist: t.artist.exists() ? t.artist() : "",
            }
        })

        if (tr.length > 0) {
            return JSON.stringify(tr)
        } else {
            $.exit(1)
        }
    } catch (e) {
        console.log(e)
        $.exit(2)
    }
}
