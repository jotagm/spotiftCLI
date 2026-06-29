package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"cli_spotify/internal/player"
	"cli_spotify/internal/webapi"
)

// tickMsg fires once per second to advance the local progress bar between
// WebSocket events.
type tickMsg time.Time

// playerEventMsg carries a go-librespot WebSocket event into the Bubble Tea
// update loop. ok is false when the event stream has closed.
type playerEventMsg struct {
	ev player.Event
	ok bool
}

// listenEvents returns a command that blocks until the next daemon event and
// delivers it as a playerEventMsg. It is re-issued after each event to keep
// reading the stream.
func listenEvents(events *player.EventHandler) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events.Ch
		return playerEventMsg{ev: ev, ok: ok}
	}
}

// tickCmd schedules the next one-second tick.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// searchResultsMsg carries the outcome of a track search.
type searchResultsMsg struct {
	tracks []webapi.Track
	err    error
}

// playResultMsg carries the outcome of a play request.
type playResultMsg struct {
	track string
	err   error
}

// playlistListMsg carries the user's albums and playlists from the Web API.
type playlistListMsg struct {
	albums    []webapi.Album
	playlists []webapi.Playlist
	err       error
}

// playlistTracksMsg carries the tracks for an open playlist or Liked Songs.
type playlistTracksMsg struct {
	tracks []webapi.Track
	err    error
}

// doSearch runs a track search on the Web API.
func doSearch(web *webapi.Client, query string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := web.SearchTracks(query, 25)
		return searchResultsMsg{tracks: tracks, err: err}
	}
}

// playTrack starts playback on the daemon.
// contextURI is the playlist/album URI (empty to play trackURI directly).
// trackURI is the specific track to play.
func playTrack(pc *player.Client, contextURI, trackURI, name string) tea.Cmd {
	return func() tea.Msg {
		uri, skip := contextURI, trackURI
		if uri == "" {
			uri, skip = trackURI, ""
		}
		return playResultMsg{track: name, err: pc.Play(uri, skip, false)}
	}
}

// loadPlaylists fetches the user's saved albums and playlists from the Web API.
func loadPlaylists(web *webapi.Client) tea.Cmd {
	return func() tea.Msg {
		albums, err := web.SavedAlbums()
		if err != nil {
			return playlistListMsg{err: err}
		}
		playlists, err := web.UserPlaylists()
		return playlistListMsg{albums: albums, playlists: playlists, err: err}
	}
}

// loadAlbumTracks fetches the tracks for a given album ID.
func loadAlbumTracks(web *webapi.Client, albumID string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := web.AlbumTracks(albumID)
		return playlistTracksMsg{tracks: tracks, err: err}
	}
}

// loadPlaylistTracks fetches the tracks for a given playlist URI.
func loadPlaylistTracks(web *webapi.Client, playlistURI string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := web.PlaylistTracks(playlistURI)
		return playlistTracksMsg{tracks: tracks, err: err}
	}
}

// loadLikedSongs fetches the user's saved (Liked Songs) tracks.
func loadLikedSongs(web *webapi.Client) tea.Cmd {
	return func() tea.Msg {
		tracks, err := web.SavedTracks()
		return playlistTracksMsg{tracks: tracks, err: err}
	}
}

