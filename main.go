package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "BranchPier",
		Description: "An open-source desktop Git client",
		Services:    []application.Service{application.NewService(&GitService{})},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "BranchPier",
		URL:   "/",
		Width: 1180, Height: 760,
		MinWidth: 780, MinHeight: 540,
		BackgroundColour: application.NewRGB(15, 18, 26),
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
