package uiapp

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func TestVisualAuditPages(t *testing.T) {
	dir := os.Getenv("MYGUO_VISUAL_AUDIT_DIR")
	if dir == "" {
		t.Skip("set MYGUO_VISUAL_AUDIT_DIR to write visual audit screenshots")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	save := func(t *testing.T, name string, render func(*ui.Context)) {
		t.Helper()
		width, height := 1200, 800
		tt := ui.NewTester(render, width, height)
		tt.SetDark(true)
		tt.Frame()
		file, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, tt.Image()); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	item := CatalogItem{ID: "123456", Title: "视觉审查短剧", Category: "真人剧", Heat: "280万热度", EpisodeCount: 36, Description: "剧情简介", ResumeEpisode: 4, Progress: 42}
	save(t, "discover-recommend", func(c *ui.Context) {
		app := NewTestApp()
		app.view = "recommend"
		app.catalog.Filters = []FilterRow{{Title: "题材", Key: "topic", Options: []FilterOption{{Label: "全部", Value: ""}, {Label: "都市", Value: "1"}}}}
		app.catalog.Items = []CatalogItem{item}
		app.Render(c)
	})
	save(t, "find-search", func(c *ui.Context) {
		app := NewTestApp()
		app.view = "search"
		app.search.Items = []CatalogItem{item}
		app.Render(c)
	})
	save(t, "rank", func(c *ui.Context) {
		app := NewTestApp()
		app.view = "rank"
		app.rank = RankingState{Items: []ipc.RankItemDTO{{ItemType: "drama", SeriesID: "123456", Title: "排行榜短剧", EpisodeCount: 48, Heat: "312万热度", Description: "榜单简介", Position: 1}}}
		app.Render(c)
	})
	save(t, "new-releases", func(c *ui.Context) {
		app := NewTestApp()
		app.view = "new"
		item.Upcoming = true
		item.PublishAt = "20261106"
		app.releases = ReleaseState{Calendar: true, Dates: []string{"20261106"}, Date: "20261106", Items: []CatalogItem{item}}
		app.Render(c)
	})
	for _, tab := range []struct{ name, view string }{{"watch-history", "history"}, {"favorites", "favorites"}, {"likes", "likes"}, {"reservations", "reservations"}} {
		save(t, tab.name, func(c *ui.Context) {
			app := NewTestApp()
			app.view = tab.view
			library := Library{Title: "视觉审查", Items: []CatalogItem{item}}
			switch tab.view {
			case "history":
				app.library.History = library
			case "favorites":
				app.library.Favorites = library
			case "likes":
				app.library.Likes = library
			case "reservations":
				app.library.Reserved = library
			}
			app.Render(c)
		})
	}
	save(t, "settings", func(c *ui.Context) {
		app := NewTestApp()
		app.view = "settings"
		app.Render(c)
	})
	save(t, "player", func(c *ui.Context) {
		app := NewTestApp()
		app.SetMediaHost(&MediaHost{})
		app.view = "detail"
		app.detail = Detail{ID: "123456", Episodes: []CatalogItem{{ID: "1", EpisodeCount: 1}}}
		app.PlayDetail()
		app.mu.Lock()
		app.playerFixture = true
		app.mediaOn = true
		app.mediaSession = 1
		app.playerChrome = PlayerChrome{Visible: true, ControlsVisible: true, QualityLabel: "1080P", SpeedValue: 1, Episode: 1, TotalEpisodes: 2, Quality: ControlFrame{X: 398, Y: 536, W: 52, H: 28}, Speed: ControlFrame{X: 458, Y: 536, W: 52, H: 28}, Drawer: ControlFrame{X: 618, Y: 536, W: 72, H: 28}, QualityOptions: []PlayerQualityOption{{ID: "1080p", Label: "1080P", Available: true, Active: true}}}
		app.playerChromeKnown = true
		app.mu.Unlock()
		app.Render(c)
	})
	save(t, "player-episode-drawer", func(c *ui.Context) {
		app := NewTestApp()
		app.SetMediaHost(&MediaHost{})
		app.view = "detail"
		app.detail = Detail{ID: "123456", Episodes: make([]CatalogItem, 24)}
		for index := range app.detail.Episodes {
			app.detail.Episodes[index] = CatalogItem{ID: fmt.Sprintf("e%d", index+1), EpisodeCount: index + 1}
		}
		app.PlayDetail()
		app.mu.Lock()
		app.playerFixture = true
		app.mediaOn = true
		app.mediaSession = 1
		app.playerDrawerOpen = true
		app.playerChrome = PlayerChrome{Visible: true, ControlsVisible: true, Episode: 1, TotalEpisodes: 24, Quality: ControlFrame{X: 398, Y: 536, W: 52, H: 28}, Speed: ControlFrame{X: 458, Y: 536, W: 52, H: 28}, Drawer: ControlFrame{X: 618, Y: 536, W: 72, H: 28}}
		app.playerChromeKnown = true
		app.mu.Unlock()
		app.Render(c)
	})
}
