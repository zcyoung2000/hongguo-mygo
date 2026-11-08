package uiapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"myguo/internal/ipc"
)

type fakeLibrary struct {
	history      []ipc.HistoryItem
	favorites    []ipc.FavoriteItem
	reservations []ipc.ReservationItem
	likes        []ipc.LikeItem
	err          error
}

func (f *fakeLibrary) GetHistory(context.Context) ([]ipc.HistoryItem, error) {
	return f.history, f.err
}
func (f *fakeLibrary) GetFavorites(context.Context) ([]ipc.FavoriteItem, error) {
	return f.favorites, f.err
}
func (f *fakeLibrary) GetReservations(context.Context) ([]ipc.ReservationItem, error) {
	return f.reservations, f.err
}
func (f *fakeLibrary) GetLikes(context.Context) ([]ipc.LikeItem, error) {
	return f.likes, f.err
}

func TestLoadHistoryMapsOnlyRealLocalRecords(t *testing.T) {
	provider := &fakeLibrary{history: []ipc.HistoryItem{{
		SeriesID: "123456", Title: "真实历史", CoverURL: "cover", EpisodeNum: 4,
		PositionSec: 42, DurationSec: 120, TotalEpisodes: 10, UpdatedAt: time.Now().Unix(),
	}}}
	app := NewTestApp()
	app.libraryData = provider
	app.SetInvalidator(func() {})
	app.loadLibrary(libraryHistory)
	if app.library.History.Loading || app.library.History.Error != "" || len(app.library.History.Items) != 1 {
		t.Fatalf("history state = %+v", app.library.History)
	}
	item := app.library.History.Items[0]
	if item.ID != "123456" || item.Title != "真实历史" || item.ResumeEpisode != 4 || item.Progress != 35 || item.Rank != 0 {
		t.Fatalf("history item = %+v", item)
	}
}

func TestLoadFavoritesAndReservationsUseProviderData(t *testing.T) {
	provider := &fakeLibrary{
		favorites:    []ipc.FavoriteItem{{SeriesID: "223456", Title: "收藏剧", CoverURL: "cover", TotalEpisodes: 9}},
		reservations: []ipc.ReservationItem{{SeriesID: "323456", Title: "预约剧", Note: "待上线"}},
		likes:        []ipc.LikeItem{{SeriesID: "423456", Title: "点赞剧集", EpisodeNum: 3}},
	}
	app := NewTestApp()
	app.libraryData = provider
	app.SetInvalidator(func() {})
	app.loadLibrary(libraryFavorites)
	if len(app.library.Favorites.Items) != 1 || app.library.Favorites.Items[0].Title != "收藏剧" {
		t.Fatalf("favorites = %+v", app.library.Favorites)
	}
	app.loadLibrary(libraryReservations)
	if len(app.library.Reserved.Items) != 1 || app.library.Reserved.Items[0].Title != "预约剧" {
		t.Fatalf("reservations = %+v", app.library.Reserved)
	}
	app.loadLibrary(libraryLikes)
	if len(app.library.Likes.Items) != 1 || app.library.Likes.Items[0].Title != "点赞剧集" {
		t.Fatalf("likes = %+v", app.library.Likes)
	}
}

func TestLoadLibraryShowsErrorWithoutSyntheticRecords(t *testing.T) {
	provider := &fakeLibrary{err: errors.New("sqlite unavailable")}
	app := NewTestApp()
	app.libraryData = provider
	app.SetInvalidator(func() {})
	app.loadLibrary(libraryHistory)
	if app.library.History.Loading || app.library.History.Error == "" || len(app.library.History.Items) != 0 {
		t.Fatalf("error history = %+v", app.library.History)
	}
}

