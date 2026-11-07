package uiapp

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestPlayerDrawerDefaultsToThreeRowsAndClosesOnOutsideClick(t *testing.T) {
	app := NewTestApp(nil)
	app.SetMediaHost(&MediaHost{view: &mygo.WebView{}})
	app.view = "detail"
	app.detail = Detail{ID: "123456"}
	app.PlayDetail()
	app.mu.Lock()
	app.mediaOn = true
	app.mediaSession = 1
	app.playerChrome = PlayerChrome{
		Visible: true, ControlsVisible: true, Episode: 55, TotalEpisodes: 524,
		Quality: ControlFrame{X: 500, Y: 700, W: 52, H: 28},
		Speed:   ControlFrame{X: 560, Y: 700, W: 52, H: 28},
		Drawer:  ControlFrame{X: 760, Y: 700, W: 72, H: 28},
	}
	app.playerChromeKnown = true
	app.playerDrawerOpen = true
	app.mu.Unlock()

	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	if !tt.HasText("选集播放") || !tt.HasText("共 524 集") || !tt.HasText("第55集") {
		t.Fatalf("episode drawer missing: %v", tt.Texts())
	}
	if tt.HasText("第16集") {
		t.Fatalf("collapsed episode drawer rendered beyond three rows: %v", tt.Texts())
	}
	if tt.HasText("展开全部") {
		t.Fatalf("native drawer unexpectedly rendered a web-style expansion toggle: %v", tt.Texts())
	}

	app.mu.Lock()
	app.playerDrawerOpen = true
	app.mu.Unlock()
	tt.ClickAt(600, 700)
	app.mu.Lock()
	closed := !app.playerDrawerOpen
	app.mu.Unlock()
	if !closed {
		t.Fatal("clicking the media surface outside the drawer must close it")
	}

	app.mu.Lock()
	app.playerDrawerOpen = true
	app.mu.Unlock()
	tt.Frame()
	tt.ClickAt(300, 700)
	app.mu.Lock()
	closed = !app.playerDrawerOpen
	app.mu.Unlock()
	if !closed {
		t.Fatal("clicking modal content outside the drawer must close it")
	}
}

func TestPlayerBackButtonMatchesDetailNavigationLayout(t *testing.T) {
	css, err := os.ReadFile("../../frontend/styles.css")
	if err != nil {
		t.Fatalf("read frontend stylesheet: %v", err)
	}
	topbar := regexp.MustCompile(`(?m)(?s)^\.player-topbar\s*\{[^}]*\}`).FindString(string(css))
	back := regexp.MustCompile(`(?m)(?s)^\.player-back-btn\s*\{[^}]*\}`).FindString(string(css))
	if !strings.Contains(topbar, "padding: max(0px, env(safe-area-inset-top)) 24px 0 16px") {
		t.Fatalf("player topbar ignored window safe area or legacy traffic-light inset: %q", topbar)
	}
	for _, token := range []string{"min-height: 32px", "gap: 7px", "font-size: 12px"} {
		if !strings.Contains(back, token) {
			t.Fatalf("player back button does not match detail navigation token %q: %q", token, back)
		}
	}
}
