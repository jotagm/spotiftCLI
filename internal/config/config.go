package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds application configuration.
type Config struct {
	// Legacy Spotify Web API fields (kept for .env compatibility)
	ClientID     string
	ClientSecret string
	RedirectURI  string

	// go-librespot daemon settings
	DeviceName string
	DaemonPort int
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

	return &Config{
		ClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		ClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("SPOTIFY_REDIRECT_URI"),
		DeviceName:   deviceName,
		DaemonPort:   port,
	}
}
