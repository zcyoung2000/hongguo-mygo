package uiapp

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestNativePlayerControlsOverlayAndCommands(t *testing.T) {
	app := NewTestApp()
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
			{ID: "720p", Label: "720P", Available: true},
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
	app.mu.Lock()
	host := app.media
	host.ready = true
	app.mu.Unlock()
	host.mu.Lock()
	host.draining = true
	host.sent = nil
	host.mu.Unlock()
	shell.SelectPlayerQuality("720p")
	shell.SelectPlayerSpeed(1.5)
	shell.SelectPlayerEpisode(3)
	commands := host.SentControls()
	want := []map[string]any{
		{"event": "command", "command": "quality", "value": "720p"},
		{"event": "command", "command": "speed", "value": 1.5},
		{"event": "command", "command": "episode", "value": 3},
	}
	if len(commands) != len(want) {
		t.Fatalf("player commands = %v, want %v", commands, want)
	}
	for index, payload := range want {
		for key, value := range payload {
			if commands[index][key] != value {
				t.Fatalf("player command %d %s = %v, want %v", index, key, commands[index][key], value)
			}
		}
	}

	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	// The WebView remains the media surface; native controls own their exact frames.
	if tt.WebViewAt(600, 400) == nil {
		t.Fatal("hosted page should render media-surface webview")
	}
	if !tt.HasText("清晰度") || !tt.HasText("倍速") || !tt.HasText("选集") {
		t.Fatalf("native controls missing: %v", tt.Texts())
	}
	for _, label := range []string{"清晰度", "倍速", "选集"} {
		_, found := tt.Find(label)
		if !found {
			t.Fatalf("native %s control missing", label)
		}
	}
}

func TestPlayerControlsHaveSingleRuntimeDispatcher(t *testing.T) {
	if _, err := os.Stat("player.go"); err == nil {
		t.Fatal("legacy PlayerBridge implementation remains")
	}
	for _, name := range []string{"player_chrome.go", "service.go", "features.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(data), "PlayerBridge") || strings.Contains(string(data), "player.Play(") {
			t.Fatalf("%s still references the legacy player", name)
		}
	}
}
