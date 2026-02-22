package playback

import (
	"context"
	"fmt"
	"time"

	"github.com/zmb3/spotify/v2"
)

type PlaybackController struct {
	client *spotify.Client
	ctx    context.Context
}

type PlaybackState struct {
	IsPlaying     bool
	ShuffleState  bool
	RepeatState   string
	Track         *TrackInfo
	Progress      time.Duration
	Device        *DeviceInfo
	VolumePercent int
}

type TrackInfo struct {
	ID       string
	Name     string
	Artist   string
	Album    string
	Duration time.Duration
	ImageURL string
}

type DeviceInfo struct {
	ID   string
	Name string
	Type string
}

func NewPlaybackController(client *spotify.Client) *PlaybackController {
	return &PlaybackController{
		client: client,
		ctx:    context.Background(),
	}
}

// GetCurrentPlayback returns the current playback state
func (pc *PlaybackController) GetCurrentPlayback() (*PlaybackState, error) {
	state, err := pc.client.PlayerState(pc.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get playback state: %w", err)
	}

	if state == nil || state.Item == nil {
		return nil, fmt.Errorf("no track currently playing")
	}

	playbackState := &PlaybackState{
		IsPlaying:     state.Playing,
		ShuffleState:  state.ShuffleState,
		RepeatState:   string(state.RepeatState),
		Progress:      time.Duration(state.Progress) * time.Millisecond,
		VolumePercent: int(state.Device.Volume),
	}

	// Track info
	track := state.Item
	playbackState.Track = &TrackInfo{
		ID:       track.ID.String(),
		Name:     track.Name,
		Duration: time.Duration(track.Duration) * time.Millisecond,
	}

	// Artist info
	if len(track.Artists) > 0 {
		playbackState.Track.Artist = track.Artists[0].Name
		if len(track.Artists) > 1 {
			playbackState.Track.Artist += " & " + track.Artists[1].Name
		}
	}

	// Album info
	playbackState.Track.Album = track.Album.Name
	if len(track.Album.Images) > 0 {
		playbackState.Track.ImageURL = track.Album.Images[0].URL
	}

	// Device info
	playbackState.Device = &DeviceInfo{
		ID:   state.Device.ID.String(),
		Name: state.Device.Name,
		Type: state.Device.Type,
	}

	return playbackState, nil
}

// Play resumes playback
func (pc *PlaybackController) Play() error {
	err := pc.client.Play(pc.ctx)
	if err != nil {
		return fmt.Errorf("failed to play: %w", err)
	}
	return nil
}

// Pause pauses playback
func (pc *PlaybackController) Pause() error {
	err := pc.client.Pause(pc.ctx)
	if err != nil {
		return fmt.Errorf("failed to pause: %w", err)
	}
	return nil
}

// Next skips to next track
func (pc *PlaybackController) Next() error {
	err := pc.client.Next(pc.ctx)
	if err != nil {
		return fmt.Errorf("failed to skip to next track: %w", err)
	}
	return nil
}

// Previous skips to previous track
func (pc *PlaybackController) Previous() error {
	err := pc.client.Previous(pc.ctx)
	if err != nil {
		return fmt.Errorf("failed to skip to previous track: %w", err)
	}
	return nil
}

// SetVolume sets the volume (0-100)
func (pc *PlaybackController) SetVolume(volume int) error {
	if volume < 0 || volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100")
	}

	err := pc.client.Volume(pc.ctx, volume)
	if err != nil {
		return fmt.Errorf("failed to set volume: %w", err)
	}
	return nil
}

// SetShuffle enables or disables shuffle
func (pc *PlaybackController) SetShuffle(state bool) error {
	err := pc.client.Shuffle(pc.ctx, state)
	if err != nil {
		return fmt.Errorf("failed to set shuffle: %w", err)
	}
	return nil
}

// SetRepeat sets repeat mode: "off", "track", "context"
func (pc *PlaybackController) SetRepeat(state string) error {
	err := pc.client.Repeat(pc.ctx, state)
	if err != nil {
		return fmt.Errorf("failed to set repeat: %w", err)
	}
	return nil
}

// PlayTrack plays a specific track
func (pc *PlaybackController) PlayTrack(trackID string, deviceID string) error {
	opts := &spotify.PlayOptions{
		URIs: []spotify.URI{spotify.URI(trackID)},
	}

	if deviceID != "" {
		opts.DeviceID = (*spotify.ID)(&deviceID)
	}

	err := pc.client.PlayOpt(pc.ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to play track: %w", err)
	}
	return nil
}

// PlayAlbum plays a specific album
func (pc *PlaybackController) PlayAlbum(albumID string, deviceID string) error {
	opts := &spotify.PlayOptions{
		PlaybackContext: (*spotify.URI)(&albumID),
	}

	if deviceID != "" {
		opts.DeviceID = (*spotify.ID)(&deviceID)
	}

	err := pc.client.PlayOpt(pc.ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to play album: %w", err)
	}
	return nil
}

// PlayPlaylist plays a specific playlist
func (pc *PlaybackController) PlayPlaylist(playlistID string, deviceID string) error {
	opts := &spotify.PlayOptions{
		PlaybackContext: (*spotify.URI)(&playlistID),
	}

	if deviceID != "" {
		opts.DeviceID = (*spotify.ID)(&deviceID)
	}

	err := pc.client.PlayOpt(pc.ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to play playlist: %w", err)
	}
	return nil
}

// Seek seeks to a position in the current track
func (pc *PlaybackController) Seek(position time.Duration) error {
	positionMs := int(position.Milliseconds())
	err := pc.client.Seek(pc.ctx, positionMs)
	if err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}
	return nil
}

// TogglePlayPause toggles between play and pause
func (pc *PlaybackController) TogglePlayPause() error {
	state, err := pc.GetCurrentPlayback()
	if err != nil {
		return err
	}

	if state.IsPlaying {
		return pc.Pause()
	}
	return pc.Play()
}
