#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
// osacompile -l JavaScript -o iTunes_Search2_fzf.scpt iTunes_Search2_fzf.js
ObjC.import('stdlib')

function run(argv) { 
    'use strict';
    const itunes = Application('iTunes');
    const library = itunes.sources.whose({kind: "klib"})[0];                    // Library locale independent
    const verbose = false;

    var args = $.NSProcessInfo.processInfo.arguments                            // NSArray
    var argv = []
    var argc = args.count
    for (let i = 4; i < argc; i++) {
        // skip 3-word run command at top and this file's name
        if (verbose) { console.log($(args.objectAtIndex(i)).js) }               // print each argument
        argv.push(ObjC.unwrap(args.objectAtIndex(i)))                           // collect arguments
    }
    if (verbose) { console.log(argv); }                                         // print arguments
    if (argc == 4) { argv = ['Offline', 'Library']; }                           // if empty initialize with defaults
    let searchQuery = 'Library';
    if (argv[0] == 'Offline' || argv[0] == 'Online') {
        searchQuery = argv.slice(1).join(' ');
    } else {
       searchQuery = argv.join(' '); 
    }
    if (verbose) { console.log(searchQuery); }
    
	try {
        let playlists = [library.playlists()[0]];                                 // Library playlist
        if (searchQuery !== 'Library') {
            playlists = itunes.playlists.whose( { name: { _contains: searchQuery } })().filter(p => {
		        return p.duration() > 0 ; });
        }
        if (verbose) { playlists.forEach( p => { console.log(p.name(), p.class()) }) }

        // function flatten(arr) { return Array.prototype.concat.apply([], arr); }
        function flatten(arr) { return arr.reduce((a, b) => a.concat(b), []); }
        let tr
        if (argv[0] === 'Online') {
            tr = flatten(playlists.map( p => { 
                return p.tracks().map( t => { 
                    // return `${p.name()} | ${t.artist()} | ${t.album()} | ${t.name()}` 
                    return { id: t.id(), name: t.name(), album: t.album(), collection: p.name(), artist: t.artist() }
                }) 
            }))
        } else  {
            tr = flatten(playlists.map( p => { 
                return p.fileTracks().map( t => { 
                    // return `${p.name()} | ${t.artist()} | ${t.album()} | ${t.name()}` 
                    return { id: t.id(), name: t.name(), album: t.album(), collection: p.name(), artist: t.artist() }
                }) 
            })) }
       
        if (tr.length > 0) {
             // return tr.join('\n');
             return JSON.stringify(tr);
        } else { $.exit(1) }
	} catch(e) {
		console.log(e);
        $.exit(2)
	}
}
