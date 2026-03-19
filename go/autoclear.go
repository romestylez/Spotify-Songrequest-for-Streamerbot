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
	Tracks         map[string]*TrackState `json:"tracks"`
	LastCurrID     string                 `json:"last_curr_id"`
	LastIdx        int                    `json:"last_idx"`
	LastTotal      int                    `json:"last_total"`
	LastProgressMs int64                  `json:"last_progress_ms"`
	LastDurationMs int64                  `json:"last_duration_ms"`
}

const autoclearStateFile = "autoclear_state.json"
const nearEndPadMs = 20000

func LoadAutoclearState() *AutoclearState {
	data, err := os.ReadFile(autoclearStateFile)
	if err != nil {
		return &AutoclearState{Tracks: map[string]*TrackState{}, LastIdx: -1}
	}
	var state AutoclearState
	if err := json.Unmarshal(data, &state); err != nil {
		return &AutoclearState{Tracks: map[string]*TrackState{}, LastIdx: -1}
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

	playback, err := client.GetPlaybackState()
	if err != nil {
		return fmt.Errorf("playback state: %w", err)
	}

	state := LoadAutoclearState()

	// Rule C: No active player → no deletions
	if playback == nil {
		state.LastCurrID = ""
		state.LastIdx = -1
		_ = SaveAutoclearState(state)
		return nil
	}

	trackID := playback.TrackID

	// Update max progress for current track
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

	tracks, err := client.GetPlaylistTracks(cfg.PlaylistID)
	if err != nil {
		return fmt.Errorf("get playlist tracks: %w", err)
	}

	total := len(tracks)

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
		// Rule B: Paused at position 0 with 0ms progress → check for end-wrap
		// Only delete if the previously observed track was at the last position
		// AND had progress near its end (within 20s buffer)
		lastIdx := state.LastIdx
		lastTotal := state.LastTotal
		lastDurMs := state.LastDurationMs
		lastProgMs := state.LastProgressMs

		var threshold int64
		if lastDurMs-nearEndPadMs > 0 {
			threshold = lastDurMs - nearEndPadMs
		}
		hadNearEnd := lastDurMs > 0 && lastProgMs >= threshold
		wasAtLast := lastIdx >= 0 && lastTotal > 0 && lastIdx == lastTotal-1

		if wasAtLast && hadNearEnd {
			log.Printf("[autoclear] Rule B: end-wrap confirmed, deleting all %d tracks", total)
			byURI := map[string][]int{}
			for i, t := range tracks {
				byURI[t.URI] = append(byURI[t.URI], i)
			}
			for uri, positions := range byURI {
				toDelete = append(toDelete, DeleteTrack{URI: uri, Positions: positions})
			}
		} else {
			log.Printf("[autoclear] Rule B: paused at start, no end-wrap confirmed (lastIdx=%d, lastTotal=%d, lastProg=%d, lastDur=%d)", lastIdx, lastTotal, lastProgMs, lastDurMs)
		}
	}

	// Update last-seen state for next cycle
	state.LastCurrID = trackID
	state.LastIdx = currentIndex
	state.LastTotal = total
	state.LastProgressMs = playback.Progress
	state.LastDurationMs = playback.Duration

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
		for _, t := range tracks {
			if t.URI == dt.URI {
				delete(state.Tracks, t.TrackID)
				break
			}
		}
	}

	return SaveAutoclearState(state)
}
