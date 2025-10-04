package model

// Track describes one track from the script's output
type Track struct {
	ID           string   `json:"id"`
	PersistentID string   `json:"persistent_id,omitempty"` // Apple Music persistent ID (Phase 2)
	Name         string   `json:"name"`
	Album        string   `json:"album"`
	Collection   string   `json:"collection"` // Primary playlist name or album if not in a playlist
	Artist       string   `json:"artist"`
	Playlists    []string `json:"playlists"`            // All playlists containing this track
	Genre        string   `json:"genre,omitempty"`      // Phase 2: Track genre
	Rating       int      `json:"rating,omitempty"`     // Phase 2: Track rating (0-100)
	Starred      bool     `json:"starred,omitempty"`    // Phase 2: Loved/starred status
	IsStreaming  bool     `json:"is_streaming"`         // Streaming track detection
	Kind         string   `json:"kind,omitempty"`       // Track type (e.g., "Internet audio stream")
	StreamURL    string   `json:"stream_url,omitempty"` // Stream URL for streaming tracks
}

// PlaylistData represents playlist metadata with persistent ID (Phase 2)
type PlaylistData struct {
	ID          string `json:"id"` // Persistent ID
	Name        string `json:"name"`
	SpecialKind string `json:"special_kind"` // "none" for user playlists
	TrackCount  int    `json:"track_count"`
	Genre       string `json:"genre,omitempty"`
}

// Station represents an Apple Music radio station
type Station struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Genre       string   `json:"genre"`
	Homepage    string   `json:"homepage,omitempty"` // https:// web URL for browser access
	Keywords    []string `json:"keywords"`           // For backward compatibility
}

// StationSearchResult represents the result of a station search
type StationSearchResult struct {
	Status   string    `json:"status"`
	Query    string    `json:"query"`
	Stations []Station `json:"stations"`
	Count    int       `json:"count"`
	Message  string    `json:"message,omitempty"`
}

// RefreshStats contains statistics from a library refresh operation
type RefreshStats struct {
	TotalTracks    int `json:"total_tracks"`
	TotalPlaylists int `json:"total_playlists"`
	ProcessingTime int `json:"processing_time_ms"`
}

// RefreshData contains the tracks and playlists from a refresh operation
type RefreshData struct {
	Tracks    []Track        `json:"tracks"`
	Playlists []PlaylistData `json:"playlists"`
	Stats     RefreshStats   `json:"stats"`
}

// RefreshResponse represents the complete response from the refresh script
type RefreshResponse struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    *RefreshData           `json:"data"`
	Error   string                 `json:"error,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// NowPlayingTrack contains current track information with playback details
type NowPlayingTrack struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Artist          string `json:"artist"`
	Album           string `json:"album"`
	Position        string `json:"position"`
	Duration        string `json:"duration"`
	PositionSeconds int    `json:"position_seconds"`
	DurationSeconds int    `json:"duration_seconds"`
	IsStreaming     bool   `json:"is_streaming"`
	Kind            string `json:"kind,omitempty"`
	StreamURL       string `json:"stream_url,omitempty"`
}

// JsNowPlayingResponse represents the raw response from JavaScript
type JsNowPlayingResponse struct {
	Status  string           `json:"status"`
	Track   *NowPlayingTrack `json:"track,omitempty"`
	Display string           `json:"display"`
	Message string           `json:"message"`
}

// StreamingTrack contains streaming track information
type StreamingTrack struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StreamURL      string `json:"stream_url"`
	Kind           string `json:"kind"`
	Elapsed        string `json:"elapsed"`
	ElapsedSeconds int    `json:"elapsed_seconds"`
}

// NowPlayingStatus represents the current playback status
type NowPlayingStatus struct {
	Status  string           `json:"status"`           // "playing", "paused", "stopped", "error", "streaming", "streaming_paused"
	Track   *NowPlayingTrack `json:"track,omitempty"`  // For local tracks
	Stream  *StreamingTrack  `json:"stream,omitempty"` // For streaming tracks
	Display string           `json:"display"`          // Formatted display string
	Message string           `json:"message"`
}

// PlayResult contains the result of a play operation with current track info
type PlayResult struct {
	Success    bool              `json:"success"`
	Message    string            `json:"message"`
	NowPlaying *NowPlayingStatus `json:"now_playing,omitempty"`
}

// EQStatus represents the state of the Apple Music Equalizer.
type EQStatus struct {
	Enabled          bool     `json:"enabled"`
	CurrentPreset    *string  `json:"current_preset"` // Use pointer to handle null when disabled
	AvailablePresets []string `json:"available_presets"`
	Message          string   `json:"message,omitempty"` // Optional informational message
}

// AudioOutput describes the current audio output device.
type AudioOutput struct {
	OutputType string `json:"output_type"` // "local" or "airplay"
	DeviceName string `json:"device_name"`
	Error      string `json:"error,omitempty"`
}

// AirPlayDevice represents an AirPlay-capable audio output device.
type AirPlayDevice struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Selected    bool   `json:"selected"`
	SoundVolume int    `json:"sound_volume"`
	Error       string `json:"error,omitempty"`
}