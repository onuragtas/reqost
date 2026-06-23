package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Subcommand routing — if the first arg is a known CLI verb, we don't
	// boot the GUI. Useful for CI ("reqost run collection.json") and shells
	// where bringing up a window would be wrong.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "run":
			os.Exit(cliRun(os.Args[2:]))
		case "mock":
			os.Exit(cliMock(os.Args[2:]))
		case "version", "--version", "-v":
			os.Exit(cliVersion())
		case "help", "--help", "-h":
			cliHelp()
			os.Exit(0)
		}
	}
	svc, err := NewCollectionService()
	if err != nil {
		log.Fatalf("init collection service: %v", err)
	}

	envSvc, err := NewEnvService()
	if err != nil {
		log.Fatalf("init env service: %v", err)
	}

	wsSvc := NewWSService()
	tcpSvc := NewTCPService()
	sseSvc := NewSSEService()
	grpcSvc := NewGRPCService()
	oauthSvc := NewOAuthService()
	gitSvc := NewGitService(svc)
	designSvc := NewDesignService()
	pluginSvc := NewPluginService()
	execSvc := NewExecService()
	execSvc.setPluginSvc(pluginSvc)
	windowSvc := NewWindowService()

	app := application.New(application.Options{
		Name:        "ReQost",
		Description: "High-performance desktop API client",
		Services: []application.Service{
			application.NewService(svc),
			application.NewService(execSvc),
			application.NewService(envSvc),
			application.NewService(wsSvc),
			application.NewService(tcpSvc),
			application.NewService(sseSvc),
			application.NewService(grpcSvc),
			application.NewService(oauthSvc),
			application.NewService(gitSvc),
			application.NewService(designSvc),
			application.NewService(pluginSvc),
			application.NewService(windowSvc),
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
	svc.setEnvSvc(envSvc)
	envSvc.setDialog(app.Dialog)
	wsSvc.setEmitter(app.Event)
	tcpSvc.setEmitter(app.Event)
	sseSvc.setEmitter(app.Event)
	grpcSvc.setEmitter(app.Event)
	oauthSvc.setApp(app)
	pluginSvc.setEmitter(app.Event)

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "ReQost",
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
	windowSvc.setWindow(mainWindow)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
