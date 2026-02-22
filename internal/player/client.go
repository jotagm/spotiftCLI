package player

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the go-librespot REST API.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient creates a new Client targeting the given port.
func NewClient(port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://localhost:%d", port),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Status fetches the current playback status from GET /status.
func (c *Client) Status() (*Status, error) {
	resp, err := c.http.Get(c.baseURL + "/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status endpoint returned %d", resp.StatusCode)
	}

	var s Status
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, fmt.Errorf("decoding status: %w", err)
	}
	return &s, nil
}

// PlayPause toggles play/pause via POST /player/playpause.
func (c *Client) PlayPause() error {
	return c.postEmpty("/player/playpause")
}

// Next skips to the next track via POST /player/next.
func (c *Client) Next() error {
	return c.postJSON("/player/next", map[string]any{})
}

// Prev goes to the previous track via POST /player/prev.
func (c *Client) Prev() error {
	return c.postEmpty("/player/prev")
}

// SetVolume sets the absolute volume (0–100) via POST /player/volume.
func (c *Client) SetVolume(vol int) error {
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	return c.postJSON("/player/volume", map[string]any{
		"volume":   vol,
		"relative": false,
	})
}

// SetVolumeRelative changes volume by a relative delta via POST /player/volume.
func (c *Client) SetVolumeRelative(delta int) error {
	return c.postJSON("/player/volume", map[string]any{
		"volume":   delta,
		"relative": true,
	})
}

// Seek seeks to the given position in milliseconds via POST /player/seek.
func (c *Client) Seek(ms int) error {
	return c.postJSON("/player/seek", map[string]any{
		"position": ms,
		"relative": false,
	})
}

// SetShuffle enables or disables shuffle via POST /player/shuffle_context.
func (c *Client) SetShuffle(on bool) error {
	return c.postJSON("/player/shuffle_context", map[string]any{
		"shuffle_context": on,
	})
}

// SetRepeatContext enables or disables context repeat via POST /player/repeat_context.
func (c *Client) SetRepeatContext(on bool) error {
	return c.postJSON("/player/repeat_context", map[string]any{
		"repeat_context": on,
	})
}

// SetRepeatTrack enables or disables track repeat via POST /player/repeat_track.
func (c *Client) SetRepeatTrack(on bool) error {
	return c.postJSON("/player/repeat_track", map[string]any{
		"repeat_track": on,
	})
}

// postEmpty sends a POST with no body.
func (c *Client) postEmpty(path string) error {
	resp, err := c.http.Post(c.baseURL+path, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s returned %d: %s", path, resp.StatusCode, body)
	}
	return nil
}

// postJSON sends a POST with a JSON body.
func (c *Client) postJSON(path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s returned %d: %s", path, resp.StatusCode, b)
	}
	return nil
}
