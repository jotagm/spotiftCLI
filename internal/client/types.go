package client

import "time"

// Track represents a Spotify track
type Track struct {
	Name      string
	Artist    string
	Album     string
	Duration  time.Duration
	Progress  time.Duration
	IsPlaying bool
	Shuffle   bool
	Repeat    string // "off", "track", "context"
	ImageURL  string
}

// SpotifyCurrentlyPlaying represents the API response for currently playing track
type SpotifyCurrentlyPlaying struct {
	Item struct {
		Name       string `json:"name"`
		DurationMs int    `json:"duration_ms"`
		Album      struct {
			Name   string `json:"name"`
			Images []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"album"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
	} `json:"item"`
	ProgressMs   int    `json:"progress_ms"`
	IsPlaying    bool   `json:"is_playing"`
	ShuffleState bool   `json:"shuffle_state"`
	RepeatState  string `json:"repeat_state"`
}

// Playlist represents a Spotify playlist
type Playlist struct {
	ID          string
	Name        string
	Description string
	Public      bool
	TrackCount  int
}

// PlaylistTrack represents a track in a playlist
type PlaylistTrack struct {
	Track   Track
	AddedAt time.Time
}
