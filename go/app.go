package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// appInstance is accessed by server.go to emit Wails events after OAuth
var appInstance *App

type App struct {
	ctx             context.Context
	cfg             *Config
	mainClient      *AppClient
	autoclearClient *AppClient
}

func NewApp(cfg *Config, main, autoclear *AppClient) *App {
	a := &App{cfg: cfg, mainClient: main, autoclearClient: autoclear}
	appInstance = a
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.startSystray()
}

// SafeAppConfig masks the secret but exposes connection state
type SafeAppConfig struct {
	ClientID  string `json:"client_id"`
	SecretSet bool   `json:"secret_set"`
	Connected bool   `json:"connected"`
}

type SafeConfig struct {
	ServerPort       int           `json:"server_port"`
	SpotifyMain      SafeAppConfig `json:"spotify_main"`
	SpotifyAutoclear SafeAppConfig `json:"spotify_autoclear"`
	PlaylistID       string        `json:"playlist_id"`
	FavPlaylistID    string        `json:"fav_playlist_id"`
}

func (a *App) GetSettings() SafeConfig {
	return SafeConfig{
		ServerPort: a.cfg.ServerPort,
		SpotifyMain: SafeAppConfig{
			ClientID:  a.cfg.SpotifyMain.ClientID,
			SecretSet: a.cfg.SpotifyMain.ClientSecret != "",
			Connected: a.cfg.SpotifyMain.RefreshToken != "",
		},
		SpotifyAutoclear: SafeAppConfig{
			ClientID:  a.cfg.SpotifyAutoclear.ClientID,
			SecretSet: a.cfg.SpotifyAutoclear.ClientSecret != "",
			Connected: a.cfg.SpotifyAutoclear.RefreshToken != "",
		},
		PlaylistID:    a.cfg.PlaylistID,
		FavPlaylistID: a.cfg.FavPlaylistID,
	}
}

type SaveInput struct {
	ServerPort       int    `json:"server_port"`
	MainClientID     string `json:"main_client_id"`
	MainClientSecret string `json:"main_client_secret"`
	AcClientID       string `json:"ac_client_id"`
	AcClientSecret   string `json:"ac_client_secret"`
	PlaylistID       string `json:"playlist_id"`
	FavPlaylistID    string `json:"fav_playlist_id"`
}

func (a *App) SaveSettings(input SaveInput) error {
	if input.ServerPort > 0 {
		a.cfg.ServerPort = input.ServerPort
	}
	a.cfg.SpotifyMain.ClientID = input.MainClientID
	if input.MainClientSecret != "" {
		a.cfg.SpotifyMain.ClientSecret = input.MainClientSecret
	}
	a.cfg.SpotifyAutoclear.ClientID = input.AcClientID
	if input.AcClientSecret != "" {
		a.cfg.SpotifyAutoclear.ClientSecret = input.AcClientSecret
	}
	a.cfg.PlaylistID = input.PlaylistID
	a.cfg.FavPlaylistID = input.FavPlaylistID
	return SaveConfig(a.cfg)
}

func (a *App) OpenLogin(appType string) {
	if appType != "main" && appType != "autoclear" {
		appType = "main"
	}
	openBrowser(fmt.Sprintf("http://127.0.0.1:%d/login?app=%s", a.cfg.ServerPort, appType))
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	exec.Command(cmd, args...).Start()
}