func TestCompletedHistoryRequiresRealFinalEpisodeProgress(t *testing.T) {
	rows, err := convertHistory([]ipc.HistoryItem{
		{SeriesID: "710000000000001", ContentType: 1, EpisodeNum: 2, TotalEpisodes: 2, PositionSec: 96, DurationSec: 100},
		{SeriesID: "710000000000002", EpisodeNum: 2, TotalEpisodes: 2, PositionSec: 96},
		{SeriesID: "710000000000003", EpisodeNum: 1, TotalEpisodes: 2, PositionSec: 100, DurationSec: 100},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := NewTestApp()
	app.library.History = Library{Items: rows}
	app.SetLibraryFilter("completed", "")
	completed := app.visibleLibrary(libraryHistory)
	if len(completed) != 1 || completed[0].ID != "710000000000001" {
		t.Fatalf("incorrect completed classification: %+v", completed)
	}
	app.SetLibraryFilter("unwatched", "")
	if len(app.visibleLibrary(libraryHistory)) != 2 {
		t.Fatal("unknown duration or intermediate episode classified as completed")
	}
}

// A slow favorites read must not prevent the shared header from learning likes.
type delayedCollections struct {
	fakeLibrary
	favoritesStarted chan struct{}
	likesStarted     chan struct{}
	releaseFavorites chan struct{}
}

func (f *delayedCollections) GetFavorites(context.Context) ([]ipc.FavoriteItem, error) {
	close(f.favoritesStarted)
	<-f.releaseFavorites
	return f.favorites, f.err
}
func (f *delayedCollections) GetLikes(context.Context) ([]ipc.LikeItem, error) {
	close(f.likesStarted)
	return f.likes, f.err
}

func TestEnteringEitherCollectionLoadsBothCounts(t *testing.T) {
	for _, tab := range []LibraryTab{libraryFavorites, libraryLikes} {
		app := NewTestApp()
		provider := &delayedCollections{
			fakeLibrary: fakeLibrary{
				favorites: []ipc.FavoriteItem{{SeriesID: "123456"}},
				likes:     []ipc.LikeItem{{SeriesID: "123456", EpisodeID: "654321"}},
			},
			favoritesStarted: make(chan struct{}), likesStarted: make(chan struct{}), releaseFavorites: make(chan struct{}),
		}
		app.libraryData = provider
		if libraryCountLabel(app.library.Likes) != "…" {
			t.Fatal("unloaded likes reported as zero")
		}
		done := make(chan struct{})
		go func() { app.loadLibrary(tab); close(done) }()
		for _, started := range []chan struct{}{provider.favoritesStarted, provider.likesStarted} {
			select {
			case <-started:
			case <-time.After(time.Second):
				close(provider.releaseFavorites)
				t.Fatal("both collections must start without waiting for the other")
			}
		}
		close(provider.releaseFavorites)
		<-done
		if libraryCountLabel(app.library.Favorites) != "1" || libraryCountLabel(app.library.Likes) != "1" {
			t.Fatalf("first entry counts: %+v", app.library)
		}
	}
}

func TestCollectionCountDistinguishesEmptyFailedAndCached(t *testing.T) {
	for _, tc := range []struct {
		library Library
		want    string
	}{
		{Library{Loaded: true, Empty: true}, "0"},
		{Library{Error: "read failed"}, "—"},
		{Library{Loaded: true, Error: "refresh failed", Items: []CatalogItem{{ID: "123456"}}}, "1"},
	} {
		if got := libraryCountLabel(tc.library); got != tc.want {
			t.Fatalf("count = %q, want %q", got, tc.want)
		}
	}
}

func TestVisibleLibraryIsTheSingleSourceForCountsAndGrid(t *testing.T) {
	items := []CatalogItem{
		{ID: "favorite-1", Title: "可见收藏一"},
		{ID: "favorite-2", Title: "可见收藏二"},
		{ID: "favorite-3", Title: "被过滤收藏"},
	}
	app := NewTestApp()
	app.SetInvalidator(func() {})
	app.library.Favorites = Library{Loaded: true, Items: items}
	app.library.History = Library{Loaded: true, Items: items}
	app.historyQuery = "可见"
	app.refreshVisibleLibraryLocked(libraryHistory)
	app.refreshVisibleLibraryLocked(libraryFavorites)
	if got := app.visibleLibrary(libraryHistory); len(got) != 2 || got[0].ID != "favorite-1" || got[1].ID != "favorite-2" {
		t.Fatalf("filtered favorites = %+v", got)
	}
	if len(app.library.Favorites.Items) != 3 {
		t.Fatalf("snapshot mutated: %+v", app.library.Favorites.Items)
	}
}

func TestHistoryHeaderMatchesFilteredGrid(t *testing.T) {
	items := []CatalogItem{
		{ID: "history-1", Title: "可见历史", Completed: false},
		{ID: "history-2", Title: "其他历史", Completed: true},
	}
	app := NewTestApp()
	app.SetInvalidator(func() {})
	app.library.History = Library{Loaded: true, Items: items}
	app.refreshVisibleLibraryLocked(libraryHistory)

	app.historyQuery = "可见"
	app.refreshVisibleLibraryLocked(libraryHistory)
	visible := app.visibleLibrary(libraryHistory)
	if len(visible) != 1 || visible[0].ID != "history-1" {
		t.Fatalf("filtered history = %#v", visible)
	}

	app.historyQuery = ""
	app.historyFilter = "completed"
	app.refreshVisibleLibraryLocked(libraryHistory)
	visible = app.visibleLibrary(libraryHistory)
	if len(visible) != 1 || visible[0].ID != "history-2" {
		t.Fatalf("completed history = %#v", visible)
	}
}

func TestReservationVisibleListAndCountsShareFilteredData(t *testing.T) {
	items := []CatalogItem{
		{ID: "online-1", Title: "已上线", Upcoming: false},
		{ID: "pending-1", Title: "待上线", Upcoming: true},
	}
	app := NewTestApp()
	app.SetInvalidator(func() {})
	app.library.Reserved = Library{Loaded: true, Items: items}
	app.reservationsPending = true
	app.refreshVisibleLibraryLocked(libraryReservations)
	visible := app.visibleLibrary(libraryReservations)
	if len(visible) != 1 || visible[0].ID != "pending-1" {
		t.Fatalf("pending reservations = %+v", visible)
	}
	if len(app.library.Reserved.Items) != 2 {
		t.Fatalf("reservation snapshot mutated: %+v", app.library.Reserved.Items)
	}
}
