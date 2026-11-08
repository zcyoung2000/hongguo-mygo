package uiapp

import (
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

type delayedDetail struct {
	fakeDetail
	started chan struct{}
	release chan struct{}
}

func (d *delayedDetail) GetSeriesPage(context.Context, string) (*ipc.SeriesDetailPageDTO, error) {
	close(d.started)
	<-d.release
	return &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "过期详情"}}, nil
}
func TestLateDetailCannotReplaceNavigationOrStartPlayer(t *testing.T) {
	for _, target := range []string{"history", "detail"} {
		t.Run(target, func(t *testing.T) {
			source := &delayedDetail{started: make(chan struct{}), release: make(chan struct{})}
			app := NewTestApp()
			app.detailSource = source
			app.OpenDetail("123456")
			done := make(chan struct{})
			go func() { app.loadDetail("123456"); close(done) }()
			<-source.started
			if target == "history" {
				app.OpenView("history")
			} else {
				app.OpenDetail("223456")
			}
			close(source.release)
			<-done
			if app.View() != target || app.DetailState().Title == "过期详情" || app.PlayerVisible() {
				t.Fatalf("late response changed current page: %s %+v player=%t", app.View(), app.DetailState(), app.PlayerVisible())
			}
		})
	}
}
func TestProductionAppLoadsInitialDiscover(t *testing.T) {
	source := &fakeDetail{}
	app := NewApp(nil)
	app.detailSource = source
	app.OpenView("discover")
	waitFor(time.Second, func() bool {
		return source.discovers.Load() == 1 && source.filters.Load() == 1 && !app.CatalogState().Loading
	})
	if source.discovers.Load() != 1 || source.filters.Load() != 1 {
		t.Fatal("initial discover never loaded")
	}
	app.OpenView("discover")
	if source.discovers.Load() != 1 {
		t.Fatal("duplicate initial load")
	}
}
func TestSearchTypingSurvivesFramesAndSubmit(t *testing.T) {
	app := NewTestApp()
	app.OpenView("search")
	tt := ui.NewTester(app.Render, 1200, 800)
	bar, barOK := tt.Find("搜索短剧")
	if !barOK {
		t.Fatal("search input missing")
	}
	tt.ClickAt(bar.X+bar.W/2, bar.Y+bar.H/2)
	for _, word := range []string{"短", "剧", "关键词"} {
		tt.Type(word)
		tt.Frame()
	}
	if app.Query() != "短剧关键词" {
		t.Fatalf("input lost: %q", app.Query())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	waitFor(time.Second, func() bool { return app.SearchState().Loading })
	if app.Query() != "短剧关键词" || !app.SearchState().Loading {
		t.Fatal("submission lost query")
	}
}

type shelfDeadlineLibrary struct {
	fakeLibrary
	refreshes atomic.Int64
}

func (s *shelfDeadlineLibrary) RefreshFavorites(context.Context) ([]ipc.FavoriteItem, error) {
	s.refreshes.Add(1)
	return nil, context.DeadlineExceeded
}

func (s *shelfDeadlineLibrary) RefreshHistory(ctx context.Context) ([]ipc.HistoryItem, error) {
	return s.GetHistory(ctx)
}

func (s *shelfDeadlineLibrary) RefreshLikes(ctx context.Context) ([]ipc.LikeItem, error) {
	return s.GetLikes(ctx)
}

func (s *shelfDeadlineLibrary) RefreshReservations(ctx context.Context) ([]ipc.ReservationItem, error) {
	return s.GetReservations(ctx)
}

func TestShelfDeadlineDoesNotBlockOrDisguiseCoreDetail(t *testing.T) {
	library := &shelfDeadlineLibrary{
		fakeLibrary: fakeLibrary{favorites: []ipc.FavoriteItem{{SeriesID: "123456"}}},
	}
	detail := &fakeDetail{page: &ipc.SeriesDetailPageDTO{
		SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "真实详情标题", CoverURL: "https://media.invalid/cover.jpg", EpisodeCount: 12},
	}}
	app := NewTestApp()
	app.detailSource = detail
	app.libraryData = library
	app.SetInvalidator(func() {})

	app.OpenDetail("123456")
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	if !tt.HasText("正在加载详情") {
		t.Fatalf("core detail loading state missing: %v", tt.Texts())
	}

	done := make(chan struct{})
	go func() {
		app.loadDetail("123456")
		close(done)
	}()
	app.loadLibraryWithRefresh(libraryFavorites, true)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shelf deadline blocked core detail loading")
	}
	tt.Frame()
	tt.Advance(200 * time.Millisecond)

	state := app.DetailState()
	libraryState := app.LibraryState()
	if state.Title != "真实详情标题" || state.Loading || state.Error != "" {
		t.Fatalf("core detail state = %+v", state)
	}
	if libraryState.Favorites.Loading || libraryState.Favorites.Error == "" {
		t.Fatalf("favorites state = %+v", libraryState.Favorites)
	}
	if app.GlobalError() != "" || tt.HasText("资料库加载失败") || tt.HasText("context deadline exceeded") {
		t.Fatalf("misleading global shelf banner shown: global=%q texts=%v", app.GlobalError(), tt.Texts())
	}
	title, titleVisible := tt.Find("真实详情标题")
	if !titleVisible || title.X < 0 || title.Y < 0 || title.X+title.W > 1200 || title.Y+title.H > 800 {
		t.Fatalf("detail title clipped or absent: %+v", title)
	}
	if !tt.HasText("播放 / 继续观看") {
		t.Fatalf("core detail controls missing: %v", tt.Texts())
	}
	if !tt.HasText("正在加载...") || tt.HasText("重试") {
		t.Fatalf("supplement state changed by independent shelf failure: %v", tt.Texts())
	}

	screenshotDir := "/tmp/mygo-ui-bugs/regression"
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		t.Fatal(err)
	}
	screenshotPath := filepath.Join(screenshotDir, "detail-shelf-deadline.png")
	file, err := os.Create(screenshotPath)
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
	t.Logf("detail resilience screenshot: %s", screenshotPath)
}
func TestArtworkFetchesMissingImageOnceAndInvalidates(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		png.Encode(w, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	}))
	defer server.Close()
	ready := make(chan struct{}, 1)
	loader := newArtworkLoader(t.TempDir(), func(string) string { return server.URL }, func() { ready <- struct{}{} })
	for range 5 {
		loader.get("uncached-cover")
	}
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("image completion never invalidated")
	}
	if loader.get("uncached-cover") == nil || requests.Load() != 1 {
		t.Fatal("missing image or duplicate fetches")
	}
}
