# Songrequest – Spotify Playlist API for Streamer.bot

A lightweight Go application with a built-in HTTP server for adding Spotify tracks to a playlist – including OAuth login, automatic removal of played songs, and direct integration with **Streamer.bot** via simple HTTP requests. No PHP, no Node.js required.

## Features

- 🎧 **Add track** via Spotify URL or `spotify:track:` URI
- ⭐ **Favorite track**: Save the currently playing song *or* a specific track to a separate favorites playlist (duplicates prevented)
- 🎵 **Now Playing**: Query the currently playing Spotify track (ideal for OBS overlays)
- 🔐 **OAuth login** via built-in browser flow (MAIN & AUTOCLEAR)
- 🧹 **Auto-clean**: Removes already played songs from the playlist automatically
- 🗑️ **Clear playlist** via endpoint
- ⚙️ **config.json** based configuration (managed via the GUI)
- 🤝 **Streamer.bot compatible**: Direct HTTP request actions, plain text responses — no scripts needed

---

## Quick Start

### Requirements

- Spotify Developer Account + **two apps**
  - **MAIN** → For song requests & favorites
  - **AUTOCLEAR** → For automatic cleanup
  - Redirect URI in both apps: `http://127.0.0.1:8765/callback`
  - Required scopes: `playlist-modify-private`, `playlist-modify-public`, `user-read-playback-state`

### Installation

Download the latest release, run the `.exe` and configure it via the GUI.

### Spotify Login

OAuth login is done via the GUI — click **Connect** for MAIN and AUTOCLEAR separately.

---

## Endpoints

All endpoints return **plain text** – ready to use directly in Streamer.bot.

### Song Request
```
GET /fetch?raw=SPOTIFY_URL
```
- Validates the track exists on Spotify before adding
- Returns the song name on success

**Responses:**
```
🎵 Hinzugefügt: Artist — Title
❌ Song nicht gefunden – ungültiger oder nicht existierender Track-Link
❌ Kein gültiger Spotify-Track-Link gefunden
❌ Das ist ein Album-Link, bitte einen einzelnen Song-Link senden
```

---

### Favorite Track
```
GET /fav
GET /fav?url=SPOTIFY_URL
```
- Without `url` → saves the currently playing track
- With `url` → saves the provided track
- Duplicates are prevented

**Responses:**
```
⭐ Favorit gespeichert: Artist — Title
❌ Song ist bereits in den Favoriten
❌ Kein Song wird gerade gespielt
```

---

### Now Playing
```
GET /nowplaying
```

**Responses:**
```
🎵 Aktueller Song: Artist — Title
🎧 Aktuell wird kein Song abgespielt
```

---

### Add Track (JSON)
```
POST /add
GET  /add?url=SPOTIFY_URL
```
Returns JSON – for custom integrations.

---

### Clear Playlist
```
GET /clear
```

---

## Configuration (config.json)

Managed via the GUI. Structure:

```json
{
  "server_port": 8765,
  "playlist_id": "YOUR_PLAYLIST_ID",
  "fav_playlist_id": "YOUR_FAV_PLAYLIST_ID",
  "spotify_main": {
    "client_id": "",
    "client_secret": "",
    "refresh_token": ""
  },
  "spotify_autoclear": {
    "client_id": "",
    "client_secret": "",
    "refresh_token": ""
  }
}
```

> `refresh_token` is filled in automatically after the OAuth login.

---

## Streamer.bot Setup

All commands use **Fetch URL** action with a direct HTTP GET request.
The response variable (e.g. `%fetchOutput%`) contains the plain text message — send it directly to chat.

### Song Request (Channel Points / Command)

```
http://localhost:8765/fetch?raw=%rawInput%
```

### Favorite current song

```
http://localhost:8765/fav
```

### Favorite a specific song

```
http://localhost:8765/fav?url=%rawInput%
```

### Now Playing

```
http://localhost:8765/nowplaying
```
