package main

import (
	"embed"
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	woptions "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend
var rawAssets embed.FS

func main() {
	cfg := LoadConfig()
	mainClient := NewAppClient(cfg, "main")
	autoclearClient := NewAppClient(cfg, "autoclear")

	go StartServer(cfg, mainClient, autoclearClient)
	go RunAutoclear(autoclearClient, cfg)

	assets, _ := fs.Sub(rawAssets, "frontend")
	app := NewApp(cfg, mainClient, autoclearClient)

	err := wails.Run(&options.App{
		Title:             "Spotify Songrequest",
		Width:             680,
		Height:            760,
		BackgroundColour:  &options.RGBA{R: 13, G: 15, B: 20, A: 255},
		AssetServer:       &assetserver.Options{Assets: assets},
		OnStartup:         app.startup,
		Bind:              []interface{}{app},
		HideWindowOnClose: true,
		Windows: &woptions.Options{
			Theme: woptions.Dark,
		},
	})
	if err != nil {
		panic(err)
	}
}
