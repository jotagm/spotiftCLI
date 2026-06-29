package webapi

import "net/url"

// SearchTracks searches for tracks matching query via GET /search?type=track.
// The number of results is capped by the app's Spotify quota (5 in dev mode).
func (c *Client) SearchTracks(query string, _ int) ([]Track, error) {
	q := url.Values{
		"q":    {query},
		"type": {"track"},
	}
	var resp struct {
		Tracks struct {
			Items []Track `json:"items"`
		} `json:"tracks"`
	}
	if err := c.get("/search", q, &resp); err != nil {
		return nil, err
	}
	return resp.Tracks.Items, nil
}
