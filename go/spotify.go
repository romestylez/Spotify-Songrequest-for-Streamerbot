package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const spotifyAPIBase = "https://api.spotify.com/v1"

type TrackInfo struct {
	ID       string
	Title    string
	Artists  string
	Duration int64
	Progress int64
}

type PlaylistItem struct {
	URI      string
	TrackID  string
	Position int
}

type PlaybackState struct {
	IsPlaying  bool
	TrackID    string
	Duration   int64
	Progress   int64
	Index      int
	SnapshotID string
}

type DeleteTrack struct {
	URI       string
	Positions []int
}

var (
	trackURLRegex = regexp.MustCompile(`(?:spotify:track:|open\.spotify\.com/(?:[a-z-]+/)*track/)([A-Za-z0-9]{22})`)
	albumURLRegex = regexp.MustCompile(`open\.spotify\.com/(?:[a-z-]+/)*album/`)
)

func ExtractTrackURI(input string) (string, error) {
	if albumURLRegex.MatchString(input) {
		return "", fmt.Errorf("Das ist ein Album-Link, bitte einen einzelnen Song-Link senden")
	}
	m := trackURLRegex.FindStringSubmatch(input)
	if m == nil {
		return "", fmt.Errorf("Kein gültiger Spotify-Track-Link gefunden")
	}
	return "spotify:track:" + m[1], nil
}

func (c *AppClient) spotifyRequest(method, endpoint string, body interface{}) ([]byte, int, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return nil, 0, err
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, spotifyAPIBase+endpoint, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode == 401 {
		c.InvalidateToken()
		return nil, resp.StatusCode, fmt.Errorf("unauthorized, token invalidated")
	}

	return respBody, resp.StatusCode, nil
}

func (c *AppClient) GetTrackInfo(trackID string) (*TrackInfo, error) {
	data, status, err := c.spotifyRequest("GET", "/tracks/"+trackID, nil)
	if err != nil {
		return nil, err
	}
	if status == 400 || status == 404 {
		return nil, fmt.Errorf("Song nicht gefunden – ungültiger oder nicht existierender Track-Link")
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("Spotify-Fehler (Status %d)", status)
	}

	var track struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		DurationMs int64 `json:"duration_ms"`
	}
	if err := json.Unmarshal(data, &track); err != nil {
		return nil, err
	}
	if track.ID == "" {
		return nil, fmt.Errorf("Song nicht gefunden")
	}

	artists := make([]string, len(track.Artists))
	for i, a := range track.Artists {
		artists[i] = a.Name
	}
	return &TrackInfo{
		ID:       track.ID,
		Title:    track.Name,
		Artists:  strings.Join(artists, ", "),
		Duration: track.DurationMs,
	}, nil
}

func (c *AppClient) AddTrack(playlistID, trackURI string) error {
	payload := map[string]interface{}{
		"uris": []string{trackURI},
	}
	_, status, err := c.spotifyRequest("POST", "/playlists/"+playlistID+"/tracks", payload)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("spotify returned status %d", status)
	}
	return nil
}

func (c *AppClient) GetCurrentTrack() (*TrackInfo, error) {
	data, status, err := c.spotifyRequest("GET", "/me/player", nil)
	if err != nil {
		return nil, err
	}
	if status == 204 || len(data) == 0 {
		return nil, nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("spotify returned status %d", status)
	}

	var player struct {
		Item struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			DurationMs int64 `json:"duration_ms"`
		} `json:"item"`
		ProgressMs int64 `json:"progress_ms"`
		IsPlaying  bool  `json:"is_playing"`
	}
	if err := json.Unmarshal(data, &player); err != nil {
		return nil, err
	}
	if player.Item.ID == "" {
		return nil, nil
	}

	artists := make([]string, len(player.Item.Artists))
	for i, a := range player.Item.Artists {
		artists[i] = a.Name
	}
	return &TrackInfo{
		ID:       player.Item.ID,
		Title:    player.Item.Name,
		Artists:  strings.Join(artists, ", "),
		Duration: player.Item.DurationMs,
		Progress: player.ProgressMs,
	}, nil
}

func (c *AppClient) GetPlaylistTracks(playlistID string) ([]PlaylistItem, error) {
	var items []PlaylistItem
	offset := 0
	for {
		endpoint := fmt.Sprintf("/playlists/%s/tracks?limit=100&offset=%d&fields=items(track(id,uri)),next", playlistID, offset)
		data, status, err := c.spotifyRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("spotify returned status %d", status)
		}

		var result struct {
			Items []struct {
				Track struct {
					ID  string `json:"id"`
					URI string `json:"uri"`
				} `json:"track"`
			} `json:"items"`
			Next string `json:"next"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			items = append(items, PlaylistItem{
				URI:      item.Track.URI,
				TrackID:  item.Track.ID,
				Position: offset + len(items),
			})
		}

		if result.Next == "" {
			break
		}
		offset += 100
	}
	return items, nil
}

func (c *AppClient) GetSnapshotID(playlistID string) (string, error) {
	data, status, err := c.spotifyRequest("GET", "/playlists/"+playlistID+"?fields=snapshot_id", nil)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("spotify returned status %d", status)
	}
	var result struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.SnapshotID, nil
}

func (c *AppClient) DeletePlaylistTracks(playlistID string, tracks []DeleteTrack, snapshotID string) error {
	type trackEntry struct {
		URI       string `json:"uri"`
		Positions []int  `json:"positions"`
	}
	var entries []trackEntry
	for _, t := range tracks {
		entries = append(entries, trackEntry{URI: t.URI, Positions: t.Positions})
	}
	payload := map[string]interface{}{
		"tracks":      entries,
		"snapshot_id": snapshotID,
	}
	_, status, err := c.spotifyRequest("DELETE", "/playlists/"+playlistID+"/tracks", payload)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("spotify returned status %d", status)
	}
	return nil
}

func (c *AppClient) GetPlaybackState() (*PlaybackState, error) {
	data, status, err := c.spotifyRequest("GET", "/me/player", nil)
	if err != nil {
		return nil, err
	}
	if status == 204 || len(data) == 0 {
		return nil, nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("spotify returned status %d", status)
	}

	var player struct {
		IsPlaying  bool  `json:"is_playing"`
		ProgressMs int64 `json:"progress_ms"`
		Item       struct {
			ID         string `json:"id"`
			DurationMs int64  `json:"duration_ms"`
		} `json:"item"`
		Context struct{} `json:"context"`
	}
	if err := json.Unmarshal(data, &player); err != nil {
		return nil, err
	}
	if player.Item.ID == "" {
		return nil, nil
	}
	return &PlaybackState{
		IsPlaying: player.IsPlaying,
		TrackID:   player.Item.ID,
		Duration:  player.Item.DurationMs,
		Progress:  player.ProgressMs,
	}, nil
}
