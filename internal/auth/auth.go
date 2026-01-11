package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	spotifyAuthURL  = "https://accounts.spotify.com/authorize"
	spotifyTokenURL = "https://accounts.spotify.com/api/token"
)

// Auth handles Spotify authentication
type Auth struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	httpClient   *http.Client
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// NewAuth creates a new Auth instance
func NewAuth(clientID, clientSecret, redirectURI string) *Auth {
	return &Auth{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAuthURL generates the authorization URL for the user to visit
func (a *Auth) GetAuthURL(scopes []string, state string) string {
	params := url.Values{}
	params.Set("client_id", a.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", a.RedirectURI)
	params.Set("scope", strings.Join(scopes, " "))
	params.Set("state", state)

	return fmt.Sprintf("%s?%s", spotifyAuthURL, params.Encode())
}

// ExchangeCodeForToken exchanges the authorization code for an access token
func (a *Auth) ExchangeCodeForToken(code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", a.RedirectURI)
	data.Set("client_id", a.ClientID)
	data.Set("client_secret", a.ClientSecret)

	req, err := http.NewRequest("POST", spotifyTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// RefreshToken refreshes an expired access token
func (a *Auth) RefreshToken(refreshToken string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", a.ClientID)
	data.Set("client_secret", a.ClientSecret)

	req, err := http.NewRequest("POST", spotifyTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	return &tokenResp, nil
}

// StartAuthFlow starts the OAuth flow and waits for the callback
func (a *Auth) StartAuthFlow(ctx context.Context, scopes []string) (*TokenResponse, error) {
	// Generate random state for security
	state, err := generateRandomString(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Create channel to receive the authorization code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start HTTP server to handle callback
	server := &http.Server{Addr: ":8888"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Verify state
		if r.URL.Query().Get("state") != state {
			errChan <- fmt.Errorf("state mismatch")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "State mismatch error")
			return
		}

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "No authorization code received")
			return
		}

		codeChan <- code
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Authorization successful! You can close this window and return to the CLI.")
	})

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("failed to start callback server: %w", err)
		}
	}()

	// Generate and print auth URL
	authURL := a.GetAuthURL(scopes, state)
	fmt.Println("\nPlease visit this URL to authorize the application:")
	fmt.Println(authURL)
	fmt.Println("\nWaiting for authorization...")

	// Wait for code or error
	var code string
	select {
	case code = <-codeChan:
		// Success
	case err := <-errChan:
		server.Shutdown(ctx)
		return nil, err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return nil, fmt.Errorf("authorization timeout")
	}

	// Shutdown server
	server.Shutdown(context.Background())

	// Exchange code for token
	token, err := a.ExchangeCodeForToken(code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	return token, nil
}

// generateRandomString generates a random string of the specified length
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
