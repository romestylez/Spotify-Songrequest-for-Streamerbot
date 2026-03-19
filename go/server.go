package main

import (
	"fmt"
	"net/http"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func StartServer(cfg *Config, mainClient, autoclearClient *AppClient) {
	mux := http.NewServeMux()

	// Song request endpoints (main app credentials)
	mux.HandleFunc("/add", HandleAdd(mainClient, cfg))
	mux.HandleFunc("/fav", HandleFav(mainClient, cfg))
	mux.HandleFunc("/song", HandleSong(mainClient))
	mux.HandleFunc("/clear", HandleClear(mainClient, cfg))

	// Streamer.bot integration endpoints
	mux.HandleFunc("/fetch", HandleFetch(mainClient, cfg))
	mux.HandleFunc("/nowplaying", HandleNowPlaying(mainClient))

	// Autoclear endpoints
	mux.HandleFunc("/autoclear", func(w http.ResponseWriter, r *http.Request) {
		dryRun := r.URL.Query().Get("dry") == "1"
		if err := doAutoclear(autoclearClient, cfg, dryRun); err != nil {
			jsonFail(w, 500, "Autoclear error: "+err.Error())
			return
		}
		jsonOK(w, "Autoclear ausgeführt")
	})

	// OAuth endpoints
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		appType := r.URL.Query().Get("app")
		if appType != "main" && appType != "autoclear" {
			appType = "main"
		}
		authURL := BuildAuthURL(cfg, appType)
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		appType := r.URL.Query().Get("state")
		if code == "" {
			jsonFail(w, 400, "Kein Authorization Code erhalten")
			return
		}

		var client *AppClient
		if appType == "autoclear" {
			client = autoclearClient
		} else {
			client = mainClient
		}

		redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", cfg.ServerPort)
		if err := client.ExchangeCode(code, redirectURI); err != nil {
			jsonFail(w, 500, "Token-Austausch fehlgeschlagen: "+err.Error())
			return
		}

		// Notify Wails frontend
		if appInstance != nil && appInstance.ctx != nil {
			wailsruntime.EventsEmit(appInstance.ctx, "oauth:success")
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html><head><meta charset="utf-8">
<style>body{font-family:sans-serif;background:#1a1a2e;color:#e0e0e0;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;}
.box{background:#16213e;padding:2rem;border-radius:12px;text-align:center;border:1px solid #1db954;}
h2{color:#1db954;}</style>
<script>window.onload=function(){setTimeout(function(){window.close();},2000);};</script>
</head>
<body><div class="box"><h2>&#10003; Verbindung erfolgreich!</h2>
<p>Spotify wurde erfolgreich verbunden.</p>
<p style="color:#5a6175;font-size:0.85em">Dieses Fenster schlie&#223;t sich automatisch&hellip;</p>
</div></body></html>`)
	})

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	if err := http.ListenAndServe(addr, mux); err != nil {
		panic("Server failed: " + err.Error())
	}
}
