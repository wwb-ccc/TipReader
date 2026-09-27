package main

import (
	"embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/* webextra/*
var embeddedAssets embed.FS

// appDir returns the directory containing the executable (spec §13).
func appDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// serveEmbedded streams one of the embedded files (e.g. viewer.html) to the
// web viewer served by the controller's LAN HTTP server.
func serveEmbedded(w http.ResponseWriter, r *http.Request, name string) {
	f, err := embeddedAssets.Open(name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, f)
}

func main() {
	log.SetFlags(log.LstdFlags)
	dir := appDir()

	cfg, err := newConfigStore(dir)
	if err != nil {
		log.Fatalf("配置初始化失败: %v", err)
	}

	appSvc := NewApp(dir, cfg)

	// For the controller, a missing read.md must fail clearly, not silently.
	if parseErr := appSvc.Load(); parseErr != nil {
		if cfg.Get().Role == "01" {
			appSvc.setRemoteErr(fmt.Sprintf(
				"无法读取稿件：未找到 %s\n请将 read.md 放到 TipReader.exe 同一文件夹。",
				filepath.Join(dir, "read.md")))
		}
	}

	app := application.New(application.Options{
		Name: "TipReader",
		Services: []application.Service{
			application.NewService(appSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(embeddedAssets),
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
	})

	winOpts := cfg.Get()
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:               "TipReader",
		Width:               winOpts.WindowWidth,
		Height:              winOpts.WindowHeight,
		MinWidth:            200,
		MinHeight:           120,
		AlwaysOnTop:         winOpts.AlwaysOnTop,
		Frameless:           true,
		BackgroundType:      application.BackgroundTypeTransparent,
		BackgroundColour:    application.NewRGBA(0, 0, 0, 0),
		ContentProtectionEnabled: winOpts.ContentProtection,
		URL:                 "/",
		Windows: application.WindowsWindow{
			DisableFramelessWindowDecorations: true,
		},
		InitialPosition: application.WindowCentered,
	})

	appSvc.bindWindow(app, win)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}