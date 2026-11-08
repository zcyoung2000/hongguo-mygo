package uiapp

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func TestRankPaginationDeduplicatesAndStopsCursorCycles(t *testing.T) {
	provider := &parityProvider{}
	app := NewTestApp(nil)
	app.featuresSource = provider
	for page, cursor := range []string{"a", "b", "a"} {
		provider.rankResult = ipc.RankResponse{Content: "all", Board: "hot", Cursor: cursor, HasMore: true, Items: []ipc.RankItemDTO{
			{ItemType: "drama", SeriesID: "123456", Title: "榜单短剧", EpisodeCount: 46},
			{ItemType: "actor", UserID: "123456", Title: "榜单演员"},
		}}
		app.LoadRank(page > 0)
	}
	if app.rank.HasMore || len(app.rank.Items) != 2 {
		t.Fatalf("cursor cycle or repeated items survived: %+v", app.rank)
	}
	app.view = "rank"
	tt := ui.NewTester(app.Render, 1200, 800)
	if !tt.HasText("46集") || !tt.HasText("立即播放") {
		t.Fatalf("rank metadata/actions missing: %v", tt.Texts())
	}
}

type pagingDetail struct {
	fakeDetail
	page int
}

func (p *pagingDetail) GetDiscover(context.Context, string, map[string]string, string) (ipc.DiscoverResponse, error) {
	cursors := []string{"a", "b", "a"}
	cursor := cursors[p.page]
	p.page++
	return ipc.DiscoverResponse{Items: []ipc.SeriesCard{{ID: "123456", Title: "短剧"}}, Cursor: cursor, HasMore: true}, nil
}
func TestDiscoverAndActorPaginationStopCursorCycles(t *testing.T) {
	app := NewTestApp(nil)
	app.detailSource = &pagingDetail{}
	app.loadCatalog(false)
	app.loadCatalog(true)
	app.loadCatalog(true)
	if app.catalog.HasMore || len(app.catalog.Items) != 1 {
		t.Fatalf("catalog cycle not stopped: %+v", app.catalog)
	}
	provider := &parityProvider{profile: &ipc.ActorProfileDTO{UserID: "actor", Works: []ipc.SeriesCard{{ID: "123456"}}, HasMore: true, Cursor: "a"}}
	app.featuresSource = provider
	app.OpenActor("actor")
	app.loadActor(false)
	for _, cursor := range []string{"b", "a"} {
		provider.works = &ipc.ActorWorksResponse{Works: []ipc.SeriesCard{{ID: "223456"}}, HasMore: true, Cursor: cursor}
		app.loadActor(true)
	}
	if app.actor.HasMore || len(app.actor.Works) != 2 {
		t.Fatalf("actor cycle not stopped: %+v", app.actor)
	}
}

type blockingFavorite struct {
	fakeFavorite
	started, release chan struct{}
	calls            atomic.Int64
}

func (p *blockingFavorite) ToggleFavorite(context.Context, string, string, string, string, int) (bool, error) {
	p.calls.Add(1)
	close(p.started)
	<-p.release
	return true, nil
}

type revalidatingDetail struct {
	fakeDetail
	started, release chan struct{}
	calls            atomic.Int64
}

func (p *revalidatingDetail) GetSeriesPage(context.Context, string) (*ipc.SeriesDetailPageDTO, error) {
	if p.calls.Add(1) == 1 {
		close(p.started)
		<-p.release
		return &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "过期快照"}}, nil
	}
	return &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "最新快照", IsFavorite: true}}, nil
}
func TestPendingFavoriteCannotToggleTwiceOrBeUndoneByAnOlderRead(t *testing.T) {
	provider := &blockingFavorite{started: make(chan struct{}), release: make(chan struct{})}
	source := &revalidatingDetail{started: make(chan struct{}), release: make(chan struct{})}
	app := NewTestApp(nil)
	app.favorite = provider
	app.detailSource = source
	app.OpenDetail("123456")
	app.mu.Lock()
	app.detail = Detail{ID: "123456", Title: "原详情"}
	app.mu.Unlock()
	done := make(chan struct{})
	go func() { app.loadDetail("123456"); close(done) }()
	<-source.started
	app.ToggleDetailFavorite()
	<-provider.started
	app.ToggleDetailFavorite()
	close(provider.release)
	waitFor(time.Second, func() bool { return app.DetailState().Favorite })
	close(source.release)
	<-done
	waitFor(time.Second, func() bool { return app.DetailState().Title == "最新快照" })
	if provider.calls.Load() != 1 || !app.DetailState().Favorite || app.DetailState().Title != "最新快照" {
		t.Fatalf("mutation lost: calls=%d detail=%+v", provider.calls.Load(), app.DetailState())
	}
}
func TestReservationUpdatesEveryVisibleCatalogAndConfirmsCancellation(t *testing.T) {
	provider := &fakeFavorite{reserved: true}
	app := NewTestApp(nil)
	app.favorite = provider
	item := CatalogItem{ID: "123456", Title: "待播剧", Upcoming: true}
	app.catalog.Items = []CatalogItem{item}
	app.search.Items = []CatalogItem{item}
	app.releases.Items = []CatalogItem{item}
	app.ToggleItemReservation(item)
	if !app.catalog.Items[0].Reserved || !app.search.Items[0].Reserved || !app.releases.Items[0].Reserved {
		t.Fatal("reservation state did not propagate")
	}
	app.SetConfirmation(func(string) bool { return false })
	item.Reserved = true
	app.ToggleItemReservation(item)
	if provider.reservationCalls.Load() != 1 {
		t.Fatal("cancelled confirmation still mutated reservation")
	}
}


