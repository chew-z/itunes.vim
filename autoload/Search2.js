#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// osacompile -l JavaScript -o iTunes_Search2_fzf.scpt iTunes_Search2_fzf.js
ObjC.import('stdlib')

function run(argv) {
    'use strict'
    var music = Application('Music')
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
    let searchQuery = argv.join(' ')
    if (verbose) {
        console.log(searchQuery)
    }

    try {
        let playlists = music.playlists
            .whose({ name: { _contains: searchQuery } })()
            .filter((p) => {
                return p.duration() > 0 && p.id() > 65
            })
        if (verbose) {
            playlists.forEach((p) => {
                console.log(p.name(), p.class())
            })
        }

        // function flatten(arr) { return Array.prototype.concat.apply([], arr); }
        function flatten(arr) {
            return arr.reduce((a, b) => a.concat(b), [])
        }
        let tr
        tr = flatten(
            playlists.map((p) => {
                return p.tracks().map((t) => {
                    // return `${p.name()} | ${t.artist()} | ${t.album()} | ${t.name()}`
                    return {
                        id: t.id(),
                        name: t.name(),
                        album: t.album(),
                        collection: p.name(),
                        artist: t.artist(),
                    }
                })
            })
        )

        if (tr.length > 0) {
            // return tr.join('\n');
            return JSON.stringify(tr, null, 4)
            // return JSON.stringify(tr)
        } else {
            $.exit(1)
        }
    } catch (e) {
        console.log(e)
        $.exit(2)
    }
}
