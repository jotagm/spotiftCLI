package player

import "encoding/json"

// Status represents the full playback status returned by GET /status.
type Status struct {
	Stopped        bool   `json:"stopped"`
	Paused         bool   `json:"paused"`
	Buffering      bool   `json:"buffering"`
	Volume         int    `json:"volume"`
	VolumeSteps    int    `json:"volume_steps"`
	RepeatContext  bool   `json:"repeat_context"`
	RepeatTrack    bool   `json:"repeat_track"`
	ShuffleContext bool   `json:"shuffle_context"`
	Track          *Track `json:"track"`
}

// Track represents a Spotify track in the go-librespot API responses.
type Track struct {
	URI         string   `json:"uri"`
	Name        string   `json:"name"`
	ArtistNames []string `json:"artist_names"`
	AlbumName   string   `json:"album_name"`
	AlbumCover  string   `json:"album_cover_url"`
	Duration    int      `json:"duration"` // milliseconds
}

// Event is a WebSocket event sent by go-librespot on /events.
type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// EventMetadata is the data payload for "metadata" events.
type EventMetadata struct {
	URI         string   `json:"uri"`
	Name        string   `json:"name"`
	ArtistNames []string `json:"artist_names"`
	AlbumName   string   `json:"album_name"`
	AlbumCover  string   `json:"album_cover_url"`
	Duration    int      `json:"duration"` // ms
	Position    int      `json:"position"` // ms
}

// EventSeek is the data payload for "seek" events.
type EventSeek struct {
	Position int `json:"position"` // ms
	Duration int `json:"duration"` // ms
}

// EventVolume is the data payload for "volume" events.
type EventVolume struct {
	Value int `json:"value"`
	Max   int `json:"max"`
}

// EventBool is the data payload for shuffle/repeat toggle events.
type EventBool struct {
	Value bool `json:"value"`
}
