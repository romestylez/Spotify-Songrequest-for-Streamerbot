package main

import (
	"encoding/json"
	"os"
	"sync"
)

type SpotifyAppConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
}

type Config struct {
	ServerPort       int              `json:"server_port"`
	SpotifyMain      SpotifyAppConfig `json:"spotify_main"`
	SpotifyAutoclear SpotifyAppConfig `json:"spotify_autoclear"`
	PlaylistID       string           `json:"playlist_id"`
	FavPlaylistID    string           `json:"fav_playlist_id"`
}

var configMu sync.RWMutex

func defaultConfig() *Config {
	return &Config{
		ServerPort: 8765,
	}
}

func LoadConfig() *Config {
	configMu.RLock()
	defer configMu.RUnlock()

	data, err := os.ReadFile("config.json")
	if err != nil {
		return defaultConfig()
	}
	cfg := defaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return defaultConfig()
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = 8765
	}
	return cfg
}

func SaveConfig(cfg *Config) error {
	configMu.Lock()
	defer configMu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("config.json", data, 0644)
}
