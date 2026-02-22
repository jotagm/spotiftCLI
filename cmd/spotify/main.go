package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cli_spotify/internal/config"
	"cli_spotify/internal/daemon"
	"cli_spotify/internal/display"
	"cli_spotify/internal/player"

	"golang.org/x/term"
)

// appState holds the live playback state updated from WebSocket events.
type appState struct {
	trackName   string
	artistNames string
	albumName   string
	duration    time.Duration
	progress    time.Duration
	isPlaying   bool
	shuffle     bool
	repeat      string // "off", "context", "track"
	volume      int
	stopped     bool
}

func main() {
	cfg := config.Load()

	// Start go-librespot daemon
	mgr := daemon.NewManager(cfg)
	if err := mgr.Start(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[✗] Failed to start daemon: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Stop()

	// Create HTTP client for player controls
	pc := player.NewClient(cfg.DaemonPort)

	// Connect to WebSocket event stream
	events, err := player.NewEventHandler(cfg.DaemonPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[✗] Failed to connect to event stream: %v\n", err)
		os.Exit(1)
	}
	defer events.Close()
	events.Start()

	// Seed initial state from /status
	state := &appState{repeat: "off"}
	if status, err := pc.Status(); err == nil {
		applyStatus(state, status)
	}

	// Put terminal in raw mode for keyboard input
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[✗] Could not set raw terminal: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(fd, oldState)

	// Channel for key presses
	keysCh := make(chan []byte, 16)
	go readKeys(keysCh)

	// Graceful shutdown on SIGINT/SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Ticker to advance local progress every second
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	render(state)

	for {
		select {
		case <-sigChan:
			term.Restore(fd, oldState)
			fmt.Println("\r\nShutting down...")
			return

		case ev, ok := <-events.Ch:
			if !ok {
				return
			}
			applyEvent(state, ev)
			render(state)

		case <-ticker.C:
			if state.isPlaying && !state.stopped {
				state.progress += time.Second
				if state.progress > state.duration {
					state.progress = state.duration
				}
			}
			render(state)

		case key := <-keysCh:
			if handleKey(key, state, pc, fd, oldState) {
				return
			}
			render(state)
		}
	}
}

// handleKey processes a raw key sequence. Returns true if the user wants to quit.
func handleKey(key []byte, state *appState, pc *player.Client, fd int, oldState *term.State) bool {
	switch {
	case len(key) == 1 && key[0] == 'q',
		len(key) == 1 && key[0] == 3: // Ctrl+C
		term.Restore(fd, oldState)
		fmt.Println("\r\nShutting down...")
		return true

	case len(key) == 1 && key[0] == ' ':
		_ = pc.PlayPause()
		state.isPlaying = !state.isPlaying

	case len(key) == 1 && key[0] == 'l',
		len(key) == 3 && key[0] == 0x1b && key[1] == '[' && key[2] == 'C': // →
		_ = pc.Next()

	case len(key) == 1 && key[0] == 'h',
		len(key) == 3 && key[0] == 0x1b && key[1] == '[' && key[2] == 'D': // ←
		_ = pc.Prev()

	case len(key) == 1 && key[0] == 'k',
		len(key) == 3 && key[0] == 0x1b && key[1] == '[' && key[2] == 'A': // ↑
		_ = pc.SetVolumeRelative(5)
		state.volume = min(100, state.volume+5)

	case len(key) == 1 && key[0] == 'j',
		len(key) == 3 && key[0] == 0x1b && key[1] == '[' && key[2] == 'B': // ↓
		_ = pc.SetVolumeRelative(-5)
		state.volume = max(0, state.volume-5)

	case len(key) == 1 && key[0] == 's':
		newShuffle := !state.shuffle
		_ = pc.SetShuffle(newShuffle)
		state.shuffle = newShuffle

	case len(key) == 1 && key[0] == 'r':
		cycleRepeat(state, pc)
	}
	return false
}

// cycleRepeat cycles through: off → context → track → off
func cycleRepeat(state *appState, pc *player.Client) {
	switch state.repeat {
	case "off":
		_ = pc.SetRepeatContext(true)
		state.repeat = "context"
	case "context":
		_ = pc.SetRepeatContext(false)
		_ = pc.SetRepeatTrack(true)
		state.repeat = "track"
	default:
		_ = pc.SetRepeatTrack(false)
		state.repeat = "off"
	}
}

// applyEvent updates appState from a WebSocket event.
func applyEvent(state *appState, ev player.Event) {
	switch ev.Type {
	case "metadata":
		var d player.EventMetadata
		if err := json.Unmarshal(ev.Data, &d); err == nil {
			state.trackName = d.Name
			state.artistNames = joinStrings(d.ArtistNames)
			state.albumName = d.AlbumName
			state.duration = time.Duration(d.Duration) * time.Millisecond
			state.progress = time.Duration(d.Position) * time.Millisecond
			state.stopped = false
		}
	case "playing":
		state.isPlaying = true
		state.stopped = false
	case "paused":
		state.isPlaying = false
	case "stopped":
		state.isPlaying = false
		state.stopped = true
	case "seek":
		var d player.EventSeek
		if err := json.Unmarshal(ev.Data, &d); err == nil {
			state.progress = time.Duration(d.Position) * time.Millisecond
			state.duration = time.Duration(d.Duration) * time.Millisecond
		}
	case "volume":
		var d player.EventVolume
		if err := json.Unmarshal(ev.Data, &d); err == nil {
			state.volume = d.Value
		}
	case "shuffle_context":
		var d player.EventBool
		if err := json.Unmarshal(ev.Data, &d); err == nil {
			state.shuffle = d.Value
		}
	case "repeat_context":
		var d player.EventBool
		if err := json.Unmarshal(ev.Data, &d); err == nil {
			if d.Value {
				state.repeat = "context"
			} else if state.repeat == "context" {
				state.repeat = "off"
			}
		}
	case "repeat_track":
		var d player.EventBool
		if err := json.Unmarshal(ev.Data, &d); err == nil {
			if d.Value {
				state.repeat = "track"
			} else if state.repeat == "track" {
				state.repeat = "off"
			}
		}
	}
}

// applyStatus seeds appState from the REST /status response.
func applyStatus(state *appState, s *player.Status) {
	state.isPlaying = !s.Paused && !s.Stopped
	state.stopped = s.Stopped
	state.shuffle = s.ShuffleContext
	state.volume = s.Volume

	switch {
	case s.RepeatTrack:
		state.repeat = "track"
	case s.RepeatContext:
		state.repeat = "context"
	default:
		state.repeat = "off"
	}

	if s.Track != nil {
		state.trackName = s.Track.Name
		state.artistNames = joinStrings(s.Track.ArtistNames)
		state.albumName = s.Track.AlbumName
		state.duration = time.Duration(s.Track.Duration) * time.Millisecond
	}
}

// render draws the current state to the terminal.
func render(state *appState) {
	if state.stopped || state.trackName == "" {
		fmt.Print("\033[2J\033[H")
		fmt.Println()
		fmt.Println("  No track currently playing.")
		fmt.Println("  Select \"Spotify CLI\" as the device in Spotify to start playing here.")
		fmt.Println()
		fmt.Println(display.ColorGray + "  [q] quit" + display.ColorReset)
		return
	}

	repeatMode := "off"
	switch state.repeat {
	case "context":
		repeatMode = "context"
	case "track":
		repeatMode = "track"
	}

	t := display.Track{
		Name:      state.trackName,
		Artist:    state.artistNames,
		Album:     state.albumName,
		Duration:  state.duration,
		Progress:  state.progress,
		IsPlaying: state.isPlaying,
		Shuffle:   state.shuffle,
		Repeat:    repeatMode,
	}
	display.DisplayCurrentTrack(t)
}

// readKeys reads raw key bytes from stdin and sends them to ch.
// Arrow keys produce 3-byte escape sequences (ESC [ X).
func readKeys(ch chan<- []byte) {
	buf := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}
		key := make([]byte, n)
		copy(key, buf[:n])
		ch <- key
	}
}

// joinStrings joins a slice of strings with ", ".
func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += ", " + s
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
