package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type jsonResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	ID      string `json:"id,omitempty"`
}

func jsonOK(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonResponse{OK: true, Message: msg})
}

func jsonOKWithID(w http.ResponseWriter, msg, id string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonResponse{OK: true, Message: msg, ID: id})
}

func jsonFail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(jsonResponse{OK: false, Message: msg})
}

func plainText(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprint(w, msg)
}

// HandleAdd handles POST/GET /add — adds a track to the main playlist
func HandleAdd(client *AppClient, cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := extractTrackInput(r)
		if input == "" {
			jsonFail(w, 400, "Kein Track-Link angegeben")
			return
		}

		uri, err := ExtractTrackURI(input)
		if err != nil {
			jsonFail(w, 400, err.Error())
			return
		}

		trackID := strings.TrimPrefix(uri, "spotify:track:")
		trackInfo, err := client.GetTrackInfo(trackID)
		if err != nil {
			jsonFail(w, 400, "❌ "+err.Error())
			return
		}

		if err := client.AddTrack(cfg.PlaylistID, uri); err != nil {
			jsonFail(w, 500, "Fehler beim Hinzufügen: "+err.Error())
			return
		}

		jsonOK(w, fmt.Sprintf("🎵 Hinzugefügt: %s — %s", trackInfo.Artists, trackInfo.Title))
	}
}

// HandleFav handles POST/GET /fav — adds track to favorites with duplicate check
func HandleFav(client *AppClient, cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var uri string

		input := extractTrackInput(r)
		if input != "" {
			var err error
			uri, err = ExtractTrackURI(input)
			if err != nil {
				plainText(w, 200, "❌ "+err.Error())
				return
			}
		} else {
			track, err := client.GetCurrentTrack()
			if err != nil || track == nil {
				plainText(w, 200, "❌ Kein Song wird gerade gespielt")
				return
			}
			uri = "spotify:track:" + track.ID
		}

		trackID := strings.TrimPrefix(uri, "spotify:track:")

		// Duplicate check: fetch all tracks in fav playlist
		existing, err := client.GetPlaylistTracks(cfg.FavPlaylistID)
		if err != nil {
			plainText(w, 200, "❌ Fehler beim Prüfen der Favoriten: "+err.Error())
			return
		}
		for _, item := range existing {
			if item.TrackID == trackID {
				plainText(w, 200, "❌ Song ist bereits in den Favoriten")
				return
			}
		}

		if err := client.AddTrack(cfg.FavPlaylistID, uri); err != nil {
			plainText(w, 200, "❌ Fehler beim Hinzufügen zu Favoriten: "+err.Error())
			return
		}

		// Get track info for response
		track, err := client.GetCurrentTrack()
		if err == nil && track != nil {
			plainText(w, 200, fmt.Sprintf("⭐ Favorit gespeichert: %s – %s", track.Artists, track.Title))
			return
		}
		plainText(w, 200, "⭐ Song zu Favoriten hinzugefügt")
	}
}

// HandleSong handles GET /song — returns currently playing track
func HandleSong(client *AppClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		track, err := client.GetCurrentTrack()
		if err != nil {
			jsonFail(w, 500, "Fehler beim Abrufen des aktuellen Songs: "+err.Error())
			return
		}
		if track == nil {
			jsonOKWithID(w, "🎧 Aktuell wird kein Song abgespielt", "")
			return
		}
		msg := fmt.Sprintf("🎵 Aktueller Song: %s — %s", track.Artists, track.Title)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonResponse{OK: true, Message: msg, ID: track.ID})
	}
}

// HandleClear handles GET /clear — removes all tracks from main playlist
func HandleClear(client *AppClient, cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tracks, err := client.GetPlaylistTracks(cfg.PlaylistID)
		if err != nil {
			jsonFail(w, 500, "Fehler beim Laden der Playlist: "+err.Error())
			return
		}
		if len(tracks) == 0 {
			jsonOK(w, "Playlist ist bereits leer")
			return
		}

		snapshotID, err := client.GetSnapshotID(cfg.PlaylistID)
		if err != nil {
			jsonFail(w, 500, "Fehler beim Abrufen der Snapshot-ID: "+err.Error())
			return
		}

		// Group by URI with positions
		byURI := map[string][]int{}
		for i, t := range tracks {
			byURI[t.URI] = append(byURI[t.URI], i)
		}
		var toDelete []DeleteTrack
		for uri, positions := range byURI {
			toDelete = append(toDelete, DeleteTrack{URI: uri, Positions: positions})
		}

		if err := client.DeletePlaylistTracks(cfg.PlaylistID, toDelete, snapshotID); err != nil {
			jsonFail(w, 500, "Fehler beim Leeren der Playlist: "+err.Error())
			return
		}

		jsonOK(w, fmt.Sprintf("%d Songs aus der Playlist entfernt", len(tracks)))
	}
}

// HandleFetch handles GET /fetch — for Streamer.bot direct HTTP request
// Returns plain text so %response% or %output1% contains only the message
func HandleFetch(client *AppClient, cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.URL.Query().Get("raw")
		msg := r.URL.Query().Get("msg")

		input := raw
		if input == "" {
			input = msg
		}
		if input == "" {
			plainText(w, 200, "❌ Kein Link angegeben")
			return
		}

		uri, err := ExtractTrackURI(input)
		if err != nil {
			plainText(w, 200, "❌ "+err.Error())
			return
		}

		trackID := strings.TrimPrefix(uri, "spotify:track:")
		trackInfo, err := client.GetTrackInfo(trackID)
		if err != nil {
			plainText(w, 200, "❌ "+err.Error())
			return
		}

		if err := client.AddTrack(cfg.PlaylistID, uri); err != nil {
			plainText(w, 200, "❌ Fehler beim Hinzufügen: "+err.Error())
			return
		}

		plainText(w, 200, fmt.Sprintf("🎵 Hinzugefügt: %s — %s", trackInfo.Artists, trackInfo.Title))
	}
}

// HandleNowPlaying handles GET /nowplaying — replaces fetch_nowplaying.js for Streamer.bot
// Returns plain text so %response%/%output1% contains only the message
func HandleNowPlaying(client *AppClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		track, err := client.GetCurrentTrack()
		if err != nil {
			plainText(w, 200, "❌ Fehler: "+err.Error())
			return
		}
		if track == nil {
			plainText(w, 200, "🎧 Aktuell wird kein Song abgespielt")
			return
		}
		plainText(w, 200, fmt.Sprintf("🎵 Aktueller Song: %s — %s", track.Artists, track.Title))
	}
}

func extractTrackInput(r *http.Request) string {
	// Try POST body first
	if r.Method == http.MethodPost {
		var body struct {
			URL      string `json:"url"`
			RawInput string `json:"rawInput"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if body.URL != "" {
				return body.URL
			}
			if body.RawInput != "" {
				return body.RawInput
			}
		}
	}
	// Fall back to query params
	if u := r.URL.Query().Get("url"); u != "" {
		return u
	}
	return r.URL.Query().Get("rawInput")
}
