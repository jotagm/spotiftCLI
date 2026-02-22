# CLI Spotify

A beautiful command-line interface for Spotify that displays your currently playing track with real-time updates.

## Features

- OAuth authentication with Spotify
- Real-time display of currently playing track
- Shows track name, artist, album
- Visual progress bar
- Playback status (playing/paused)
- Shuffle and repeat indicators
- Auto-refreshing display

## Project Structure

```
cli_spotify/
├── cmd/
│   └── spotify/
│       └── main.go          # Application entry point
├── internal/
│   ├── auth/
│   │   └── auth.go          # OAuth authentication logic
│   ├── client/
│   │   ├── spotify.go       # Spotify API client
│   │   └── types.go         # Data structures
│   ├── config/
│   │   └── config.go        # Configuration management
│   └── display/
│       └── display.go       # Terminal UI rendering
├── go.mod
├── .env.example
└── README.md
```

## Setup

1. Get your Spotify API credentials:
   - Go to [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
   - Create a new app
   - Add `http://localhost:8888/callback` to your app's Redirect URIs
   - Copy your Client ID and Client Secret

2. Configure environment variables:
   ```bash
   cp .env.example .env
   ```

   Edit `.env` with your credentials:
   ```
   SPOTIFY_CLIENT_ID=your_client_id_here
   SPOTIFY_CLIENT_SECRET=your_client_secret_here
   SPOTIFY_REDIRECT_URI=http://localhost:8888/callback
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Run the application:
   ```bash
   go run cmd/spotify/main.go
   ```

5. The app will open your browser for authentication. After authorizing, return to the terminal to see your currently playing track!

## Build

Build a binary:
```bash
go build -o spotify cmd/spotify/main.go
```

Then run it:
```bash
./spotify
```

## Requirements

- Go 1.25 or higher
- Active Spotify Premium or Free account
- Internet connection

## Example Usage

```go
// Create managers
dm := devices.NewDeviceManager(client)
pc := playback.NewPlaybackController(client)

// Ensure active device
device, _ := dm.EnsureActiveDevice()

// Control playback
pc.Play()
pc.Next()
pc.SetVolume(70)
pc.SetShuffle(true)

// Play specific track
pc.PlayTrack("spotify:track:...", device.ID)
```

#   s p o t i f y - C L I 
 
 #   s p o t i f t C L I 
 
 #   s p o t i f t C L I 
 
 
