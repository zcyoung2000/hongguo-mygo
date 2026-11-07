package uiapp

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func TestNativePlayerControlsOverlayAndCommands(t *testing.T) {
	app := NewTestApp(nil)
	playback := &fakePlayback{playback: &ipc.PlaybackDTO{
		SeriesID: "123456", EpisodeID: "654321", EpisodeNum: 2,
		StreamURL: "local://episode-2", Quality: "1080p", Qualities: []ipc.QualityOptionDTO{
			{ID: "1080p", Label: "1080P", Available: true, Active: true},
			{ID: "720p", Label: "720P", Available: true},
		},
	}}
	app.player = NewTestPlayerBridge()
	app.player.playback = playback
	app.player.Play("123456", "654321", 2, 0, true)
	waitFor(time.Second, func() bool { return app.player.state().StreamURL != "" })
	if app.player.state().StreamURL == "" {
		t.Fatalf("playback did not resolve: %+v", app.player.state())
	}
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

func TestMediaSurfaceClickTogglesPlaybackWithoutSystemMediaForwarding(t *testing.T) {
	playback := &fakePlayback{playback: &ipc.PlaybackDTO{SeriesID: "s", EpisodeID: "e", EpisodeNum: 1, StreamURL: "local://video"}}
	player := NewTestPlayerBridge()
	player.playback = playback
	player.Play("s", "e", 1, 0, true)
	waitFor(time.Second, func() bool { return player.state().StreamURL != "" })

	player.Toggle()
	if player.state().Playing {
		t.Fatalf("toggle did not pause bridge state: %+v", player.state())
	}
	commands := player.commandsForTest()
	if len(commands) == 0 || commands[len(commands)-1].payload["event"] != "pause" {
		t.Fatalf("pause media event missing: %v", commands)
	}
	player.Toggle()
	if !player.state().Playing {
		t.Fatalf("toggle did not resume bridge state: %+v", player.state())
	}
	commands = player.commandsForTest()
	if len(commands) == 0 || commands[len(commands)-1].payload["event"] != "resume" {
		t.Fatalf("resume media event missing: %v", commands)
	}
}

func TestPlayerBridgeControlCommandsReachProviderAndMedia(t *testing.T) {
	playback := &fakePlayback{playback: &ipc.PlaybackDTO{
		SeriesID: "123456", EpisodeID: "654321", EpisodeNum: 2,
		StreamURL: "local://episode-2", Quality: "1080p",
	}}
	player := NewTestPlayerBridge()
	player.playback = playback
	player.testMode = false
	player.Play("123456", "654321", 2, 0, true)
	waitFor(time.Second, func() bool { return player.state().StreamURL != "" })

	player.SelectQuality("720p")
	waitFor(time.Second, func() bool { return playback.quality == "720p" })
	waitFor(time.Second, func() bool { return playback.quality == "720p" && player.state().Quality == "720p" })
	if playback.quality != "720p" {
		t.Fatalf("provider quality = %q, want 720p", playback.quality)
	}
	if state := player.state(); state.Quality != "720p" {
		t.Fatalf("active quality = %q, want 720p", state.Quality)
	}

	player.SelectSpeed(2)
	waitFor(time.Second, func() bool { return player.state().Speed == 2 })
	if state := player.state(); state.Speed != 2 {
		t.Fatalf("active speed = %v, want 2", state.Speed)
	}
	commands := player.commandsForTest()
	if len(commands) == 0 || commands[len(commands)-1].payload["event"] != "speed" || commands[len(commands)-1].payload["value"] != 2.0 {
		t.Fatalf("speed media command missing: %v", commands)
	}
}
