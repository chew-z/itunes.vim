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
        // Try to read from cache first
        ObjC.import('Foundation')
        let tmpDir = $.NSTemporaryDirectory().js
        let cacheDir = tmpDir + "itunes-cache"
        let cacheFilePath = cacheDir + "/library.json"
        
        // Check if cache file exists
        let fileManager = $.NSFileManager.defaultManager
        let fileExists = fileManager.fileExistsAtPath(cacheFilePath)
        
        if (!fileExists) {
            if (verbose) {
                console.log("Cache file does not exist. Please refresh library first.")
            }
            return JSON.stringify({ status: "error", message: "Cache file does not exist. Please refresh library first." })
        }
        
        // Read the cache file
        let data = $.NSData.dataWithContentsOfFile(cacheFilePath)
        if (!data) {
            if (verbose) {
                console.log("Could not read cache file")
            }
            return JSON.stringify({ status: "error", message: "Could not read cache file" })
        }
        
        let jsonString = $.NSString.alloc.initWithDataEncoding(data, $.NSUTF8StringEncoding).js
        let allTracks = JSON.parse(jsonString)
        
        if (verbose) {
            console.log("Loaded " + allTracks.length + " tracks from cache")
        }
        
        // Search through cached tracks with result limiting
        let matches = []
        let exactMatches = []
        let partialMatches = []
        let queryLower = searchQuery.toLowerCase()
        
        for (let track of allTracks) {
            let trackName = (track.name || "").toLowerCase()
            let artistName = (track.artist || "").toLowerCase()
            let albumName = (track.album || "").toLowerCase()
            let collectionName = (track.collection || "").toLowerCase()
            
            // Check for exact matches first (higher priority)
            if (trackName === queryLower || artistName === queryLower) {
                exactMatches.push(track)
            }
            // Then partial matches
            else {
                let searchableText = [collectionName, trackName, artistName, albumName].join(' ')
                if (searchableText.includes(queryLower)) {
                    partialMatches.push(track)
                }
            }
        }
        
        // Combine results with exact matches first, limit to 15 total
        matches = exactMatches.concat(partialMatches).slice(0, 15)
        
        if (matches.length === 0) {
            if (verbose) {
                console.log("No tracks found matching query.")
            }
            return JSON.stringify({ status: "success", data: [], message: "No tracks found matching query." })
        }
        
        if (verbose) {
            console.log("Found " + matches.length + " matches (" + exactMatches.length + " exact, " + Math.min(partialMatches.length, 15 - exactMatches.length) + " partial)")
        }

        return JSON.stringify({ status: "success", data: matches });
    } catch (e) {
        return JSON.stringify({ status: "error", message: "Search error: " + e.message, error: e.name })
    }
}
