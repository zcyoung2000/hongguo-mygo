package uiapp

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func scrollInputTester(app *App, width, height int) *ui.Tester {
	return ui.NewTesterWith(app.Render, width, height, ui.TesterOptions{
		Preferences: ui.Preferences{ReduceMotion: true, TextScale: 1},
	})
}

func TestDiscoverWheelInputScrollsPageAndCollapsesFilters(t *testing.T) {
	rows := make([]FilterRow, 3)
	for index := range rows {
		rows[index] = FilterRow{
			Title:   fmt.Sprintf("滚轮维度%d", index+1),
			Options: []FilterOption{{Label: "全部", Value: ""}, {Label: "条件", Value: "1"}},
		}
	}
	items := make([]CatalogItem, 40)
	for index := range items {
		items[index] = CatalogItem{
			ID:        fmt.Sprintf("wheel-discover-%d", index),
			EpisodeID: "1",
			Title:     fmt.Sprintf("滚轮找剧%d", index+1),
		}
	}

	app := NewTestApp()
	app.catalog.Filters = rows
	app.catalog.Items = items
	app.view = "discover"
	tt := scrollInputTester(app, 1600, 1000)
	tt.Frame()

	page, ok := tt.AccessByRole(ui.RoleGroup, "discover-page-scroll")
	if !ok {
		t.Fatal("discover scroll container missing before wheel")
	}
	tt.Wheel(page.Bounds.X+page.Bounds.W/2, page.Bounds.Y+20, 0, 120)
	if app.discoverPageScroll.Y != 120 {
		t.Fatalf("discover wheel did not scroll by the event delta: y=%v want=120", app.discoverPageScroll.Y)
	}
	tt.Frame()
	if app.discoverFilterState != discoverCollapsed {
		t.Fatalf("discover wheel did not collapse filters: state=%v", app.discoverFilterState)
	}
	for index := range rows {
		if tt.HasText(rows[index].Title) {
			t.Fatalf("collapsed discover still shows filter row %q", rows[index].Title)
		}
	}
}

func TestRankWheelInputScrollsList(t *testing.T) {
	items := make([]ipc.RankItemDTO, 80)
	for index := range items {
		items[index] = ipc.RankItemDTO{
			Position: index + 1,
			Title:    fmt.Sprintf("滚轮榜单%d", index+1),
			SeriesID: fmt.Sprintf("rank-%d", index+1),
		}
	}

	app := NewTestApp()
	app.rank.Items = items
	app.view = "rank"
	tt := scrollInputTester(app, 1200, 800)
	tt.Frame()
	if app.rankScroll.MaxY <= 120 {
		t.Fatalf("rank list has too little travel for a wheel event: max=%v", app.rankScroll.MaxY)
	}

	first, ok := tt.Find("滚轮榜单1")
	if !ok {
		t.Fatalf("rank item missing before wheel: %v", tt.Texts())
	}
	tt.Wheel(first.X+first.W/2, first.Y+first.H/2, 0, 120)
	if app.rankScroll.Y != 120 {
		t.Fatalf("rank wheel did not scroll by the event delta: y=%v want=120", app.rankScroll.Y)
	}
}

func TestDetailWideWheelInputScrollsColumnsIndependently(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = detailWideFixture()
	app.sidebarWidth = 0
	tt := scrollInputTester(app, 1600, 1000)
	tt.Frame()
	if app.detailLeftScroll.MaxY <= 1 || app.detailEpisodesScroll.MaxY <= 1 {
		t.Fatalf("detail columns cannot accept wheel events: left-max=%v right-max=%v", app.detailLeftScroll.MaxY, app.detailEpisodesScroll.MaxY)
	}

	seasons, seasonsOK := tt.Find("同系列短剧")
	episodes, episodesOK := tt.Find("第1集")
	if !seasonsOK || !episodesOK {
		t.Fatalf("detail wheel landmarks missing: seasons=%v episodes=%v", seasonsOK, episodesOK)
	}
	leftDelta := min(120, app.detailLeftScroll.MaxY)
	tt.Wheel(seasons.X+seasons.W/2, seasons.Y+seasons.H/2, 0, leftDelta)
	if app.detailLeftScroll.Y != leftDelta {
		t.Fatalf("left detail wheel did not scroll by the event delta: y=%v want=%v", app.detailLeftScroll.Y, leftDelta)
	}
	if app.detailEpisodesScroll.Y != 0 {
		t.Fatalf("left detail wheel changed the right column: y=%v", app.detailEpisodesScroll.Y)
	}

	tt.Frame()
	episodes, ok := tt.Find("第1集")
	if !ok {
		t.Fatal("right detail landmark disappeared after left-column wheel")
	}
	rightDelta := min(120, app.detailEpisodesScroll.MaxY)
	tt.Wheel(episodes.X+episodes.W/2, episodes.Y+episodes.H/2, 0, rightDelta)
	if app.detailEpisodesScroll.Y != rightDelta {
		t.Fatalf("right detail wheel did not scroll by the event delta: y=%v want=%v", app.detailEpisodesScroll.Y, rightDelta)
	}
	if app.detailLeftScroll.Y != leftDelta {
		t.Fatalf("right detail wheel changed the left column: y=%v want=%v", app.detailLeftScroll.Y, leftDelta)
	}
}
