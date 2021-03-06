JsOsaDAS1.001.00bplist00�Vscript_�// @flow
// @flow-NotIssue
// osacompile -l JavaScript -o iTunes_Play_Tracks.scpt iTunes_Play_Tracks.js
ObjC.import("stdlib")
function run(argv) {
    'use strict';
    const itunes = Application('iTunes');
    const library = itunes.sources.whose({kind: "klib"})[0];
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
        if (verbose) { console.log('Usage: play [ playlist] [ track ]'); }
        $.exit(1);
    }
    try {
        
        let playlist = itunes.playlists.byName(argv[0]);
        if (verbose) { console.log('Playing from: ', playlist.name(), JSON.stringify(playlist.properties())) } 
        let tracks = playlist.tracks();
        if (verbose) { tracks.forEach(t => { console.log(t.id(), t.name()) } ) } 
        
        itunes.mute = true;
        playlist.reveal();
        playlist.play();

        if ( argv.length > 1) {
            let track = tracks.find(t => { return t.name() == argv[1] } );
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
                              � jscr  ��ޭ