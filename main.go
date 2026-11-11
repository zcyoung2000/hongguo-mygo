package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"myguo/internal/appdirs"
	"myguo/internal/auth"
	"myguo/internal/ipc"
	"myguo/internal/media"
	"myguo/internal/native"
	"myguo/internal/resourcelog"
	userstore "myguo/internal/store"
	"myguo/internal/uiapp"
	"myguo/internal/upstream"
)

//go:embed frontend/*
var embeddedFS embed.FS

func main() {
	started := time.Now()
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
	if logFile, err := resourcelog.OpenRotated(
		filepath.Join(appDir, "app.log"),
		resourcelog.DefaultMaxLogBytes,
		resourcelog.DefaultMaxLogFiles,
	); err == nil {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("[startup] stage=process-init pid=%d elapsedMs=%d", os.Getpid(), time.Since(started).Milliseconds())

	// 1. Initialize SQLite first; account and anonymous-device identities live
	// beside user data in the same permissioned database.
	store, err := userstore.NewStore(appDir)
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()
	log.Printf("[startup] stage=store-ready elapsedMs=%d", time.Since(started).Milliseconds())

	profileStore := store.ProfileStore()
	hostPicker := upstream.NewHostPicker(store.UpstreamStateStore())
	if err := hostPicker.Restore(); err != nil {
		log.Printf("[upstream] host state unavailable: %v", err)
	}
	hostPicker.Configure(
		upstream.DefaultSuccessGain,
		upstream.DefaultFailurePenalty,
		upstream.DefaultMaxHealth,
		upstream.DefaultMinHealth,
		upstream.DefaultBadThreshold,
		upstream.CooldownFromEnvironment(),
		upstream.HalfOpenFromEnvironment(),
		upstream.StaleAgeFromEnvironment(),
	)
	hostTransport := upstream.NewTransport(hostPicker, http.DefaultTransport)
	hostTransport.SetEnabled(upstream.EnabledFromEnvironment())
	hostTransport.SetMaxAttempts(upstream.AttemptsFromEnvironment())
	authenticatedTransport := auth.New()
	authenticatedTransport.SetHTTPClient(&http.Client{Timeout: 30 * time.Second, Transport: hostTransport})
	client, err := sdk.NewClient(sdk.Options{
		Transport:     authenticatedTransport,
		ProfileStore:  profileStore,
		APIOrigin:     "https://" + upstream.DefaultPlaybackHosts[0],
		RetryAttempts: 3,
	})
	if err != nil {
		log.Fatalf("failed to init openhongguo client: %v", err)
	}
	log.Printf("[startup] stage=sdk-ready elapsedMs=%d", time.Since(started).Milliseconds())

	// 3. Initialize local media streaming & decryption server
	mediaServer, err := media.NewMediaServer(client)
	if err != nil {
		log.Fatalf("failed to init media server: %v", err)
	}
	defer mediaServer.Close()
	log.Printf("[startup] stage=media-ready elapsedMs=%d", time.Since(started).Milliseconds())
	mediaServer.SetEpisodeCacheCallback(store.RememberEpisode)

	if ffmpegPath, err := media.FFmpegCommand(); err != nil {
		log.Printf("[ffmpeg] playback decryption unavailable: %v", err)
	} else {
		log.Printf("[ffmpeg] using %s", ffmpegPath)
	}
	native.SetFeedScrollCapture(false)
	// 4. Create and bind HongguoService for MyGo IPC
	service := ipc.NewHongguoService(
		client,
		store,
		mediaServer,
		appDir,
		ipc.WithAuthTransport(authenticatedTransport),
		ipc.WithDeviceProfileStore(profileStore),
		ipc.WithUpstreamTransport(hostTransport),
	)
	defer ipc.CloseService(service)
	mediaServer.SetDeviceResetter(service.ResetDevice)
	mediaServer.SetMultiModelDeviceIdentity(service.DeviceIdentity)
	mygo.Bind(service)

	// 5. Setup embedded frontend
	frontendFS, err := fs.Sub(embeddedFS, "frontend")
	if err != nil {
		log.Fatalf("failed to sub frontend fs: %v", err)
	}
	mygo.SetFrontend(frontendFS)

	app := uiapp.NewApp(service)
	host := &uiapp.MediaHost{}
	app.SetMediaHost(host)
	app.SetArtworkSource(mediaServer.CacheDir(), mediaServer.ImageURL)
	app.LoadPreferences(filepath.Join(appDir, "native-ui.json"))
	app.SetConfirmation(func(message string) bool {
		ok, err := native.ShowConfirmDialog(native.ConfirmDialogArgs{Message: message, ConfirmText: "确认", Destructive: true})
		return err == nil && ok
	})
	mygo.BindAs("NativeShell", &uiapp.NativeShell{App: app})

	// Set the app-wide source before Run so the initial sidebar material is
	// already dark; MyGo applies this setting when the app starts.
	mygo.Theme.SetSource(mygo.ThemeDark)

	// 6. Launch macOS native window via MyGo
	mygo.App.WhenReady(func() {
		log.Printf("[startup] stage=native-ready elapsedMs=%d", time.Since(started).Milliseconds())
		// Initialize macOS native capabilities: Menus, Media Controls & NowPlaying
		native.Init()

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
			Content:         ui.View(app.Render),
		})
		playerView, err := win.NewWebView(mygo.WebViewOptions{
			URL: "/",
			Page: mygo.PageOptions{
				PictureInPicture: true,
				PreloadScript:    `window.myguoNativeMedia = true;`,
			},
		})
		if err != nil {
			log.Fatalf("failed to create player webview: %v", err)
		}
		host.Attach(playerView)
		native.SetScrollPage(playerView.Page())
		native.SetPictureInPictureHandler(func() { playerView.Page().TogglePictureInPicture() })
		app.SetLibraryGrids(uiapp.NewLibraryGrids())
		app.SetInvalidator(win.Invalidate)
		native.SetMediaCommandHandler(app.MediaCommand)
		native.SetNavigateHandler(func(payload native.NavigatePayload) {
			if payload.SeriesID != "" {
				go app.OpenDetail(payload.SeriesID)
				return
			}
			go app.OpenView(payload.View)
		})
		native.SetWindow(win)
		log.Printf("[startup] stage=native-window-created elapsedMs=%d", time.Since(started).Milliseconds())
		win.Show()
		win.Focus()
		native.StartScrollGestureMonitor()
		go app.OpenView("recommend")
		win.Invalidate()
		win.OnClosed(func() {
			fmt.Println("Main window closed")
			service.FlushPendingProgress()
			app.FlushPreferences()
			native.Cleanup()
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatalf("app run error: %v", err)
	}
}
