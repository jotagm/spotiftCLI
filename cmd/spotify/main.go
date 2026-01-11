package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cli_spotify/internal/auth"
	"cli_spotify/internal/client"
	"cli_spotify/internal/config"
	"cli_spotify/internal/display"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Validate configuration
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURI == "" {
		log.Fatal("Missing required environment variables. Please set SPOTIFY_CLIENT_ID, SPOTIFY_CLIENT_SECRET, and SPOTIFY_REDIRECT_URI")
	}

	// Create auth instance
	authClient := auth.NewAuth(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURI)

	// Required scopes for reading currently playing track
	scopes := []string{
		"user-read-currently-playing",
		"user-read-playback-state",
	}

	// Start OAuth flow
	fmt.Println("Starting Spotify authentication...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	token, err := authClient.StartAuthFlow(ctx, scopes)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	fmt.Println("\nAuthentication successful!")
	fmt.Println("Starting playback monitor...\n")

	// Create Spotify client
	spotifyClient := client.NewClient(token.AccessToken)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create ticker for refreshing track info
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var currentTrack *client.Track

	// Main loop
	for {
		select {
		case <-sigChan:
			fmt.Println("\n\nShutting down...")
			return

		case <-ticker.C:
			// Fetch current track
			track, err := spotifyClient.GetCurrentTrack()
			if err != nil {
				// If no track is playing, show a message
				if err.Error() == "no track currently playing" {
					fmt.Print("\033[2J\033[H")
					fmt.Println("\n  No track currently playing.")
					fmt.Println("  Start playing a song on Spotify to see it here!")
					fmt.Println("\n  Press Ctrl+C to exit")
					currentTrack = nil
					continue
				}
				log.Printf("Error fetching track: %v", err)
				continue
			}

			// Update progress for currently playing track
			if currentTrack != nil && track.Name == currentTrack.Name && track.IsPlaying {
				track.Progress = currentTrack.Progress + time.Second
			}
			currentTrack = track

			// Display the track
			display.DisplayCurrentTrack(*track)
		}
	}
}
