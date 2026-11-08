package uiapp

import (
	"fmt"
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
	}
	app := NewTestApp()
	app.view = "reservations"
	app.library.Reserved = Library{Loaded: true, Items: items}
	app.library.Grids[3] = &ui.GridState{}
	app.reservationsPending = false
	tester := ui.NewTester(app.Render, 560, 900)
	tester.Frame()
	first, firstOK := tester.Find("我能看到别人的情绪第三季")
	second, secondOK := tester.Find("我就是丧尸之王第二季")
	if !firstOK || !secondOK {
		t.Fatalf("reservation cards missing: %v", tester.Texts())
	}
	rowHeight := float64(second.Y - first.Y)
	if rowHeight <= float64(first.H)+12 {
		t.Fatalf("reservation grid rows overlap: rowHeight=%.1f cardHeight=%.1f first=%+v second=%+v", rowHeight, first.H, first, second)
	}
	cancel, cancelOK := tester.Find("取消预约")
	if !cancelOK {
		t.Fatal("reservation action missing")
	}
	if cancel.Y+cancel.H > second.Y {
		t.Fatalf("action overlaps next row: cancel=%+v nextRow=%.1f", cancel, second.Y)
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
			if actionOK && action.Y+action.H <= title.Y+title.H {
				t.Fatalf("action is clipped by title: title=%+v action=%+v", title, action)
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
