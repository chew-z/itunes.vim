#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// osacompile -l JavaScript -o play.scpt play.js
ObjC.import("stdlib")
function run(argv) {
    'use strict';
    const itunes = Application('iTunes');
    const verbose = false;

    var args = $.NSProcessInfo.processInfo.arguments                            // NSArray
    var argv = []
    var argc = args.count
    for (let i = 4; i < argc; i++) {
        // skip 3-word run command at top and this file's name
        if (verbose) { console.log($(args.objectAtIndex(i)).js) }               // print each argument
        argv.push(ObjC.unwrap(args.objectAtIndex(i)))                           // collect arguments
    }
    if (verbose) { console.log(argv) }                                          // print arguments
    if(argv.length == 0) {
        if (verbose) { console.log('Usage: play [ track ]'); }
        $.exit(1);
    }
    let query = argv.join(' ');
    if (verbose) { console.log(query) }
    try {
        let result = itunes.tracks.whose({ name: { _contains: query } })
        if (result.length > 0) {
            // result.forEach( r => { console.log( `${r.name()}` )})
            result[0].play();
            $.exit(0)
        } else { 
            console.log('Track not found');
            $.exit(1) }
    } catch(e) { 
        console.log('Track unavailable - Offline?')
        console.log(e)
        $.exit(2)
    }
}
