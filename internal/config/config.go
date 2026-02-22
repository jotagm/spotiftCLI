package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Load reads configuration from environment variables
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	return &Config{
		ClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		ClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("SPOTIFY_REDIRECT_URI"),
	}
}
