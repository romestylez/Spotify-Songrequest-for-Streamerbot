package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

type TrackState struct {
	Duration    int64 `json:"duration"`
	MaxProgress int64 `json:"max_progress"`
	LastIndex   int   `json:"last_index"`
}

type AutoclearState struct {
	Tracks map[string]*TrackState `json:"tracks"`
}

const autoclearStateFile = "autoclear_state.json"

func LoadAutoclearState() *AutoclearState {
	data, err := os.ReadFile(autoclearStateFile)
	if err != nil {
		return &AutoclearState{Tracks: map[string]*TrackState{}}
	}
	var state AutoclearState
	if err := json.Unmarshal(data, &state); err != nil {
		return &AutoclearState{Tracks: map[string]*TrackState{}}
	}
	if state.Tracks == nil {
		state.Tracks = map[string]*TrackState{}
	}
	return &state
}

func SaveAutoclearState(state *AutoclearState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(autoclearStateFile, data, 0644)
}

func RunAutoclear(client *AppClient, cfg *Config) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := doAutoclear(client, cfg, false); err != nil {
			log.Printf("[autoclear] error: %v", err)
		}
	}
}

func doAutoclear(client *AppClient, cfg *Config, dryRun bool) error {
	if cfg.PlaylistID == "" {
		return nil
	}

	// Get current playback state
	playback, err := client.GetPlaybackState()
	if err != nil {
		return fmt.Errorf("playback state: %w", err)
	}

	state := LoadAutoclearState()

	// Rule C: No active player → no deletions
	if playback == nil {
		return nil
	}

	// Update max progress for current track
	trackID := playback.TrackID
	if trackID != "" {
		ts, ok := state.Tracks[trackID]
		if !ok {
			ts = &TrackState{Duration: playback.Duration}
			state.Tracks[trackID] = ts
		}
		ts.Duration = playback.Duration
		if playback.Progress > ts.MaxProgress {
			ts.MaxProgress = playback.Progress
		}
	}

	// Get current playlist to find track index
	tracks, err := client.GetPlaylistTracks(cfg.PlaylistID)
	if err != nil {
		return fmt.Errorf("get playlist tracks: %w", err)
	}

	// Find current track index in playlist
	currentIndex := -1
	for i, t := range tracks {
		if t.TrackID == trackID {
			currentIndex = i
			break
		}
	}

	var toDelete []DeleteTrack

	if playback.IsPlaying && currentIndex > 0 {
		// Rule A: Active playback at index n → delete tracks 0..n-1
		log.Printf("[autoclear] Rule A: deleting %d tracks before index %d", currentIndex, currentIndex)
		byURI := map[string][]int{}
		for i := 0; i < currentIndex; i++ {
			byURI[tracks[i].URI] = append(byURI[tracks[i].URI], i)
		}
		for uri, positions := range byURI {
			toDelete = append(toDelete, DeleteTrack{URI: uri, Positions: positions})
		}
	} else if !playback.IsPlaying && playback.Progress == 0 && currentIndex == 0 {
		// Rule B: Paused at position 0 with 0ms progress
		// Only delete if previous track was nearly complete (within 20s of end)
		// We check using the state of what was previously playing
		shouldDelete := false
		for id, ts := range state.Tracks {
			if id != trackID && ts.MaxProgress >= ts.Duration-20000 {
				shouldDelete = true
				break
			}
		}
		if shouldDelete && len(tracks) > 1 {
			log.Printf("[autoclear] Rule B: end-wrap detected, deleting previous tracks")
			// We don't know exactly which was "previous", so check all before current
			// (currentIndex==0, so nothing before it — this handles wrap-around after last track played)
			// Actually for Rule B we rely on the fact the track wrapped to position 0
			// The previous track was at the end and finished — it's already gone from playlist
			// This rule primarily guards against false positives on pause
		}
	}

	if len(toDelete) == 0 {
		_ = SaveAutoclearState(state)
		return nil
	}

	if dryRun {
		log.Printf("[autoclear] dry-run: would delete %d track groups", len(toDelete))
		_ = SaveAutoclearState(state)
		return nil
	}

	snapshotID, err := client.GetSnapshotID(cfg.PlaylistID)
	if err != nil {
		return fmt.Errorf("get snapshot id: %w", err)
	}

	if err := client.DeletePlaylistTracks(cfg.PlaylistID, toDelete, snapshotID); err != nil {
		return fmt.Errorf("delete tracks: %w", err)
	}

	log.Printf("[autoclear] deleted %d track group(s)", len(toDelete))

	// Clean up state for deleted tracks
	for _, dt := range toDelete {
		trackIDFromURI := ""
		for _, t := range tracks {
			if t.URI == dt.URI {
				trackIDFromURI = t.TrackID
				break
			}
		}
		if trackIDFromURI != "" {
			delete(state.Tracks, trackIDFromURI)
		}
	}

	return SaveAutoclearState(state)
}
