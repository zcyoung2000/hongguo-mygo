package uiapp

import (
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
}
