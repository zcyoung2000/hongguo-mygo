package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/filestore"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/egoist/mygo"
	"myguo/internal/appdirs"
	"myguo/internal/auth"
	"myguo/internal/ipc"
	"myguo/internal/media"
	"myguo/internal/native"
	userstore "myguo/internal/store"
)

//go:embed frontend/*
var embeddedFS embed.FS

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get user home: %v", err)
	}

	appDir, err := appdirs.AppSupportDir(home)
	if err != nil {
		log.Fatalf("failed to prepare app dir: %v", err)
	}
	if err := os.MkdirAll(appDir, 0755); err != nil {
		log.Fatalf("failed to create app dir: %v", err)
	}

	// GUI launches discard stdout/stderr, so retain backend diagnostics on disk.
	if logFile, err := os.OpenFile(filepath.Join(appDir, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}

	// 1. Initialize OpenHongguo client with persistent anonymous device profile
	profilePath := filepath.Join(appDir, "anonymous.json")
	authenticatedTransport := auth.New()
	client, err := adapters.NewClient(profilePath, sdk.Options{
		Transport:     authenticatedTransport,
		RetryAttempts: 3,
	})
	if err != nil {
		log.Fatalf("failed to init openhongguo client: %v", err)
	}

	// 2. Initialize persistent store for user data (history, favorites, likes, reservations)
	store, err := userstore.NewStore(appDir)
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	// 3. Initialize local media streaming & decryption server
	mediaServer, err := media.NewMediaServer(client)
	if err != nil {
		log.Fatalf("failed to init media server: %v", err)
	}
	defer mediaServer.Close()
	mediaServer.SetEpisodeCacheCallback(store.RememberEpisode)

	if ffmpegPath, err := media.FFmpegCommand(); err != nil {
		log.Printf("[ffmpeg] playback decryption unavailable: %v", err)
	} else {
		log.Printf("[ffmpeg] using %s", ffmpegPath)
	}
	native.SetFeedScrollCapture(true)

	// 4. Create and bind HongguoService for MyGo IPC
	service := ipc.NewHongguoService(client, store, mediaServer, appDir)
	service.SetAuthTransport(authenticatedTransport)
	service.SetDeviceProfileStore(filestore.New(profilePath))
	mediaServer.SetDeviceResetter(service.ResetDeviceWithContext)
	mygo.Bind(service)

	// 5. Setup embedded frontend
	frontendFS, err := fs.Sub(embeddedFS, "frontend")
	if err != nil {
		log.Fatalf("failed to sub frontend fs: %v", err)
	}
	mygo.SetFrontend(frontendFS)

	// Set the app-wide source before Run so the initial sidebar material is
	// already dark; MyGo applies this setting when the app starts.
	mygo.Theme.SetSource(mygo.ThemeDark)

	// 6. Launch macOS native window via MyGo
	mygo.App.WhenReady(func() {
		// Initialize macOS native capabilities: Menus, Media Controls & NowPlaying
		native.Init()

		// Keep the content area opaque while only the transparent sidebar exposes
		// AppKit's native sidebar vibrancy.
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           appdirs.AppName,
			Width:           1280,
			Height:          820,
			MinWidth:        1000,
			MinHeight:       640,
			TitleBarStyle:   mygo.TitleBarHiddenInset,
			Transparent:     true,
			BackgroundColor: "transparent",
			Vibrancy:        mygo.VibrancySidebar,
			URL:             "/",
			Page: mygo.PageOptions{
				PictureInPicture: true,
			},
		})
		native.SetWindow(win)
		native.StartScrollGestureMonitor()
		win.OnClosed(func() {
			fmt.Println("Main window closed")
			native.Cleanup()
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatalf("app run error: %v", err)
	}
}
