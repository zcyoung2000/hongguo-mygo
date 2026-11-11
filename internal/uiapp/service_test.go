package uiapp

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/catalog"
	"myguo/internal/ipc"
)

func TestCatalogValuesUsesSelectedFiltersAndDefaultsToGenre(t *testing.T) {
	rows := []FilterRow{
		{Key: "topic", Options: []FilterOption{{Value: ""}, {Value: "love"}}},
		{Key: "status", Options: []FilterOption{{Value: ""}, {Value: "hot"}}},
	}
	got := catalogValues("short_play", rows, []int{0, 1})
	if got["status"] != "hot" || got["genre"] != "" {
		t.Fatalf("catalogValues = %v", got)
	}
	empty := catalogValues("comic_series", rows, nil)
	if empty["genre"] != "comic_series" || len(empty) != 1 {
		t.Fatalf("default selections = %v", empty)
	}
}

func TestSelectGenreResetsCatalogAndTriggersAsyncLoad(t *testing.T) {
	app := NewTestApp()
	app.SetInvalidator(func() {})
	host := &MediaHost{}
	app.SetMediaHost(host)
	app.catalog.Items = []CatalogItem{{ID: "old"}}
	app.catalog.Cursor = "cursor"
	app.catalog.HasMore = true

	app.SelectGenre(1)
	if app.catalog.GenreIndex != 1 || len(app.catalog.Items) != 0 || app.catalog.Cursor != "" || !app.catalog.HasMore {
		t.Fatalf("catalog was not reset: %+v", app.catalog)
	}
	if app.catalogGenre() != "comic_series" {
		t.Fatalf("genre = %q", app.catalogGenre())
	}
}

func TestOpenDetailShowsLoadingWithoutPlayer(t *testing.T) {
	app := NewTestApp()
	app.SetInvalidator(func() {})
	host := &MediaHost{}
	app.SetMediaHost(host)
	app.OpenDetail("123456")
	if app.View() != "detail" || app.PlayerVisible() || !app.DetailState().Loading {
		t.Fatalf("detail state = view %q player %t detail %+v", app.View(), app.PlayerVisible(), app.DetailState())
	}
	app.OpenView("discover")
	if app.PlayerVisible() || app.DetailState().ID != "" {
		t.Fatalf("navigation did not clear detail/player: %+v", app)
	}
}

func TestLibraryGridsAreStablePerTab(t *testing.T) {
	app := NewTestApp()
	grids := [4]*ui.GridState{{}, {}, {}, {}}
	app.SetLibraryGrids(grids)
	app.SetLibraryGrids(grids)
	if app.library.Grids != grids {
		t.Fatal("library grids were replaced")
	}
}

func TestConvertCardsPreservesVisibleCatalogMetadata(t *testing.T) {
	items := convertCards([]catalog.SeriesCard{{
		ID: "123456", Title: "测试剧", Description: "剧情", CoverURL: "proxy", EpisodeCount: 8,
		Category: "真人剧", Heat: "热播", Tags: []string{"甜宠"},
	}})
	if len(items) != 1 || items[0].ID != "123456" || items[0].Title != "测试剧" || items[0].EpisodeCount != 8 || items[0].Subtitles[0] != "甜宠" {
		t.Fatalf("convertCards = %+v", items)
	}
}

func TestDetailProviderIsInvokedDirectly(t *testing.T) {
	source := &fakeDetail{page: &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "详情"}}}
	app := NewTestApp()
	app.detailSource = source
	app.loadDetail("123456")
	if source.pages.Load() != 1 || app.DetailState().Title != "详情" {
		t.Fatalf("detail provider calls=%d state=%+v", source.pages.Load(), app.DetailState())
	}
}

func TestDetailProgressReconcileFeedsResume(t *testing.T) {
	app := NewTestApp()
	app.SetInvalidator(func() {})
	host := &MediaHost{}
	app.SetMediaHost(host)
	app.detailSource = &fakeDetail{page: &ipc.SeriesDetailPageDTO{
		SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "详情"},
		ResumeEpisode:   8,
		ResumePosition:  72.5,
	}}
	app.loadDetail("123456")
	if app.resume.ResumeEpisode != 8 || app.resume.ResumePosition != 72.5 {
		t.Fatalf("resume after detail = %+v", app.resume)
	}
	app.detail = Detail{ID: "123456", EpisodeCount: 10}
	app.PlayDetail()
	if host.desired["resumeAt"] != 72.5 || host.desired["episodeNum"] != 8 {
		t.Fatalf("play scene = %#v, want reconciled resume", host.desired)
	}
}

func TestFavoriteProviderUpdatesState(t *testing.T) {
	provider := &fakeFavorite{favorite: true, reserved: true}
	app := NewTestApp()
	app.favorite = provider
	app.detail = Detail{ID: "123456", Title: "详情", EpisodeCount: 8}
	app.ToggleDetailFavorite()
	waitFor(time.Second, func() bool { return provider.favoriteCalls.Load() == 1 && app.DetailState().Favorite })
	if provider.favoriteCalls.Load() != 1 || !app.DetailState().Favorite {
		t.Fatalf("favorite calls=%d state=%+v", provider.favoriteCalls.Load(), app.detail)
	}
	app.ToggleDetailReservation()
	waitFor(time.Second, func() bool { return provider.reservationCalls.Load() == 1 && app.DetailState().Reserved })
	if provider.reservationCalls.Load() != 1 || !app.DetailState().Reserved {
		t.Fatalf("reservation calls=%d state=%+v", provider.reservationCalls.Load(), app.detail)
	}
}

func waitFor(timeout time.Duration, ready func() bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
}
