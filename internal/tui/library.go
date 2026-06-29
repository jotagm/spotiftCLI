package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cli_spotify/internal/webapi"
)

// libraryState holds the library view state (albums + playlists).
type libraryState struct {
	albums    []webapi.Album
	playlists []webapi.Playlist
	cursor    int
	loaded    bool
	status    string
}

// libraryTotal returns the total number of entries (Liked Songs + albums + playlists).
func (l *libraryState) total() int {
	return 1 + len(l.albums) + len(l.playlists)
}

// playlistState holds the open-playlist (track list) view state.
type playlistState struct {
	name    string
	uri     string // empty for Liked Songs
	isLiked bool
	tracks  []webapi.Track
	cursor  int
	status  string
}

// enterLibrary switches to the library view, triggering a playlist load on
// first visit.
func (m Model) enterLibrary() (Model, tea.Cmd) {
	m.view = viewLibrary
	if !m.library.loaded {
		m.library.status = "Loading playlists..."
		return m, loadPlaylists(m.web)
	}
	return m, nil
}

// handleLibraryKey processes key events in the library view.
func (m Model) handleLibraryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.view = viewNowPlaying
		return m, nil
	case "up", "k":
		if m.library.cursor > 0 {
			m.library.cursor--
		}
	case "down", "j":
		if m.library.cursor < m.library.total()-1 {
			m.library.cursor++
		}
	case "enter":
		return m.openLibraryEntry()
	}
	return m, nil
}

// openLibraryEntry opens the highlighted library entry.
// Liked Songs and Albums show individual tracks.
// Playlists try to load tracks; on 403 (dev mode restriction for non-owned
// playlists) the playlistTracksMsg handler falls back to direct play.
func (m Model) openLibraryEntry() (Model, tea.Cmd) {
	if m.library.cursor == 0 {
		m.playlist = playlistState{name: "♥ Liked Songs", isLiked: true, status: "Loading..."}
		m.view = viewPlaylist
		return m, loadLikedSongs(m.web)
	}
	i := m.library.cursor - 1
	if i < len(m.library.albums) {
		a := m.library.albums[i]
		m.playlist = playlistState{name: a.Name, uri: a.URI, status: "Loading..."}
		m.view = viewPlaylist
		return m, loadAlbumTracks(m.web, a.ID)
	}
	pl := m.library.playlists[i-len(m.library.albums)]
	m.playlist = playlistState{name: pl.Name, uri: pl.URI, status: "Loading..."}
	m.view = viewPlaylist
	return m, loadPlaylistTracks(m.web, pl.URI)
}

// handlePlaylistKey processes key events in the open-playlist (track list) view.
func (m Model) handlePlaylistKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.view = viewLibrary
		return m, nil
	case "up", "k":
		if m.playlist.cursor > 0 {
			m.playlist.cursor--
		}
	case "down", "j":
		if m.playlist.cursor < len(m.playlist.tracks)-1 {
			m.playlist.cursor++
		}
	case "enter":
		if t := m.selectedPlaylistTrack(); t != nil {
			m.playlist.status = "Playing: " + t.Name
			return m, playTrack(m.pc, "", t.URI, t.Name)
		}
	}
	return m, nil
}

// selectedPlaylistTrack returns the highlighted track in the open playlist, or nil.
func (m Model) selectedPlaylistTrack() *webapi.Track {
	i := m.playlist.cursor
	if i < 0 || i >= len(m.playlist.tracks) {
		return nil
	}
	return &m.playlist.tracks[i]
}

// libraryView renders the library screen (Liked Songs, Albums, Playlists).
func (m Model) libraryView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  ♪ LIBRARY") + "\n\n")

	if m.library.status != "" {
		b.WriteString(dimStyle.Render("  "+m.library.status) + "\n\n")
		b.WriteString(helpStyle.Render("  [esc] back") + "\n")
		return b.String()
	}

	total := m.library.total()
	visible := m.height - 8
	if visible < 3 {
		visible = 3
	}
	start := 0
	if m.library.cursor >= visible {
		start = m.library.cursor - visible + 1
	}
	end := start + visible
	if end > total {
		end = total
	}

	for i := start; i < end; i++ {
		var line string
		switch {
		case i == 0:
			line = "♥  Liked Songs"
		case i-1 < len(m.library.albums):
			a := m.library.albums[i-1]
			line = "♫  " + truncate(a.Name, 38) + dimSep + truncate(a.ArtistNames(), 20)
		default:
			pl := m.library.playlists[i-1-len(m.library.albums)]
			line = "≡  " + truncate(pl.Name, 55)
		}
		if i == m.library.cursor {
			b.WriteString(greenStyle.Render("  ▸ "+line) + "\n")
		} else {
			b.WriteString("    " + dimStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [↑↓] move  [enter] open  [esc] back  (♥ liked  ♫ album  ≡ playlist)") + "\n")
	return b.String()
}

// playlistView renders the open-playlist (track list) screen.
func (m Model) playlistView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  ♪ "+truncate(m.playlist.name, 55)) + "\n\n")

	if len(m.playlist.tracks) == 0 {
		msg := m.playlist.status
		if msg == "" {
			msg = "No tracks."
		}
		b.WriteString(dimStyle.Render("  "+msg) + "\n\n")
		b.WriteString(helpStyle.Render("  [esc] back") + "\n")
		return b.String()
	}

	if m.playlist.status != "" {
		b.WriteString(dimStyle.Render("  "+m.playlist.status) + "\n\n")
	}

	visible := m.height - 9
	if visible < 3 {
		visible = 3
	}
	cur := m.playlist.cursor
	start := 0
	if cur >= visible {
		start = cur - visible + 1
	}
	end := start + visible
	if end > len(m.playlist.tracks) {
		end = len(m.playlist.tracks)
	}

	for i := start; i < end; i++ {
		t := m.playlist.tracks[i]
		line := truncate(t.Name, 45) + dimSep + truncate(t.ArtistNames(), 30)
		if i == cur {
			b.WriteString(greenStyle.Render("  ▸ "+line) + "\n")
		} else {
			b.WriteString("    " + dimStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [↑↓] move  [enter] play  [esc] back") + "\n")
	return b.String()
}
