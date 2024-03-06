#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// osacompile -l JavaScript -o iTunes_Play_Tracks.scpt iTunes_Play_Tracks.js
ObjC.import("stdlib")
function run(argv) {
    'use strict';
    var itunes = Application('Music');
    var library = music.sources['Library']
    var verbose = true;

    var args = $.NSProcessInfo.processInfo.arguments                            // NSArray
    var argv = []
    var argc = args.count
    for (var i = 4; i < argc; i++) {
        // skip 3-word run command at top and this file's name
        if (verbose) { console.log($(args.objectAtIndex(i)).js) }               // print each argument
        argv.push(ObjC.unwrap(args.objectAtIndex(i)))                           // collect arguments
    }
    if (verbose) { console.log(argv) }                                          // print arguments
    if(argv.length == 0) {
        if (verbose) { console.log('Usage: play [ playlist] [ track ]'); }
        $.exit(1);
    }
    try {
        
        var playlist = itunes.playlists.byName(argv[0]);
        if (verbose) { console.log('Playing from: ', playlist.name(), JSON.stringify(playlist.properties())) } 
        var tracks = playlist.tracks();
        if (verbose) { tracks.forEach(function(t) { console.log(t.id(), t.name()) } ) } 
        
        itunes.mute = true;
        playlist.reveal();
        playlist.play();

        if ( argv.length > 1) {
            // var track = tracks.find(t => { return t.name() == argv[1] } );
            var track = tracks.find(function(t) { return t.name() == argv[1] } );
            if (verbose) { console.log('Playing: ', track.id(), track.name()) } 
            
            //playlist.stop();
            track.play();
        }
        itunes.mute = false;
    } catch(e) { 
        console.log(e)
        $.exit(2)
    }
}
