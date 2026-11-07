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
	app := NewTestApp(nil)
	app.SetInvalidator(func() {})
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

func TestOpenDetailShowsLoadingAndPlayerBridge(t *testing.T) {
	app := NewTestApp(NewTestPlayerBridge())
	app.SetInvalidator(func() {})
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
	app := NewTestApp(nil)
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

func TestPlayerBridgeRejectsInvalidProgressAndTracksCommands(t *testing.T) {
	source := &fakePlayback{playback: &ipc.PlaybackDTO{SeriesID: "123456", EpisodeID: "654321", EpisodeNum: 2}}
	player := NewTestPlayerBridge()
	player.playback = source
	if err := player.SaveProgress(progressPayload{SeriesID: "123", EpisodeID: "", Position: 1, Duration: 2}); err != nil {
		t.Fatalf("SaveProgress returned error for empty episode: %v", err)
	}
	player.Play("123456", "654321", 2, 12.5, true)
	state := player.state()
	if state.SeriesID != "123456" || state.EpisodeID != "654321" || state.EpisodeNum != 2 || state.Position != 12.5 || !state.Playing {
		t.Fatalf("play state = %+v", state)
	}
	player.MediaCommand("pause", 0)
	if player.state().Playing {
		t.Fatal("pause did not update bridge state")
	}
	player.MediaCommand("next", 0)
	if player.state().EpisodeNum != 3 || player.state().Playing {
		t.Fatalf("next state = %+v", player.state())
	}
}

func TestPlayerBridgeWithNilProviderDoesNotCallService(t *testing.T) {
	player := NewTestPlayerBridge()
	player.Play("123456", "654321", 2, 12.5, true)
	if player.state().EpisodeNum != 2 {
		t.Fatalf("state = %+v", player.state())
	}
}

func TestDetailProviderIsInvokedDirectly(t *testing.T) {
	source := &fakeDetail{page: &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "详情"}}}
	app := NewTestApp(nil)
	app.detailSource = source
	app.loadDetail("123456")
	if source.pages.Load() != 1 || app.DetailState().Title != "详情" {
		t.Fatalf("detail provider calls=%d state=%+v", source.pages.Load(), app.DetailState())
	}
}

func TestFavoriteProviderUpdatesState(t *testing.T) {
	provider := &fakeFavorite{favorite: true, reserved: true}
	app := NewTestApp(nil)
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

func TestHistoryClearUsesProvider(t *testing.T) {
	clearer := &fakeHistoryClear{}
	app := NewTestApp(nil)
	app.historyClear = clearer
	app.ClearHistory()
	waitFor(time.Second, func() bool { return clearer.calls.Load() == 1 })
	if clearer.calls.Load() != 1 {
		t.Fatalf("clear calls=%d", clearer.calls.Load())
	}
}

func TestPlayerBridgeUsesPlaybackProvider(t *testing.T) {
	source := &fakePlayback{playback: &ipc.PlaybackDTO{
		SeriesID: "123456", EpisodeID: "654321", EpisodeNum: 2, Title: "第2集",
		StreamURL: "http://127.0.0.1/video", LastPosition: 8, DurationSec: 100,
	}}
	player := NewTestPlayerBridge()
	player.playback = source
	player.Play("123456", "", 2, 0, true)
	waitFor(time.Second, func() bool { return source.calls.Load() == 1 && player.state().Duration == 100 })
	if source.calls.Load() != 1 {
		t.Fatalf("playback calls=%d", source.calls.Load())
	}
	state := player.state()
	if state.EpisodeID != "654321" || state.Position != 8 || state.Duration != 100 {
		t.Fatalf("bridge state=%+v", state)
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
