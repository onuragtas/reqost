package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	svc, err := NewCollectionService()
	if err != nil {
		log.Fatalf("init collection service: %v", err)
	}

	envSvc, err := NewEnvService()
	if err != nil {
		log.Fatalf("init env service: %v", err)
	}

	wsSvc := NewWSService()

	app := application.New(application.Options{
		Name:        "reqost",
		Description: "High-performance API client",
		Services: []application.Service{
			application.NewService(svc),
			application.NewService(NewExecService()),
			application.NewService(envSvc),
			application.NewService(wsSvc),
			application.NewService(NewGRPCService()),
			application.NewService(NewUpdateService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	svc.setEmitter(app.Event)
	svc.setDialog(app.Dialog)
	wsSvc.setEmitter(app.Event)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "reqost",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(20, 20, 20),
		URL:              "/",
		Width:            1280,
		Height:           800,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
