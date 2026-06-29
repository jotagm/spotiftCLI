// Package webapi is a small client for the Spotify Web API, used to discover
// content (search, playlists, library). Playback itself is handled by the
// go-librespot daemon, not by this package.
package webapi

// User is a subset of the current user's profile (GET /me).
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Product     string `json:"product"` // "premium", "free", ...
}

// Track is a subset of a Spotify track object.
type Track struct {
	URI      string   `json:"uri"`
	Name     string   `json:"name"`
	Duration int      `json:"duration_ms"`
	Artists  []Artist `json:"artists"`
	Album    Album    `json:"album"`
}

// ArtistNames joins the track's artist names with ", ".
func (t Track) ArtistNames() string {
	names := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		names = append(names, a.Name)
	}
	return joinComma(names)
}

// Artist is a subset of a Spotify artist object.
type Artist struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// Album is a subset of a Spotify album object.
type Album struct {
	ID          string   `json:"id"`
	URI         string   `json:"uri"`
	Name        string   `json:"name"`
	Artists     []Artist `json:"artists"`
	TotalTracks int      `json:"total_tracks"`
}

// ArtistNames joins the album's artist names with ", ".
func (a Album) ArtistNames() string {
	names := make([]string, 0, len(a.Artists))
	for _, ar := range a.Artists {
		names = append(names, ar.Name)
	}
	return joinComma(names)
}

// Playlist is a subset of a Spotify playlist object.
type Playlist struct {
	URI    string `json:"uri"`
	Name   string `json:"name"`
	Owner  Owner  `json:"owner"`
	Tracks struct {
		Total int `json:"total"`
	} `json:"tracks"`
}

// Owner is the owner of a playlist.
type Owner struct {
	DisplayName string `json:"display_name"`
}

func joinComma(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += ", " + s
	}
	return out
}
