// Package display provides small formatting helpers for the playback UI.
package display

import (
	"fmt"
	"strings"
	"time"
)

// FormatDuration converts a duration to MM:SS format.
func FormatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// CreateProgressBar creates a visual progress bar of the given width.
func CreateProgressBar(current, total time.Duration, width int) string {
	if total == 0 {
		return strings.Repeat("─", width)
	}

	filled := int(float64(width) * float64(current) / float64(total))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	return strings.Repeat("━", filled) + "●" + strings.Repeat("─", width-filled-1)
}
