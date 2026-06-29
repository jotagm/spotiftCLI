package webapi

import "strings"

// SavedAlbums returns the user's saved albums (GET /me/albums).
func (c *Client) SavedAlbums() ([]Album, error) {
	var resp struct {
		Items []struct {
			Album Album `json:"album"`
		} `json:"items"`
	}
	if err := c.get("/me/albums", nil, &resp); err != nil {
		return nil, err
	}
	albums := make([]Album, 0, len(resp.Items))
	for _, item := range resp.Items {
		if item.Album.ID != "" {
			albums = append(albums, item.Album)
		}
	}
	return albums, nil
}

// AlbumTracks returns the tracks in an album (GET /albums/{id}/tracks).
func (c *Client) AlbumTracks(albumID string) ([]Track, error) {
	var resp struct {
		Items []Track `json:"items"`
	}
	if err := c.get("/albums/"+albumID+"/tracks", nil, &resp); err != nil {
		return nil, err
	}
	tracks := make([]Track, 0, len(resp.Items))
	for _, t := range resp.Items {
		if t.URI != "" {
			tracks = append(tracks, t)
		}
	}
	return tracks, nil
}

// UserPlaylists returns the authenticated user's playlists (GET /me/playlists).
func (c *Client) UserPlaylists() ([]Playlist, error) {
	var resp struct {
		Items []Playlist `json:"items"`
	}
	if err := c.get("/me/playlists", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// PlaylistTracks returns the tracks in a playlist (GET /playlists/{id}/items).
// playlistURI may be a full Spotify URI (spotify:playlist:ID) or a bare ID.
// The February 2026 API migration renamed the endpoint from /tracks to /items
// and the per-item field from "track" to "item"; both fields are still present.
func (c *Client) PlaylistTracks(playlistURI string) ([]Track, error) {
	id := uriID(playlistURI)
	var resp struct {
		Items []struct {
			Item  *Track `json:"item"`  // primary field (Feb 2026+)
			Track *Track `json:"track"` // legacy field (still populated)
		} `json:"items"`
	}
	if err := c.get("/playlists/"+id+"/items", nil, &resp); err != nil {
		return nil, err
	}
	tracks := make([]Track, 0, len(resp.Items))
	for _, item := range resp.Items {
		t := item.Item
		if t == nil {
			t = item.Track
		}
		if t != nil && t.URI != "" && strings.HasPrefix(t.URI, "spotify:track:") {
			tracks = append(tracks, *t)
		}
	}
	return tracks, nil
}

// SavedTracks returns the user's Liked Songs (GET /me/tracks).
func (c *Client) SavedTracks() ([]Track, error) {
	var resp struct {
		Items []struct {
			Track *Track `json:"track"`
		} `json:"items"`
	}
	if err := c.get("/me/tracks", nil, &resp); err != nil {
		return nil, err
	}
	tracks := make([]Track, 0, len(resp.Items))
	for _, item := range resp.Items {
		if item.Track != nil && item.Track.URI != "" {
			tracks = append(tracks, *item.Track)
		}
	}
	return tracks, nil
}

// uriID extracts the resource ID from a Spotify URI (spotify:type:id),
// or returns the string unchanged if it is already a bare ID.
func uriID(uri string) string {
	parts := strings.Split(uri, ":")
	if len(parts) == 3 {
		return parts[2]
	}
	return uri
}
