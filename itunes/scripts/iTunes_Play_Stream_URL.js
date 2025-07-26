#!/usr/bin/env osascript -l JavaScript
// Play streaming URL in Apple Music - supports various streaming formats
ObjC.import('stdlib')

function run(argv) {
    'use strict'
    let music = Application('Music')
    const verbose = false

    var args = $.NSProcessInfo.processInfo.arguments // NSArray
    var argv = []
    var argc = args.count
    for (let i = 0; i < argc; i++) {
        if (verbose) {
            console.log("Arg " + i + ": " + $(args.objectAtIndex(i)).js)
        }
    }
    for (let i = 2; i < argc; i++) {
        argv.push(ObjC.unwrap(args.objectAtIndex(i)))
    }
    
    try {
        if (argv.length === 0) {
            return "ERROR: No URL provided. Usage: play_stream <streaming_url>"
        }
        
        let streamUrl = argv[0]
        
        if (verbose) {
            console.log("Attempting to play stream URL: " + streamUrl)
        }
        
        // Validate that it's some kind of URL
        if (!streamUrl.includes('://')) {
            return "ERROR: Invalid URL format. Please provide a valid streaming URL (http://, https://, itmss://, etc.)"
        }
        
        try {
            // Activate Music app and use openLocation to play any streaming URL
            music.activate()
            music.openLocation(streamUrl)
            music.play()
            
            // Small delay to allow stream to start
            $.NSThread.sleepForTimeInterval(1.0)
            
            // Get the current track info to confirm playback
            let currentTrack = music.currentTrack
            if (currentTrack) {
                let trackName = currentTrack.name()
                return "OK: Started streaming: " + trackName
            } else {
                return "OK: Stream command sent successfully"
            }
            
        } catch (openError) {
            return "ERROR: Failed to open stream URL '" + streamUrl + "': " + openError.message
        }
        
    } catch (e) {
        return "ERROR: Script execution failed: " + e.message
    }
}