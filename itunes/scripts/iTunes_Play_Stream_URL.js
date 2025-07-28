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
        
        // Validate URL format and protocol
        if (!streamUrl.includes('://')) {
            return "ERROR: Invalid URL format. Please provide a valid streaming URL (http://, https://, itmss://, etc.)"
        }
        
        // Check for supported protocols
        const supportedProtocols = ['http://', 'https://', 'itmss://']
        const isSupported = supportedProtocols.some(protocol => streamUrl.toLowerCase().startsWith(protocol))
        if (!isSupported) {
            return "ERROR: Unsupported URL protocol. Supported protocols: " + supportedProtocols.join(', ')
        }
        
        // Special handling for itmss:// URLs - these are Apple Music's internal protocol
        if (streamUrl.toLowerCase().startsWith('itmss://')) {
            if (verbose) {
                console.log("Detected Apple Music internal protocol (itmss://)")
            }
        }
        
        try {
            // Activate Music app and use openLocation to play any streaming URL
            music.activate()
            music.openLocation(streamUrl)
            music.play()
            
            // Give more time for Apple Music to process the URL, especially for itmss:// URLs
            const waitTime = streamUrl.toLowerCase().startsWith('itmss://') ? 3.0 : 2.0
            $.NSThread.sleepForTimeInterval(waitTime)
            
            // Get the current track info to confirm playback
            let currentTrack = music.currentTrack
            if (currentTrack) {
                let trackName = currentTrack.name()
                let trackKind = ""
                let trackAddress = ""
                
                try {
                    trackKind = currentTrack.kind ? currentTrack.kind() : "unknown"
                } catch (e) {
                    if (verbose) {
                        console.log("Warning: Could not get track kind: " + e.message)
                    }
                }
                
                try {
                    trackAddress = currentTrack.address ? currentTrack.address() : ""
                } catch (e) {
                    if (verbose) {
                        console.log("Warning: Could not get track address: " + e.message)
                    }
                }
                
                // Enhanced success message with validation
                let message = "OK: Started streaming: " + trackName + " (kind: " + trackKind + ", requested: " + streamUrl
                if (trackAddress && trackAddress !== streamUrl) {
                    message += ", actual: " + trackAddress
                }
                message += ")"
                
                return message
            } else {
                // No current track - this might indicate an issue with itmss:// URLs
                if (streamUrl.toLowerCase().startsWith('itmss://')) {
                    return "ERROR: Apple Music failed to play itmss:// URL. URL may be invalid or service unavailable: " + streamUrl
                } else {
                    return "OK: Stream command sent for: " + streamUrl
                }
            }
            
        } catch (openError) {
            return "ERROR: Failed to open stream URL '" + streamUrl + "': " + openError.message
        }
        
    } catch (e) {
        return "ERROR: Script execution failed: " + e.message
    }
}