package itunes

import _ "embed"

//go:embed scripts/iTunes_Search2_fzf.js
var searchScript string

//go:embed scripts/iTunes_Play_Playlist_Track.js
var playScript string
