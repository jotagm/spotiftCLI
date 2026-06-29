package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds application configuration.
type Config struct {
	// go-librespot daemon settings
	DeviceName string
	DaemonPort int

	// LibrespotPath is an optional path to a pre-installed go-librespot binary.
	// When set, the daemon uses it instead of downloading one. This is the only
	// way to run on platforms without an official go-librespot release (Windows,
	// macOS) — build go-librespot yourself and point this at it.
	LibrespotPath string

	// Spotify Web API (used for search and library/playlist browsing). Auth uses
	// the Authorization Code flow with PKCE, so only the Client ID is required —
	// the Client Secret is not used.
	ClientID    string
	RedirectURI string
}

// Load reads configuration from the .env file or system environment variables.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	port := 3678
	if v := os.Getenv("SPOTIFY_DAEMON_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			port = p
		}
	}

	deviceName := os.Getenv("SPOTIFY_DEVICE_NAME")
	if deviceName == "" {
		deviceName = "Spotify CLI"
	}

	redirectURI := os.Getenv("SPOTIFY_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://127.0.0.1:8080/callback"
	}

	return &Config{
		DeviceName:    deviceName,
		DaemonPort:    port,
		LibrespotPath: os.Getenv("SPOTIFY_LIBRESPOT_PATH"),
		ClientID:      os.Getenv("SPOTIFY_CLIENT_ID"),
		RedirectURI:   redirectURI,
	}
}
