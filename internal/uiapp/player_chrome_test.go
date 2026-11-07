package uiapp

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestNativePlayerControlsOverlayAndCommands(t *testing.T) {
	app := NewTestApp(nil)
	view := &mygo.WebView{}
	app.SetMediaHost(&MediaHost{view: view})
	app.detail = Detail{ID: "123456", Episodes: []CatalogItem{{ID: "1", EpisodeCount: 1}, {ID: "2", EpisodeCount: 2}}}
	app.PlayDetail()
	shell := NativeShell{App: app}
	shell.PlayerChromeChanged(PlayerChrome{
		Visible: true, ControlsVisible: true,
		Quality:      ControlFrame{X: 500, Y: 700, W: 52, H: 28},
		Speed:        ControlFrame{X: 560, Y: 700, W: 52, H: 28},
		Drawer:       ControlFrame{X: 760, Y: 700, W: 72, H: 28},
		QualityLabel: "1080P", QualityOptions: []PlayerQualityOption{
			{ID: "1080p", Label: "1080P", Available: true, Active: true},
			{ID: "720p", Label: "720P"},
		}, SpeedValue: 1.25, Episode: 2, TotalEpisodes: 12,
	})
	app.UpdatePlayerChrome(PlayerChrome{Visible: true, Quality: ControlFrame{W: 10000}, Speed: ControlFrame{W: 20}, Drawer: ControlFrame{W: 20}})
	app.mu.Lock()
	chrome := app.playerChrome
	app.mu.Unlock()
	if chrome.QualityLabel != "1080P" || chrome.TotalEpisodes != 12 {
		t.Fatalf("valid native player chrome was lost: %+v", chrome)
	}

	// Verify player commands invoke cleanly without panic
	shell.SelectPlayerQuality("720p")
	shell.SelectPlayerSpeed(1.5)
	shell.SelectPlayerEpisode(3)

	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	// WebView hosts media surface directly; no unaligned native overlays steal focus
	if tt.WebViewAt(600, 400) == nil {
		t.Fatal("hosted page should render media-surface webview")
	}
}
