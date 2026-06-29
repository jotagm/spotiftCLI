package tui

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"cli_spotify/internal/display"
	"cli_spotify/internal/player"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51"))
	trackStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// nowPlayingView renders the now-playing screen.
func (m Model) nowPlayingView() string {
	if m.pb.stopped || m.pb.trackName == "" {
		var b strings.Builder
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("  ♪ NOW PLAYING") + "\n\n")
		b.WriteString("  No track currently playing.\n")
		b.WriteString(dimStyle.Render("  Press / to search or p to browse your library.") + "\n\n")
		b.WriteString(helpStyle.Render("  [/] search  [p] library  [q] quit") + "\n")
		return b.String()
	}

	bar := display.CreateProgressBar(m.pb.progress, m.pb.duration, 50)
	cur := display.FormatDuration(m.pb.progress)
	total := display.FormatDuration(m.pb.duration)

	statusIcon, statusText := "⏸", "Paused"
	if m.pb.isPlaying {
		statusIcon, statusText = "▶", "Playing"
	}

	shuffle := " "
	if m.pb.shuffle {
		shuffle = "🔀"
	}
	repeat := " "
	switch m.pb.repeat {
	case "track":
		repeat = "🔂"
	case "context":
		repeat = "🔁"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  ♪ NOW PLAYING") + "\n\n")
	b.WriteString(trackStyle.Render("  "+truncate(m.pb.trackName, 60)) + "\n")
	b.WriteString(dimStyle.Render("  "+truncate(m.pb.artists, 60)) + "\n")
	b.WriteString(dimStyle.Render("  "+truncate(m.pb.album, 60)) + "\n\n")
	b.WriteString("  " + dimStyle.Render(cur) + " " + greenStyle.Render(bar) + " " + dimStyle.Render(total) + "\n\n")
	b.WriteString("  " + greenStyle.Render(statusIcon+" "+statusText) +
		"   " + yellowStyle.Render(shuffle) +
		"   " + yellowStyle.Render(repeat) +
		"   " + dimStyle.Render("vol "+strconv.Itoa(m.pb.volume)+"%") + "\n\n")
	b.WriteString(helpStyle.Render("  [space] play/pause  [←→] prev/next  [↑↓] vol  [s] shuffle  [r] repeat  [/] search  [p] library  [q] quit") + "\n")
	return b.String()
}

// applyEvent updates playback state from a WebSocket event.
func (m *Model) applyEvent(ev player.Event) {
	switch ev.Type {
	case "metadata":
		var d player.EventMetadata
		if json.Unmarshal(ev.Data, &d) == nil {
			m.pb.trackName = d.Name
			m.pb.artists = strings.Join(d.ArtistNames, ", ")
			m.pb.album = d.AlbumName
			m.pb.duration = time.Duration(d.Duration) * time.Millisecond
			m.pb.progress = time.Duration(d.Position) * time.Millisecond
			m.pb.stopped = false
		}
	case "playing":
		m.pb.isPlaying = true
		m.pb.stopped = false
	case "paused":
		m.pb.isPlaying = false
	case "stopped":
		m.pb.isPlaying = false
		m.pb.stopped = true
	case "seek":
		var d player.EventSeek
		if json.Unmarshal(ev.Data, &d) == nil {
			m.pb.progress = time.Duration(d.Position) * time.Millisecond
			m.pb.duration = time.Duration(d.Duration) * time.Millisecond
		}
	case "volume":
		var d player.EventVolume
		if json.Unmarshal(ev.Data, &d) == nil {
			m.pb.volume = d.Value
		}
	case "shuffle_context":
		var d player.EventBool
		if json.Unmarshal(ev.Data, &d) == nil {
			m.pb.shuffle = d.Value
		}
	case "repeat_context":
		var d player.EventBool
		if json.Unmarshal(ev.Data, &d) == nil {
			if d.Value {
				m.pb.repeat = "context"
			} else if m.pb.repeat == "context" {
				m.pb.repeat = "off"
			}
		}
	case "repeat_track":
		var d player.EventBool
		if json.Unmarshal(ev.Data, &d) == nil {
			if d.Value {
				m.pb.repeat = "track"
			} else if m.pb.repeat == "track" {
				m.pb.repeat = "off"
			}
		}
	}
}

// applyStatus seeds playback state from the REST /status response.
func (m *Model) applyStatus(s *player.Status) {
	m.pb.isPlaying = !s.Paused && !s.Stopped
	m.pb.stopped = s.Stopped
	m.pb.shuffle = s.ShuffleContext
	m.pb.volume = s.Volume

	switch {
	case s.RepeatTrack:
		m.pb.repeat = "track"
	case s.RepeatContext:
		m.pb.repeat = "context"
	default:
		m.pb.repeat = "off"
	}

	if s.Track != nil {
		m.pb.trackName = s.Track.Name
		m.pb.artists = strings.Join(s.Track.ArtistNames, ", ")
		m.pb.album = s.Track.AlbumName
		m.pb.duration = time.Duration(s.Track.Duration) * time.Millisecond
	}
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}
