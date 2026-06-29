package webapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const apiBase = "https://api.spotify.com/v1"

// Client calls the Spotify Web API with a bearer token, refreshing it
// automatically when it expires.
type Client struct {
	auth *Authenticator
	http *http.Client

	mu  sync.Mutex
	tok *Token
}

// NewClient returns a Client. It loads a saved token if present; otherwise it
// runs the interactive login flow. Either way the token is valid on return.
func NewClient(auth *Authenticator) (*Client, error) {
	c := &Client{
		auth: auth,
		http: &http.Client{Timeout: 15 * time.Second},
		tok:  auth.LoadToken(),
	}

	if c.tok == nil {
		tok, err := auth.Login()
		if err != nil {
			return nil, err
		}
		c.tok = tok
	} else if !c.tok.valid() {
		tok, err := auth.Refresh(c.tok)
		if err != nil {
			// Refresh failed (revoked/expired) — fall back to a fresh login.
			tok, err = auth.Login()
			if err != nil {
				return nil, err
			}
		}
		c.tok = tok
	}

	return c, nil
}

// accessToken returns a currently-valid access token, refreshing if needed.
func (c *Client) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.tok.valid() {
		tok, err := c.auth.Refresh(c.tok)
		if err != nil {
			return "", err
		}
		c.tok = tok
	}
	return c.tok.AccessToken, nil
}

// get performs an authenticated GET against the Web API and decodes the JSON
// response into out. path is relative to apiBase; query may be nil.
func (c *Client) get(path string, query url.Values, out any) error {
	full := apiBase + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	resp, err := c.do(http.MethodGet, full)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s returned %d: %s", path, resp.StatusCode, body)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// do issues the request with a bearer token, refreshing once on a 401.
func (c *Client) do(method, fullURL string) (*http.Response, error) {
	token, err := c.accessToken()
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest(method, fullURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.mu.Lock()
		tok, rerr := c.auth.Refresh(c.tok)
		if rerr == nil {
			c.tok = tok
		}
		c.mu.Unlock()
		if rerr != nil {
			return nil, fmt.Errorf("unauthorized and token refresh failed: %w", rerr)
		}
		req, _ := http.NewRequest(method, fullURL, nil)
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		return c.http.Do(req)
	}

	return resp, nil
}

// CurrentUser fetches the authenticated user's profile (GET /me).
func (c *Client) CurrentUser() (*User, error) {
	var u User
	if err := c.get("/me", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}
