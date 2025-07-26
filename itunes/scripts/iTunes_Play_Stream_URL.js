#!/usr/bin/env osascript -l JavaScript
// Play Apple Music stream from itmss:// URL
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
            return "ERROR: No URL provided. Usage: play_stream <itmss://url>"
        }
        
        let streamUrl = argv[0]
        
        if (verbose) {
            console.log("Attempting to play stream URL: " + streamUrl)
        }
        
        // Validate URL format
        if (!streamUrl.startsWith('itmss://') && !streamUrl.startsWith('https://music.apple.com/')) {
            return "ERROR: Invalid URL format. Expected itmss:// or https://music.apple.com/ URL"
        }
        
        // Convert https://music.apple.com to itmss:// if needed
        if (streamUrl.startsWith('https://music.apple.com/')) {
            streamUrl = streamUrl.replace('https://music.apple.com/', 'itmss://music.apple.com/')
            if (verbose) {
                console.log("Converted to itmss:// format: " + streamUrl)
            }
        }
        
        try {
            // Use Apple Music's open location command to play the stream
            music.openLocation(streamUrl)
            
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