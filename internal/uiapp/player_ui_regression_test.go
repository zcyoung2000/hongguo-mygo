package uiapp

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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
	app.playerFixture = true
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

func TestPlayerChromeStaysInsideFittedVideoAndNativeControlsOwnHitTesting(t *testing.T) {
	css, err := os.ReadFile("../../frontend/styles.css")
	if err != nil {
		t.Fatalf("read frontend stylesheet: %v", err)
	}
	if !strings.Contains(string(css), "--stage-chrome-inset: 18px") {
		t.Fatal("player stage must reserve a safe inset between fitted video and action rail")
	}

	app := NewTestApp(nil)
	app.SetMediaHost(&MediaHost{})
	app.TogglePlayerFixture()
	app.detail = Detail{ID: "123456"}
	app.mu.Lock()
	app.view = "detail"
	app.mediaOn = true
	app.mediaSession = 1
	app.playerChrome = PlayerChrome{
		Visible: true, ControlsVisible: true,
		Quality: ControlFrame{X: 410, Y: 536, W: 52, H: 28},
		Speed:   ControlFrame{X: 470, Y: 536, W: 52, H: 28},
		Drawer:  ControlFrame{X: 630, Y: 536, W: 72, H: 28},
	}
	app.playerChromeKnown = true
	app.playerDrawerOpen = false
	app.mu.Unlock()

	tt := ui.NewTester(app.renderPlayerControlsAcceptance, 988, 604)
	tt.SetScale(1)
	tt.Frame()
	if center := tt.WebViewAt(594, 262); center != nil {
		t.Fatal("headless visual fixture must not retain an invisible WebView hole")
	}
	for _, label := range []string{"清晰度", "倍速", "选集"} {
		if err := tt.Click(label); err != nil {
			t.Fatalf("native %s control is not clickable: %v", label, err)
		}
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
	app.SetMediaHost(&MediaHost{})
	app.PlayDetail()
	host := app.media
	if host == nil {
		t.Fatal("media host missing after player start")
	}
	host.ready = true
	host.mu.Lock()
	host.draining = true
	host.sent = nil
	host.mu.Unlock()
	app.mu.Lock()
	app.mediaOn = true
	app.mediaSession = 1
	app.playerFixture = true
	app.resume = CatalogItem{ID: "812", ResumeEpisode: 22}
	app.playerChrome = PlayerChrome{Visible: true, ControlsVisible: true, Episode: 22, TotalEpisodes: 812}
	app.playerChrome.Quality = ControlFrame{X: 410, Y: 536, W: 52, H: 28}
	app.playerChrome.Speed = ControlFrame{X: 470, Y: 536, W: 52, H: 28}
	app.playerChrome.Drawer = ControlFrame{X: 630, Y: 536, W: 72, H: 28}
	app.playerChromeKnown = true
	app.mu.Unlock()

	tt := ui.NewTester(app.Render, 988, 604)
	tt.SetScale(1)
	tt.Frame()
	if err := tt.Click("选集"); err != nil {
		t.Fatalf("native episode button is not clickable: %v", err)
	}
	app.mu.Lock()
	opened := app.playerDrawerOpen
	app.mu.Unlock()
	if !opened {
		t.Fatal("native episode button must open the episode drawer")
	}
	tt.Frame()
	for number := 1; number <= 12; number++ {
		if !tt.HasText(fmt.Sprintf("%d", number)) {
			t.Fatalf("collapsed drawer omitted episode %d: %v", number, tt.Texts())
		}
	}
	if !tt.HasText(fmt.Sprintf("%d", 812)) {
		t.Fatal("episode drawer must expose the continuous virtual collection, not truncate it to 12")
	}
	app.playerDrawerGrid.ScrollIntoView(21)
	tt.Frame()
	target, targetFound := tt.Find("第22集")
	if !targetFound {
		t.Fatal("episode 22 accessible control is missing")
	}
	openScreenshot := writePlayerScreenshot(t, tt, filepath.Join(os.TempDir(), "mygo-player-drawer-812-open.png"))
	t.Logf("open drawer screenshot: %s", openScreenshot)
	tt.ClickAt(target.X+target.W/2, target.Y+target.H/2)
	waitFor(time.Second, func() bool { return len(host.SentControls()) == 1 })
	commands := host.SentControls()
	if len(commands) < 1 || commands[len(commands)-1]["event"] != "command" || commands[len(commands)-1]["command"] != "episode" || commands[len(commands)-1]["value"] != 22 {
		t.Fatalf("episode click sent %v, want episode 22", commands)
	}
	app.mu.Lock()
	closed := !app.playerDrawerOpen
	app.mu.Unlock()
	if !closed {
		t.Fatal("selecting an episode must close the native drawer")
	}

	selectedScreenshot := writePlayerScreenshot(t, tt, filepath.Join(os.TempDir(), "mygo-player-drawer-812-selected.png"))
	t.Logf("selected screenshot: %s", selectedScreenshot)
}

func (a *App) renderPlayerControlsAcceptance(c *ui.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.renderPlayerVisualFixture(c)
	a.renderNativePlayerControls(c)
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
