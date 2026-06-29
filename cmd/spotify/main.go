package main

import (
	"fmt"
	"os"
	"path/filepath"

	"cli_spotify/internal/config"
	"cli_spotify/internal/daemon"
	"cli_spotify/internal/player"
	"cli_spotify/internal/tui"
	"cli_spotify/internal/webapi"
)

func main() {
	cfg := config.Load()

	// Start the go-librespot daemon (handles audio playback).
	mgr := daemon.NewManager(cfg)
	if err := mgr.Start(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[✗] Failed to start daemon: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Stop()

	// Authenticate with the Spotify Web API (search and library browsing).
	web, err := newWebClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[✗] Spotify Web API login failed: %v\n", err)
		os.Exit(1)
	}
	if u, err := web.CurrentUser(); err == nil {
		fmt.Printf("[✓] Web API authenticated as %s (%s).\n", u.DisplayName, u.Product)
	}

	// HTTP client for player controls and WebSocket event stream.
	pc := player.NewClient(cfg.DaemonPort)

	events, err := player.NewEventHandler(cfg.DaemonPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[✗] Failed to connect to event stream: %v\n", err)
		os.Exit(1)
	}
	defer events.Close()
	events.Start()

	// Seed the UI with the current playback status.
	var status *player.Status
	if s, err := pc.Status(); err == nil {
		status = s
	}

	if err := tui.Run(tui.New(pc, web, events, status)); err != nil {
		fmt.Fprintf(os.Stderr, "[✗] UI error: %v\n", err)
		os.Exit(1)
	}
}

// newWebClient builds an authenticated Spotify Web API client, running the
// interactive login on first use and reusing the saved token afterwards.
func newWebClient(cfg *config.Config) (*webapi.Client, error) {
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("SPOTIFY_CLIENT_ID not set (add it to .env)")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	tokenPath := filepath.Join(home, ".spotify-cli", "webapi-token.json")
	auth := webapi.NewAuthenticator(cfg.ClientID, cfg.RedirectURI, tokenPath)
	return webapi.NewClient(auth)
}
