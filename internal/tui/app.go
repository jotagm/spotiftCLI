// Package tui implements the terminal user interface with Bubble Tea. It owns
// the screen, keyboard input, and the live playback view, driving the
// go-librespot daemon (playback) and the Spotify Web API (discovery).
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"cli_spotify/internal/player"
	"cli_spotify/internal/webapi"
)

// view identifies the active screen.
type view int

const (
	viewNowPlaying view = iota
	viewSearch
	viewLibrary
	viewPlaylist
)

// playback holds the live state of the current track, updated from WebSocket
// events and advanced by the per-second tick.
type playback struct {
	trackName string
	artists   string
	album     string
	duration  time.Duration
	progress  time.Duration
	isPlaying bool
	shuffle   bool
	repeat    string // "off", "context", "track"
	volume    int
	stopped   bool
}

// Model is the root Bubble Tea model.
type Model struct {
	pc     *player.Client
	web    *webapi.Client
	events *player.EventHandler

	view     view
	pb       playback
	search   searchState
	library  libraryState
	playlist playlistState
	width    int
	height   int
}

// New creates the root model, seeding playback state from an initial status
// snapshot (may be nil).
func New(pc *player.Client, web *webapi.Client, events *player.EventHandler, status *player.Status) Model {
	m := Model{
		pc:     pc,
		web:    web,
		events: events,
		view:   viewNowPlaying,
		pb:     playback{repeat: "off"},
		search: newSearchState(),
	}
	if status != nil {
		m.applyStatus(status)
	}
	return m
}

// Run starts the Bubble Tea program with an alternate screen.
func Run(m Model) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(listenEvents(m.events), tickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		if m.pb.isPlaying && !m.pb.stopped {
			m.pb.progress += time.Second
			if m.pb.progress > m.pb.duration {
				m.pb.progress = m.pb.duration
			}
		}
		return m, tickCmd()

	case playerEventMsg:
		if !msg.ok {
			return m, tea.Quit // event stream closed
		}
		m.applyEvent(msg.ev)
		return m, listenEvents(m.events)

	case searchResultsMsg:
		if msg.err != nil {
			m.search.status = "Search failed: " + msg.err.Error()
			return m, nil
		}
		m.search.results = msg.tracks
		m.search.cursor = 0
		if len(msg.tracks) == 0 {
			m.search.status = "No results."
		} else {
			m.search.status = ""
		}
		return m, nil

	case playResultMsg:
		if msg.err != nil {
			m.search.status = "Play failed: " + msg.err.Error()
			m.playlist.status = "Error: " + msg.err.Error()
		}
		return m, nil

	case playlistListMsg:
		m.library.loaded = true
		if msg.err != nil {
			m.library.status = "Failed: " + msg.err.Error()
		} else {
			m.library.albums = msg.albums
			m.library.playlists = msg.playlists
			m.library.status = ""
		}
		return m, nil

	case playlistTracksMsg:
		if msg.err != nil {
			// 403 on non-owned playlists in dev mode — play the playlist directly
			// as a context URI instead of showing individual tracks.
			if m.playlist.uri != "" {
				m.view = viewNowPlaying
				return m, playTrack(m.pc, m.playlist.uri, "", m.playlist.name)
			}
			m.playlist.status = "Error: " + msg.err.Error()
		} else {
			m.playlist.tracks = msg.tracks
			m.playlist.cursor = 0
			m.playlist.status = ""
		}
		return m, nil

	case tea.KeyMsg:
		switch m.view {
		case viewSearch:
			return m.handleSearchKey(msg)
		case viewLibrary:
			return m.handleLibraryKey(msg)
		case viewPlaylist:
			return m.handlePlaylistKey(msg)
		default:
			return m.handleKey(msg)
		}
	}

	return m, nil
}

func (m Model) View() string {
	switch m.view {
	case viewSearch:
		return m.searchView()
	case viewLibrary:
		return m.libraryView()
	case viewPlaylist:
		return m.playlistView()
	default:
		return m.nowPlayingView()
	}
}

// handleKey processes a key press in the now-playing view.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "/":
		return m.enterSearch()

	case "p":
		return m.enterLibrary()

	case " ":
		_ = m.pc.PlayPause()
		m.pb.isPlaying = !m.pb.isPlaying

	case "right", "l":
		_ = m.pc.Next()

	case "left", "h":
		_ = m.pc.Prev()

	case "up", "k":
		_ = m.pc.SetVolumeRelative(5)
		m.pb.volume = min(100, m.pb.volume+5)

	case "down", "j":
		_ = m.pc.SetVolumeRelative(-5)
		m.pb.volume = max(0, m.pb.volume-5)

	case "s":
		ns := !m.pb.shuffle
		_ = m.pc.SetShuffle(ns)
		m.pb.shuffle = ns

	case "r":
		m.cycleRepeat()
	}
	return m, nil
}

// cycleRepeat cycles through: off → context → track → off.
func (m *Model) cycleRepeat() {
	switch m.pb.repeat {
	case "off":
		_ = m.pc.SetRepeatContext(true)
		m.pb.repeat = "context"
	case "context":
		_ = m.pc.SetRepeatContext(false)
		_ = m.pc.SetRepeatTrack(true)
		m.pb.repeat = "track"
	default:
		_ = m.pc.SetRepeatTrack(false)
		m.pb.repeat = "off"
	}
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
