package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AppClient struct {
	cfg        *Config
	appType    string // "main" or "autoclear"
	mu         sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func NewAppClient(cfg *Config, appType string) *AppClient {
	return &AppClient{cfg: cfg, appType: appType}
}

func (c *AppClient) appConfig() SpotifyAppConfig {
	if c.appType == "autoclear" {
		// Fall back to main credentials if autoclear is not configured
		if c.cfg.SpotifyAutoclear.RefreshToken != "" {
			return c.cfg.SpotifyAutoclear
		}
		return c.cfg.SpotifyMain
	}
	return c.cfg.SpotifyMain
}

func (c *AppClient) GetAccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	appCfg := c.appConfig()
	if appCfg.ClientID == "" || appCfg.ClientSecret == "" || appCfg.RefreshToken == "" {
		return "", fmt.Errorf("spotify credentials not configured for %s app", c.appType)
	}

	token, expiresIn, err := refreshAccessToken(appCfg.ClientID, appCfg.ClientSecret, appCfg.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("token refresh failed (%s): %w", c.appType, err)
	}

	c.accessToken = token
	c.tokenExpiry = time.Now().Add(time.Duration(expiresIn-30) * time.Second)
	return token, nil
}

func (c *AppClient) InvalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
}

func (c *AppClient) ExchangeCode(code, redirectURI string) error {
	appCfg := c.appConfig()
	if appCfg.ClientID == "" || appCfg.ClientSecret == "" {
		return fmt.Errorf("client credentials not configured for %s app", c.appType)
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(appCfg.ClientID + ":" + appCfg.ClientSecret))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		RefreshToken string `json:"refresh_token"`
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Error != "" {
		return fmt.Errorf("spotify error: %s", result.Error)
	}

	// Save refresh token to config
	c.mu.Lock()
	if c.appType == "autoclear" {
		c.cfg.SpotifyAutoclear.RefreshToken = result.RefreshToken
	} else {
		c.cfg.SpotifyMain.RefreshToken = result.RefreshToken
	}
	c.accessToken = result.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-30) * time.Second)
	c.mu.Unlock()

	return SaveConfig(c.cfg)
}

func refreshAccessToken(clientID, clientSecret, refreshToken string) (string, int, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", 0, err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, err
	}
	if result.Error != "" {
		return "", 0, fmt.Errorf("spotify: %s", result.Error)
	}
	return result.AccessToken, result.ExpiresIn, nil
}
