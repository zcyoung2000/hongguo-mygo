package uiapp

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestReservationRowsHaveSpaceForAllMetadata(t *testing.T) {
	items := []CatalogItem{
		{ID: "one", EpisodeID: "e1", Title: "我能看到别人的情绪第三季", Upcoming: false, PublishAt: "2026-11-07"},
		{ID: "two", EpisodeID: "e2", Title: "我就是丧尸之王第二季", Upcoming: false, PublishAt: "2026-10-25"},
		{ID: "three", EpisodeID: "e3", Title: "十八岁的二次回响第二季", Upcoming: false, PublishAt: "2026-10-25"},
		{ID: "four", EpisodeID: "e4", Title: "下一行卡片标题", Upcoming: false, PublishAt: "2026-12-01"},
		{ID: "five", EpisodeID: "e5", Title: "同行补充卡片", Upcoming: false, PublishAt: "2026-12-02"},
	}
	app := NewTestApp()
	app.view = "reservations"
	app.library.Reserved = Library{Loaded: true, Items: items[:4]}
	app.library.Grids[3] = &ui.GridState{}
	app.reservationsPending = false
	tester := ui.NewTester(app.Render, 1000, 900)
	tester.Frame()
	app.library.Reserved = Library{Loaded: true, Items: items}
	app.library.Grids[3] = &ui.GridState{}
	tester.Frame()
	if !tester.HasText("我能看到别人的情绪第三季") || !tester.HasText("下一行卡片标题") {
		t.Fatalf("reservation cards missing: %v", tester.Texts())
	}
	date, dateOK := tester.Find("2026-11-07")
	nextPoster, nextPosterOK := tester.Find("poster:five|e5")
	if !dateOK || !nextPosterOK {
		t.Fatalf("reservation row bounds missing: date=%v poster=%v texts=%v", dateOK, nextPosterOK, tester.Texts())
	}
	rowGap := float64(nextPoster.Y - (date.Y + date.H))
	if rowGap < 20 || rowGap > 28 {
		t.Fatalf("reservation row gap = %.1f, want 20-28: date=%+v nextPoster=%+v", rowGap, date, nextPoster)
	}
	cancel, cancelOK := tester.Find("取消预约")
	if !cancelOK {
		t.Fatal("reservation action missing")
	}
	if cancel.Y+cancel.H > nextPoster.Y {
		t.Fatalf("action overlaps next row: cancel=%+v nextRow=%.1f", cancel, nextPoster.Y)
	}
	if dir := os.Getenv("MYGUO_NATIVE_SCREENSHOT_DIR"); dir != "" {
		path := filepath.Join(dir, "reservation-row-spacing.png")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, tester.Image()); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("saved screenshot: %s", path)
	}
}

func TestPosterActionDoesNotOpenCard(t *testing.T) {
	app := NewTestApp()
	provider := &mutationProvider{}
	app.libraryMutations = provider
	app.view = "favorites"
	item := CatalogItem{ID: "favorite-1", EpisodeID: "episode-1", Title: "浮层操作剧", EpisodeCount: 10}
	app.library.Favorites = Library{Loaded: true, Items: []CatalogItem{item}}
	app.library.Grids[1] = &ui.GridState{}
	tester := ui.NewTester(app.Render, 1000, 900)
	tester.Frame()
	if err := tester.Click("取消收藏"); err != nil {
		t.Fatalf("poster action was not clickable: %v", err)
	}
	tester.Frame()
	if got := app.view; got != "favorites" {
		t.Fatalf("poster action opened the card: view=%s", got)
	}
	if len(provider.calls) != 1 || provider.calls[0] != "favorite:favorite-1" {
		t.Fatalf("poster action did not call removal exactly once: %+v", provider.calls)
	}
}

func TestEveryLibraryCollectionFitsExtremeRowsAndActions(t *testing.T) {
	longTitle := strings.Repeat("超长中文短剧标题用于验证虚拟网格真实高度", 10)
	tests := []struct {
		name    string
		view    string
		grid    int
		pending bool
	}{
		{name: "history", view: "history", grid: 0},
		{name: "favorites", view: "favorites", grid: 1},
		{name: "likes", view: "likes", grid: 2},
		{name: "reservations", view: "reservations", grid: 3, pending: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := make([]CatalogItem, 1001)
			for index := range items {
				items[index] = CatalogItem{
					ID: fmt.Sprintf("%s-%04d", test.name, index), EpisodeID: fmt.Sprintf("episode-%04d", index),
					Title: longTitle, EpisodeCount: 80, ResumeEpisode: index%80 + 1,
					UpdatedAt: 1760000000, PublishAt: "2099-12-31", Upcoming: test.pending,
				}
			}
			app := NewTestApp()
			app.SetInvalidator(func() {})
			app.view = test.view
			app.reservationsPending = test.pending
			library := Library{Loaded: true, Items: items}
			switch test.view {
			case "history":
				app.library.History = library
			case "favorites":
				app.library.Favorites = library
			case "likes":
				app.library.Likes = library
			case "reservations":
				app.library.Reserved = library
			}
			app.refreshVisibleLibraryLocked(LibraryTab(test.grid))
			if state := app.library.Grids[test.grid]; state != nil {
				state.Key = func(item int) any { return items[item].ID + "|" + items[item].EpisodeID }
				state.Label = func(item int) string { return items[item].Title }
			}
			tester := ui.NewTester(app.Render, 1000, 640)
			tester.SetSize(1000, 640)
			tester.Frame()
			title, titleOK := tester.Find("single-line-title")
			actionLabel := map[string]string{"history": "", "favorites": "取消收藏", "likes": "取消点赞", "reservations": "取消预约"}[test.view]
			action, actionOK := tester.Find(actionLabel)
			if !titleOK || !actionOK && actionLabel != "" {
				t.Fatalf("extreme row content missing: title=%v action=%v texts=%s", titleOK, actionOK, strings.Join(tester.Texts(), " | "))
			}
			if title.Y < 0 || title.Y+title.H > 640 || action.Y < 0 || action.Y+action.H > 640 {
				t.Fatalf("first row leaves viewport: title=%+v action=%+v", title, action)
			}
			if actionOK {
				posterLabel := fmt.Sprintf("poster:%s-0000|episode-0000", test.view)
				poster, posterOK := tester.Find(posterLabel)
				if !posterOK {
					t.Fatalf("action poster missing: action=%+v texts=%s", action, strings.Join(tester.Texts(), " | "))
				}
				if action.Y < poster.Y || action.Y+action.H > poster.Y+poster.H {
					t.Fatalf("poster action leaves poster: poster=%+v action=%+v", poster, action)
				}
			}
			if visible := app.visibleLibrary(LibraryTab(test.grid)); len(visible) != len(items) {
				t.Fatalf("visible source did not retain %d rows", len(items))
			}
			if snapshot := app.libraryAt(LibraryTab(test.grid)).Items; len(snapshot) != len(items) {
				t.Fatalf("library snapshot was rewritten by render")
			}
		})
	}
}
