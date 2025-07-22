package itunes

import _ "embed"

//go:embed scripts/iTunes_Play_Playlist_Track.js
var playScript string

//go:embed scripts/iTunes_Refresh_Library.js
var refreshScript string

//go:embed scripts/iTunes_Now_Playing.js
var nowPlayingScript string
