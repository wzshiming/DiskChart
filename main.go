package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/wzshiming/DiskChart/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

var version = "dev"

func main() {
	rt := &app.WailsRuntime{}
	application := app.New(rt, app.Deps{Version: version})

	err := wails.Run(&options.App{
		Title:     "DiskChart",
		Width:     1200,
		Height:    760,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: rt.Startup,
		Bind: []any{
			application,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
