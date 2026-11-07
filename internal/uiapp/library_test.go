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
	app := NewTestApp(nil)
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
	app := NewTestApp(nil)
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
	app := NewTestApp(nil)
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
	app := NewTestApp(nil)
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
