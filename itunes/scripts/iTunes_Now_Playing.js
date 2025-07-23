#!/usr/bin/env osascript -l JavaScript
// @flow
// @flow-NotIssue
ObjC.import('stdlib')

function run(argv) {
    'use strict'
    let music = Application('Music')
    const verbose = false

    try {
        // Check if Music app is running
        if (!music.running()) {
            return JSON.stringify({
                status: "stopped",
                message: "Music app is not running"
            });
        }

        // Get player state
        let playerState = music.playerState();
        
        if (verbose) {
            console.log("Player state: " + playerState);
        }

        // If not playing, return appropriate status
        if (playerState !== "playing") {
            return JSON.stringify({
                status: playerState.toString(),
                message: playerState === "paused" ? "Music is paused" : "No music playing"
            });
        }

        // Get current track info
        let currentTrack = music.currentTrack;
        if (!currentTrack || !currentTrack.exists()) {
            return JSON.stringify({
                status: "playing",
                message: "Playing but no track information available"
            });
        }

        let trackName = currentTrack.name.exists() ? currentTrack.name() : "Unknown Track";
        let artistName = currentTrack.artist.exists() ? currentTrack.artist() : "Unknown Artist";
        let albumName = currentTrack.album.exists() ? currentTrack.album() : "Unknown Album";
        let trackID = currentTrack.persistentID.exists() ? currentTrack.persistentID() : "";
        
        // Detect streaming tracks
        let trackKind = currentTrack.kind.exists() ? currentTrack.kind() : "";
        let isStreaming = trackKind === "Internet audio stream";
        
        if (isStreaming) {
            // For streaming tracks, include position but no duration (continuous stream)
            let playerPosition = music.playerPosition();
            let streamURL = "";
            try {
                if (currentTrack.address.exists()) {
                    streamURL = currentTrack.address();
                }
            } catch (e) {}
            
            // Format position as MM:SS
            let formatTime = function(seconds) {
                let mins = Math.floor(seconds / 60);
                let secs = Math.floor(seconds % 60);
                return mins + ":" + (secs < 10 ? "0" : "") + secs;
            };
            
            return JSON.stringify({
                status: "playing",
                track: {
                    id: trackID,
                    name: trackName,
                    artist: artistName,
                    album: albumName,
                    position: formatTime(playerPosition),
                    position_seconds: Math.floor(playerPosition),
                    is_streaming: true,
                    kind: trackKind,
                    stream_url: streamURL
                },
                display: trackName + (artistName ? " – " + artistName : "") + " [STREAMING]"
            });
        } else {
            // For local tracks, return standard structure with position/duration
            let playerPosition = music.playerPosition();
            let duration = currentTrack.duration.exists() ? currentTrack.duration() : 0;
            
            // Format position as MM:SS
            let formatTime = function(seconds) {
                let mins = Math.floor(seconds / 60);
                let secs = Math.floor(seconds % 60);
                return mins + ":" + (secs < 10 ? "0" : "") + secs;
            };

            return JSON.stringify({
                status: "playing",
                track: {
                    id: trackID,
                    name: trackName,
                    artist: artistName,
                    album: albumName,
                    position: formatTime(playerPosition),
                    duration: formatTime(duration),
                    position_seconds: Math.floor(playerPosition),
                    duration_seconds: Math.floor(duration)
                },
                display: trackName + " – " + artistName
            });
        }

    } catch (e) {
        return JSON.stringify({
            status: "error",
            message: "Failed to get current track: " + e.message,
            error: e.name
        });
    }
}