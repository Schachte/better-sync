package main

import (
	"embed"
	"fmt"
	"os"
	"time"

	"github.com/schachte/better-sync/pkg/device"
	"github.com/schachte/better-sync/pkg/model"
	"github.com/schachte/better-sync/pkg/operations"
	"github.com/schachte/better-sync/pkg/util"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var assets embed.FS

func (a *App) GetSongs() []model.Song {
	return a.Songs
}

func main() {
	afterBuild()
	app := NewApp()

	timeout := time.Duration(10) * time.Second
	go func() {
		for {
			util.LogError("Attempting to connect to device...")
			dev, err := device.Initialize(timeout)
			if err != nil {
				util.LogError("Failed to initialize device: %v", err)
				device.CheckForCommonMTPConflicts(err)
				time.Sleep(timeout)
				continue
			}

			storages, err := device.FetchStorages(dev, timeout)
			if err != nil {
				util.LogError("Failed to fetch storages: %v", err)
				dev.Close()
				time.Sleep(timeout)
				continue
			}

			songs, err := operations.GetSongs(dev, storages)
			if err != nil {
				util.LogError("Failed to get songs: %v", err)
				dev.Close()
				time.Sleep(timeout)
				continue
			}

			app.Songs = songs

			runtime.EventsEmit(app.ctx, "songs-loaded", songs)
			dev.Close()
			time.Sleep(timeout)
		}
	}()

	err := wails.Run(&options.App{
		Title:  "app",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}
