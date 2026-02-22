package display

import (
	"fmt"
	"strings"
	"time"
)

// Track represents a track for display purposes
type Track struct {
	Name      string
	Artist    string
	Album     string
	Duration  time.Duration
	Progress  time.Duration
	IsPlaying bool
	Shuffle   bool
	Repeat    string
	ImageURL  string
}

const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorCyan   = "\033[36m"
	ColorYellow = "\033[33m"
	ColorGray   = "\033[90m"
	ColorWhite  = "\033[97m"
	ColorBold   = "\033[1m"
)

// FormatDuration converts a duration to MM:SS format
func FormatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// CreateProgressBar creates a visual progress bar
func CreateProgressBar(current, total time.Duration, width int) string {
	if total == 0 {
		return strings.Repeat("─", width)
	}

	percentage := float64(current) / float64(total)
	filled := int(float64(width) * percentage)

	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("━", filled) + "●" + strings.Repeat("─", width-filled-1)
	return bar
}

// TruncateString truncates a string to maxLen and adds "..." if needed
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// DisplayCurrentTrack displays the currently playing track with a nice UI
func DisplayCurrentTrack(track Track) {
	// Clear screen and move cursor to top
	fmt.Print("\033[2J\033[H")

	fmt.Println()
	fmt.Println(ColorBold + ColorCyan + "  ♪ NOW PLAYING" + ColorReset)
	fmt.Println()

	// Song title
	songTitle := TruncateString(track.Name, 60)
	fmt.Printf(ColorBold+ColorWhite+"  %s"+ColorReset+"\n", songTitle)

	// Artist
	artistName := TruncateString(track.Artist, 60)
	fmt.Printf(ColorGray+"  %s"+ColorReset+"\n", artistName)

	// Album
	albumName := TruncateString(track.Album, 60)
	fmt.Printf(ColorGray+"  %s"+ColorReset+"\n", albumName)

	fmt.Println()

	// Progress bar
	progressBar := CreateProgressBar(track.Progress, track.Duration, 50)
	currentTime := FormatDuration(track.Progress)
	totalTime := FormatDuration(track.Duration)

	fmt.Printf("  %s %s %s %s\n",
		ColorGray+currentTime+ColorReset,
		ColorGreen+progressBar+ColorReset,
		ColorGray+totalTime+ColorReset,
		"",
	)

	fmt.Println()

	// Status icons
	var statusIcon string
	var statusText string
	if track.IsPlaying {
		statusIcon = "▶"
		statusText = "Playing"
	} else {
		statusIcon = "⏸"
		statusText = "Paused"
	}

	shuffleIcon := " "
	if track.Shuffle {
		shuffleIcon = "🔀"
	}

	repeatIcon := " "
	switch track.Repeat {
	case "track":
		repeatIcon = "🔂"
	case "context":
		repeatIcon = "🔁"
	}

	fmt.Printf("  %s%s %s%s   %s%s%s   %s%s%s\n",
		ColorGreen, statusIcon, statusText, ColorReset,
		ColorYellow, shuffleIcon, ColorReset,
		ColorYellow, repeatIcon, ColorReset,
	)

	fmt.Println()
	fmt.Println(ColorGray + "  [Space] play/pause  [←→] prev/next  [↑↓] volume  [s] shuffle  [r] repeat  [q] quit" + ColorReset)
	fmt.Println()
}
