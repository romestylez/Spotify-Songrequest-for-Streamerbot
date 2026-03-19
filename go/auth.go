package main

import (
	"fmt"
	"net/url"
)

const spotifyAuthURL = "https://accounts.spotify.com/authorize"

var spotifyScopes = "playlist-modify-private playlist-modify-public user-read-playback-state"

func BuildAuthURL(cfg *Config, appType string) string {
	var clientID string
	if appType == "autoclear" {
		clientID = cfg.SpotifyAutoclear.ClientID
	} else {
		clientID = cfg.SpotifyMain.ClientID
	}

	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", cfg.ServerPort)

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", spotifyScopes)
	params.Set("state", appType)

	return spotifyAuthURL + "?" + params.Encode()
}
