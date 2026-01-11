package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const spotifyAPIBaseURL = "https://api.spotify.com/v1"

// Client represents a Spotify API client
type Client struct {
	accessToken string
	httpClient  *http.Client
}

// NewClient creates a new Spotify client
func NewClient(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// GetCurrentTrack retrieves the currently playing track
func (c *Client) GetCurrentTrack() (*Track, error) {
	url := fmt.Sprintf("%s/me/player/currently-playing", spotifyAPIBaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current track: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, fmt.Errorf("no track currently playing")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("spotify API error (status %d): %s", resp.StatusCode, string(body))
	}

	var spotifyResp SpotifyCurrentlyPlaying
	if err := json.NewDecoder(resp.Body).Decode(&spotifyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to our Track struct
	track := &Track{
		Name:      spotifyResp.Item.Name,
		Duration:  time.Duration(spotifyResp.Item.DurationMs) * time.Millisecond,
		Progress:  time.Duration(spotifyResp.ProgressMs) * time.Millisecond,
		IsPlaying: spotifyResp.IsPlaying,
		Shuffle:   spotifyResp.ShuffleState,
		Repeat:    spotifyResp.RepeatState,
	}

	// Get artist names
	if len(spotifyResp.Item.Artists) > 0 {
		artistNames := make([]string, len(spotifyResp.Item.Artists))
		for i, artist := range spotifyResp.Item.Artists {
			artistNames[i] = artist.Name
		}
		track.Artist = artistNames[0] // Use first artist for simplicity
		if len(artistNames) > 1 {
			track.Artist += " & " + artistNames[1]
		}
	}

	// Get album info
	track.Album = spotifyResp.Item.Album.Name
	if len(spotifyResp.Item.Album.Images) > 0 {
		track.ImageURL = spotifyResp.Item.Album.Images[0].URL
	}

	return track, nil
}
