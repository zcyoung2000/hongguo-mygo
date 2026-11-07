package uiapp

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
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
	if !tt.HasText("选集播放") || !tt.HasText("共 524 集") {
		t.Fatalf("episode drawer missing: %v", tt.Texts())
	}
	if !tt.HasText("第12集") {
		t.Fatalf("collapsed episode drawer omitted the first virtual viewport: %v", tt.Texts())
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

func TestPlayerChromeStaysInsideFittedVideoAndHasNoDuplicateNativeControls(t *testing.T) {
	css, err := os.ReadFile("../../frontend/styles.css")
	if err != nil {
		t.Fatalf("read frontend stylesheet: %v", err)
	}
	if !strings.Contains(string(css), "--stage-chrome-inset: 18px") {
		t.Fatal("player stage must reserve a safe inset between fitted video and action rail")
	}

	app := NewTestApp(nil)
	view := &mygo.WebView{}
	app.SetMediaHost(&MediaHost{view: view})
	app.detail = Detail{ID: "123456"}
	app.mu.Lock()
	app.view = "detail"
	app.mediaOn = true
	app.mediaSession = 1
	app.playerChrome = PlayerChrome{Visible: true, ControlsVisible: true}
	app.playerChromeKnown = true
	app.mu.Unlock()

	tt := ui.NewTester(app.Render, 988, 604)
	tt.SetScale(1)
	tt.Frame()
	center := tt.WebViewAt(594, 262)
	if center == nil || tt.WebViewAt(594, 560) == nil || tt.WebViewAt(260, 560) == nil {
		t.Fatal("playback surface must own the fitted video and control bar, with no duplicate native layer")
	}
	if tt.HasText("清晰度") || tt.HasText("倍速") || tt.HasText("选集") {
		t.Fatalf("native controls duplicate WebView chrome: %v", tt.Texts())
	}

	screenshot := writePlayerScreenshot(t, tt, filepath.Join(os.TempDir(), "mygo-player-right-rail-fixed.png"))
	t.Logf("player screenshot: %s", screenshot)
}

func TestPlayerDrawer812EpisodesPreservesCompleteContinuousCollection(t *testing.T) {
	episodes := make([]CatalogItem, 812)
	for index := range episodes {
		episodes[index] = CatalogItem{ID: "812", EpisodeCount: index + 1}
	}
	app := NewTestApp(nil)
	app.detail = Detail{ID: "812", Episodes: episodes}
	if count := detailEpisodeCollapsedCount(len(episodes)); count != 12 {
		t.Fatalf("collapsed episode count = %d, want 12", count)
	}
	visible := app.detailEpisodeCollapsedIndexes(episodes)
	if len(visible) != 12 {
		t.Fatalf("collapsed visible count = %d, want 12", len(visible))
	}
	for index, expected := range visible {
		if expected != index {
			t.Fatalf("collapsed episode order broken at slot %d: got episode %d", index, expected+1)
		}
	}
	for _, episode := range episodes {
		if episode.EpisodeCount == 0 {
			t.Fatal("episode collection must remain complete")
		}
	}
	if episodes[21].EpisodeCount != 22 || episodes[811].EpisodeCount != 812 {
		t.Fatalf("episode 22 or 812 was replaced: 22=%d 812=%d", episodes[21].EpisodeCount, episodes[811].EpisodeCount)
	}
	app.detail.Episodes = episodes
	app.view = "detail"
	app.PlayDetail()
	app.mu.Lock()
	app.mediaOn = true
	app.mediaSession = 1
	app.resume = CatalogItem{ID: "812", ResumeEpisode: 22}
	app.playerChrome = PlayerChrome{Visible: true, ControlsVisible: true, Episode: 22, TotalEpisodes: 812}
	app.playerChromeKnown = true
	app.mu.Unlock()

	tt := ui.NewTester(app.Render, 988, 604)
	tt.SetScale(1)
	tt.Frame()
	if _, found := tt.Find("继续观看第 22 集"); !found {
		t.Fatal("resume metadata missing")
	}
	if _, found := tt.Find("22"); found {
		t.Fatal("collapsed episode list must render only the first three rows, not substitute episode 22")
	}
	for number := 1; number <= 12; number++ {
		if !tt.HasText(fmt.Sprintf("%d", number)) {
			t.Fatalf("collapsed drawer omitted episode %d: %v", number, tt.Texts())
		}
	}

	screenshot := writePlayerScreenshot(t, tt, filepath.Join(os.TempDir(), "mygo-player-drawer-812-fixed.png"))
	t.Logf("drawer screenshot: %s", screenshot)
}

func writePlayerScreenshot(t *testing.T, tt *ui.Tester, path string) string {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create player screenshot: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, tt.Image()); err != nil {
		t.Fatalf("encode player screenshot: %v", err)
	}
	return path
}
