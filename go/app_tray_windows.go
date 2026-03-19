//go:build windows

package main

import (
	"fmt"
	"runtime"

	"fyne.io/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startSystray() {
	runtime.LockOSThread()
	systray.Run(func() {
		systray.SetIcon(generateIcon())
		systray.SetTooltip(fmt.Sprintf("Spotify Songrequest — Port %d", a.cfg.ServerPort))

		mShow := systray.AddMenuItem("Anzeigen", "")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Beenden", "")

		go func() {
			for {
				select {
				case <-mShow.ClickedCh:
					wailsruntime.WindowShow(a.ctx)
				case <-mQuit.ClickedCh:
					systray.Quit()
					wailsruntime.Quit(a.ctx)
				}
			}
		}()
	}, func() {})
}