func TestAnOldMediaCloseCannotDismissANewerPlaybackSession(t *testing.T) {
	app := NewTestApp(nil)
	app.SetMediaHost(&MediaHost{})
	app.view = "detail"
	app.detail = Detail{ID: "123456"}
	app.PlayDetail()
	old := app.mediaSession
	app.PlayDetail()
	shell := NativeShell{App: app}
	shell.ClosePlayer(old)
	if !app.PlayerVisible() {
		t.Fatal("old close dismissed a newer player")
	}
	shell.ClosePlayer(app.mediaSession)
	if app.PlayerVisible() {
		t.Fatal("current close did not dismiss player")
	}
}

func TestNativeCollectionActionsFitTheMinimumWindow(t *testing.T) {
	for _, view := range []string{"rank", "new", "history", "favorites", "likes", "reservations"} {
		t.Run(view, func(t *testing.T) {
			app := NewTestApp(nil)
			app.view = view
			app.sidebarWidth = 420
			item := CatalogItem{ID: "123456", Title: "布局测试剧", Category: "真人剧", Description: "布局测试简介", Heat: "热播", EpisodeCount: 46}
			action := ""
			switch view {
			case "rank":
				app.rank.Items = []ipc.RankItemDTO{{ItemType: "drama", SeriesID: item.ID, Title: item.Title, EpisodeCount: 46, Description: item.Description, Heat: item.Heat, Position: 1}}
				action = "立即播放"
			case "new":
				item.Reserved = true
				item.Upcoming = true
				app.releases.Items = []CatalogItem{item}
				action = "取消预约"
			case "history":
				item.ResumeEpisode = 4
				item.Progress = 42
				item.UpdatedAt = 1
				app.library.History = Library{Title: "观看历史", Items: []CatalogItem{item}}
				action = "删除记录"
			case "favorites":
				app.library.Favorites = Library{Title: "我的收藏", Items: []CatalogItem{item}}
				action = "取消收藏"
			case "likes":
				item.EpisodeID = "654321"
				item.ResumeEpisode = 3
				app.library.Likes = Library{Title: "我的点赞", Items: []CatalogItem{item}}
				action = "取消点赞"
			case "reservations":
				item.Upcoming = true
				item.Reserved = true
				app.reservationsPending = true
				app.library.Reserved = Library{Title: "新剧预约", Items: []CatalogItem{item}}
				action = "取消预约"
			}
			tt := ui.NewTester(app.Render, 1000, 640)
			tt.SetDark(true)
			if view == "history" {
				for _, label := range []string{"删除记录", "清空全部"} {
					if _, ok := tt.Find(label); ok {
						t.Fatalf("history still exposes %s", label)
					}
				}
			} else {
				for _, removed := range []string{item.Description, "已收藏", "已点赞"} {
					if tt.HasText(removed) {
						t.Fatalf("card still displays %q: %v", removed, tt.Texts())
					}
				}
				r, ok := tt.Find(action)
				if !ok || r.Y < 0 || r.Y+r.H > 640 || r.X < 0 || r.X+r.W > 1000 {
					t.Fatalf("%s is clipped at the minimum window size: %+v (found=%v)", action, r, ok)
				}
			}
			if dir := os.Getenv("MYGUO_NATIVE_SCREENSHOT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				file, err := os.Create(filepath.Join(dir, view+".png"))
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
		})
	}
}

func TestCatalogCardsRenderDistinctAndCompleteMetadata(t *testing.T) {
	app := NewTestApp(nil)
	item := CatalogItem{
		ID: "123456", Title: "元数据测试剧", Category: "爱情", Heat: "303万热度",
		EpisodeCount: 142, Description: "剧情简介", Upcoming: true,
		Reserved: true, PublishAt: "2026-10-25T20:00:00+08:00",
	}
	app.view = "new"
	app.releases.Items = []CatalogItem{item}
	tt := ui.NewTester(app.Render, 1200, 800)
	texts := tt.Texts()
	for _, text := range texts {
		if text == "看到第1集 · 42%" || text == "待上线" {
			t.Fatalf("duplicate or redundant metadata rendered: %v", texts)
		}
	}
	if !tt.HasText("2026-10-25") || !tt.HasText("取消预约") {
		t.Fatalf("publish date or reservation action missing: %v", texts)
	}
}

func TestNativePlaybackDistinguishesSavedAndExplicitResumePositions(t *testing.T) {
	host := &MediaHost{}
	app := NewTestApp(nil)
	app.SetMediaHost(host)
	app.detail = Detail{ID: "123456"}
	app.PlayDetail()
	if host.desired["resumeAt"] != nil {
		t.Fatal("normal play suppressed the SDK's saved position")
	}
	app.resume = CatalogItem{ID: "123456", ResumeEpisode: 4, ResumePosition: 42}
	app.PlayDetail()
	if host.desired["resumeAt"] != float64(42) || host.desired["episodeNum"] != 4 {
		t.Fatal("history position was not sent to the player")
	}
	app.resume.ResumePosition = 0
	app.PlayDetail()
	if host.desired["resumeAt"] != float64(0) {
		t.Fatal("explicit zero position was lost")
	}
	app.ChooseEpisode(3)
	if host.desired["resumeAt"] != nil || host.desired["episodeNum"] != 3 {
		t.Fatal("episode selection suppressed saved progress")
	}
}
