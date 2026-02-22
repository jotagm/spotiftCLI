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
	"cli_spotify/internal/config"
	"cli_spotify/internal/devices"
	"cli_spotify/internal/display"
	"cli_spotify/internal/playback"
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

	// Start OAuth flow
	fmt.Println("Starting Spotify authentication...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := authClient.StartAuthFlow(ctx)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	fmt.Println("\nAuthentication successful!")

	// Create device manager and playback controller
	dm := devices.NewDeviceManager(client)
	pc := playback.NewPlaybackController(client)

	// Get available devices
	fmt.Println("\nChecking available devices...")
	deviceList, err := dm.GetDevices()
	if err != nil {
		log.Printf("Warning: Could not get devices: %v", err)
	} else {
		devices.DisplayDevices(deviceList)
	}

	// Ensure active device
	device, err := dm.EnsureActiveDevice()
	if err != nil {
		log.Printf("Warning: No active device found. Please start playing something on Spotify.")
	} else {
		fmt.Printf("\nActive device: %s\n", device.Name)
	}

	fmt.Println("\nStarting playback monitor...\n")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create ticker for refreshing track info
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastTrackID string
	var localProgress time.Duration

	// Main loop
	for {
		select {
		case <-sigChan:
			fmt.Println("\n\nShutting down...")
			return

		case <-ticker.C:
			// Fetch current playback state
			state, err := pc.GetCurrentPlayback()
			if err != nil {
				// If no track is playing, show a message
				if err.Error() == "no track currently playing" {
					fmt.Print("\033[2J\033[H")
					fmt.Println("\n  No track currently playing.")
					fmt.Println("  Start playing a song on Spotify to see it here!")
					fmt.Println("\n  Press Ctrl+C to exit")
					lastTrackID = ""
					continue
				}
				log.Printf("Error fetching playback state: %v", err)
				continue
			}

			// Check if track changed
			if state.Track.ID != lastTrackID {
				lastTrackID = state.Track.ID
				localProgress = state.Progress
			} else if state.IsPlaying {
				// Update local progress smoothly
				localProgress += time.Second
				if localProgress > state.Track.Duration {
					localProgress = state.Track.Duration
				}
			}

			// Convert playback state to display track
			track := display.Track{
				Name:      state.Track.Name,
				Artist:    state.Track.Artist,
				Album:     state.Track.Album,
				Duration:  state.Track.Duration,
				Progress:  localProgress,
				IsPlaying: state.IsPlaying,
				Shuffle:   state.ShuffleState,
				Repeat:    state.RepeatState,
				ImageURL:  state.Track.ImageURL,
			}

			// Display the track
			display.DisplayCurrentTrack(track)
		}
	}
}
