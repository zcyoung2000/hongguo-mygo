package uiapp

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestNativeFeedChannelSelectionSurvivesNavigationAndStartup(t *testing.T) {
	app := NewTestApp(nil)
	host := &MediaHost{}
	app.SetMediaHost(host)
	app.OpenView("recommend")
	tt := ui.NewTester(app.Render, 1200, 800)
	if err := tt.Click("漫剧"); err != nil {
		t.Fatal(err)
	}
	waitFor(time.Second, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return app.feedChannel == "漫剧" })
	app.mu.Lock()
	selected := app.feedChannel
	app.mu.Unlock()
	if selected != "漫剧" || host.desired["channel"] != "漫剧" {
		t.Fatalf("native selection didn't reach the existing feed: %q %+v", selected, host.desired)
	}
	app.OpenView("discover")
	app.SelectFeedChannel("真人剧")
	app.OpenView("recommend")
	if host.desired["channel"] != "漫剧" {
		t.Fatal("returning to recommend lost the selected channel")
	}
	app.SelectFeedChannel("invalid")
	if host.desired["channel"] != "漫剧" {
		t.Fatal("invalid channel changed the recommendation source")
	}
	host.markReady()
	if len(host.queue) != 1 || host.queue[0]["channel"] != "漫剧" {
		t.Fatal("channel selected before document ready was lost")
	}
}

func TestNativeFeedChannelsOverlayKeepsMediaAndHidesWithItsScene(t *testing.T) {
	app := NewTestApp(nil)
	view := &mygo.WebView{}
	app.SetMediaHost(&MediaHost{view: view})
	app.OpenView("recommend")
	tt := ui.NewTester(app.Render, 1200, 800)
	channels, ok := tt.Find("推荐频道")
	if !ok || tt.WebViewAt(channels.X+channels.W/2, channels.Y+channels.H/2) != nil {
		t.Fatal("native channel control doesn't take pointer input over the WebView")
	}
	if tt.WebViewAt(900, 300) != view || tt.WebViewAt(1199, 799) != view {
		t.Fatal("native channel overlay changes the full media viewport")
	}
	shell := NativeShell{App: app}
	shell.FeedChromeChanged(FeedChrome{Visible: true, RightInset: 420})
	tt.Frame()
	moved, _ := tt.Find("推荐频道")
	if channels.X-moved.X < 200 {
		t.Fatal("channel control didn't move clear of the comment panel")
	}
	shell.FeedChromeChanged(FeedChrome{Visible: false})
	tt.Frame()
	if tt.HasText("推荐频道") {
		t.Fatal("clear-screen/fullscreen state retained native controls")
	}
	shell.FeedChromeChanged(FeedChrome{Visible: true})
	app.detail = Detail{ID: "123456"}
	app.PlayDetail()
	tt.Frame()
	if tt.HasText("推荐频道") {
		t.Fatal("recommendation channels cover the full player")
	}
	app.OpenView("settings")
	tt.Frame()
	if tt.HasText("推荐频道") {
		t.Fatal("recommendation channels cover settings")
	}
}

func TestAmbientGlowUpdatesSidebarAndFeedChrome(t *testing.T) {
	app := NewTestApp(nil)
	shell := NativeShell{App: app}
	app.OpenView("recommend")
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()

	app.mu.Lock()
	if app.ambientActive {
		t.Fatal("ambientActive should be false initially")
	}
	app.mu.Unlock()

	shell.AmbientGlow(AmbientGlowPayload{
		Active: true,
		Left:   [3]uint8{200, 50, 60},
		Top:    [3]uint8{100, 150, 220},
	})

	app.mu.Lock()
	if !app.ambientActive {
		t.Fatal("ambientActive should be true after AmbientGlow notification")
	}
	app.mu.Unlock()

	tt.Frame()

	shell.AmbientGlow(AmbientGlowPayload{Active: false})
	app.mu.Lock()
	if app.ambientActive {
		t.Fatal("ambientActive should be false after deactivating")
	}
	app.mu.Unlock()
	tt.Frame()
}

func TestFeedChannelPillWidthNotCollapsed(t *testing.T) {
	app := NewTestApp(nil)
	app.feedChannel = "推荐"
	app.OpenView("recommend")
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	elem, ok := tt.Find("推荐")
	if !ok {
		t.Fatal("expected 推荐 channel to be found")
	}
	if elem.W < 50 {
		t.Fatalf("selected channel pill width collapsed: width=%v", elem.W)
	}
}

