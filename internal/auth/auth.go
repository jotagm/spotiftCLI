package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

const state = "abc123"

// Auth handles Spotify authentication
type Auth struct {
	authenticator *spotifyauth.Authenticator
	redirectURI   string
	clientChan    chan *spotify.Client
	server        *http.Server
}

// NewAuth creates a new Auth instance
func NewAuth(clientID, clientSecret, redirectURI string) *Auth {
	auth := spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopeUserReadPlaybackState,
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopePlaylistModifyPublic,
			spotifyauth.ScopePlaylistModifyPrivate,
		),
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithClientSecret(clientSecret),
	)

	return &Auth{
		authenticator: auth,
		redirectURI:   redirectURI,
		clientChan:    make(chan *spotify.Client),
	}
}

// StartAuthFlow starts the OAuth flow and returns an authenticated Spotify client
func (a *Auth) StartAuthFlow(ctx context.Context) (*spotify.Client, error) {
	// Start HTTP server to handle callback
	http.HandleFunc("/callback", a.completeAuth)

	a.server = &http.Server{Addr: ":8888"}
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Generate auth URL and prompt user
	url := a.authenticator.AuthURL(state)
	fmt.Println("\nPlease visit this URL to authorize the application:")
	fmt.Println(url)
	fmt.Println("\nWaiting for authorization...")

	// Wait for client or context cancellation
	select {
	case client := <-a.clientChan:
		// Shutdown server
		a.server.Shutdown(context.Background())
		return client, nil
	case <-ctx.Done():
		a.server.Shutdown(context.Background())
		return nil, fmt.Errorf("authorization timeout")
	}
}

// completeAuth handles the OAuth callback
func (a *Auth) completeAuth(w http.ResponseWriter, r *http.Request) {
	token, err := a.authenticator.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Printf("Authentication error: %v", err)
		return
	}

	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
		return
	}

	// Create authenticated client
	client := spotify.New(a.authenticator.Client(r.Context(), token))

	fmt.Fprintf(w, "Authentication successful! You can close this window and return to the CLI.")
	a.clientChan <- client
}

// GetClient returns a Spotify client from an existing token
func (a *Auth) GetClient(token *oauth2.Token) *spotify.Client {
	return spotify.New(a.authenticator.Client(context.Background(), token))
}
