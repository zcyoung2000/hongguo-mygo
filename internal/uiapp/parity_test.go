package uiapp

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

type parityProvider struct {
	mu            sync.Mutex
	calls         []string
	rankResult    ipc.RankResponse
	releaseResult ipc.NewReleaseResponse
	profile       *ipc.ActorProfileDTO
	works         *ipc.ActorWorksResponse
	seasons       *ipc.SeriesCollectionDTO
	adaptations   *ipc.SeriesCollectionDTO
}

func (p *parityProvider) record(value string) {
	p.mu.Lock()
	p.calls = append(p.calls, value)
	p.mu.Unlock()
}
func (p *parityProvider) has(value string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, v := range p.calls {
		if v == value {
			return true
		}
	}
	return false
}
func (p *parityProvider) GetRankings(_ context.Context, c, b, cursor string) (ipc.RankResponse, error) {
	p.record("rank:" + c + ":" + b + ":" + cursor)
	return p.rankResult, nil
}
func (p *parityProvider) GetNewReleases(_ context.Context, date string) (ipc.NewReleaseResponse, error) {
	p.record("new:" + date)
	return p.releaseResult, nil
}
func (p *parityProvider) GetActorProfile(_ context.Context, id string) (*ipc.ActorProfileDTO, error) {
	p.record("actor:" + id)
	return p.profile, nil
}
func (p *parityProvider) GetActorWorks(_ context.Context, id, cursor string) (*ipc.ActorWorksResponse, error) {
	p.record("works:" + id + ":" + cursor)
	return p.works, nil
}
func (p *parityProvider) GetSuggestions(_ context.Context, q string) ([]string, error) {
	p.record("suggest:" + q)
	return []string{q + "一", q + "二"}, nil
}
func (p *parityProvider) GetSeriesSeasons(_ context.Context, id string) (*ipc.SeriesCollectionDTO, error) {
	p.record("seasons:" + id)
	return p.seasons, nil
}
func (p *parityProvider) GetSeriesAdaptations(_ context.Context, id, source, title, cover, exclude string) (*ipc.SeriesCollectionDTO, error) {
	p.record("adaptations:" + id + ":" + source + ":" + exclude)
	return p.adaptations, nil
}

func TestNativeMediaRoutesUseFullMediaHostAndExplicitPlayback(t *testing.T) {
	host := &MediaHost{}
	app := NewTestApp(NewTestPlayerBridge())
	app.SetMediaHost(host)
	app.detailSource = &fakeDetail{page: &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "剧名", EpisodeCount: 12}}}
	app.OpenView("recommend")
	if host.desired["event"] != "recommend" {
		t.Fatal("recommend scene not reachable")
	}
	app.OpenView("settings")
	if host.desired["event"] != "settings" {
		t.Fatal("settings scene not reachable")
	}
	app.OpenDetail("123456")
	app.loadDetail("123456")
	if app.PlayerVisible() || host.desired["event"] != "stop" {
		t.Fatal("detail started playback before a user action")
	}
	app.PlayDetail()
	if !app.PlayerVisible() || host.desired["event"] != "play" || host.desired["seriesId"] != "123456" {
		t.Fatal("explicit playback not routed")
	}
	app.ExitMedia()
	if app.PlayerVisible() || app.View() != "detail" || host.desired["event"] != "stop" {
		t.Fatal("return to detail did not stop playback")
	}
	app.NavigateHistory(-1)
	if app.View() != "settings" {
		t.Fatal("back lost previous view")
	}
	app.NavigateHistory(1)
	if app.View() != "detail" || app.DetailState().ID != "123456" {
		t.Fatal("forward lost series identity")
	}
}
func TestRankingBoardsPaginationAndActorEntry(t *testing.T) {
	provider := &parityProvider{rankResult: ipc.RankResponse{Content: "actors", Board: "hot", Contents: []model.RankContent{{ID: "actors", Name: "演员", Boards: []model.RankBoard{{ID: "hot", Name: "热度榜"}}}}, Items: []ipc.RankItemDTO{{Position: 1, ItemType: "actor", UserID: "actor-id", Title: "演员甲"}}, Cursor: "next", HasMore: true}}
	app := NewTestApp(nil)
	app.featuresSource = provider
	app.OpenView("rank")
	app.SelectRank("actors", "hot")
	if !provider.has("rank:actors:hot:") {
		t.Fatal("selected board not passed to service")
	}
	tt := ui.NewTester(app.Render, 1200, 800)
	if !tt.HasText("热度榜") || !tt.HasText("演员甲") {
		t.Fatalf("ranking UI missing: %v", tt.Texts())
	}
	app.LoadRank(true)
	if !provider.has("rank:actors:hot:next") || app.rank.HasMore {
		t.Fatal("pagination cursor did not stop on repetition")
	}
	provider.profile = &ipc.ActorProfileDTO{UserID: "actor-id", Name: "演员甲", Followers: 100, Works: []ipc.SeriesCard{{ID: "123456", Title: "作品一"}}, HasMore: true, Cursor: "works-next"}
	provider.works = &ipc.ActorWorksResponse{Works: []ipc.SeriesCard{{ID: "123456", Title: "作品一"}, {ID: "223456", Title: "作品二"}}}
	app.OpenActor("actor-id")
	app.loadActor(false)
	app.loadActor(true)
	if !provider.has("works:actor-id:works-next") || len(app.actor.Works) != 2 {
		t.Fatal("actor pagination missing or duplicated works")
	}
	tt.Frame()
	if !tt.HasText("演员甲") || !tt.HasText("作品二") {
		t.Fatalf("actor profile not rendered: %v", tt.Texts())
	}
}
func TestNewCalendarAndHotBoardHaveDistinctDataSources(t *testing.T) {
	provider := &parityProvider{rankResult: ipc.RankResponse{Items: []ipc.RankItemDTO{{SeriesID: "123456", ItemType: "drama", Title: "热播剧"}}}, releaseResult: ipc.NewReleaseResponse{CurrentDate: "20261106", Dates: []string{"20261106", "20261107"}, Items: []ipc.NewReleaseItemDTO{{SeriesID: "223456", Title: "待播剧", Online: false, Reserved: true}}}}
	app := NewTestApp(nil)
	app.featuresSource = provider
	app.OpenView("new")
	app.LoadReleases()
	if !provider.has("rank:all:ranklist_new_rank_sc:") {
		t.Fatal("hot releases did not use the new-drama board")
	}
	app.SelectRelease(true, "20261107")
	if !provider.has("new:20261107") || !app.releases.Items[0].Upcoming || !app.releases.Items[0].Reserved {
		t.Fatal("calendar date or reservation state lost")
	}
	tt := ui.NewTester(app.Render, 1200, 800)
	if !tt.HasText("新剧日历") || !tt.HasText("11/07") || !tt.HasText("待播剧") || !tt.HasText("取消预约") {
		t.Fatalf("calendar affordances missing: %v", tt.Texts())
	}
}
func TestDetailSupplementsKeepSourceAndExcludeSeasons(t *testing.T) {
	provider := &parityProvider{seasons: &ipc.SeriesCollectionDTO{Title: "同系列", Items: []ipc.SeriesCard{{ID: "223456", Title: "第二季"}}}, adaptations: &ipc.SeriesCollectionDTO{Title: "同剧本", Items: []ipc.SeriesCard{{ID: "323456", Title: "改编剧"}}}}
	app := NewTestApp(nil)
	app.featuresSource = provider
	app.OpenDetail("123456")
	app.detail = Detail{ID: "123456", Source: &ipc.SourceNovelDTO{ID: "source", Title: "原著书名"}, Actors: []ipc.ActorInfo{{Name: "演员甲", Role: "主演"}}, Seasons: DetailCollection{Loading: true}, Adaptations: DetailCollection{Loading: true}}
	app.loadSupplement("123456", app.generation, false)
	waitFor(time.Second, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return !app.detail.Adaptations.Loading })
	if !provider.has("adaptations:123456:source:123456,223456") {
		t.Fatal("adaptations did not exclude same-series seasons")
	}
	tt := ui.NewTester(app.Render, 1200, 1200)
	if !tt.HasText("第二季") || !tt.HasText("改编剧") || !tt.HasText("原著书名") || !tt.HasText("演员甲") {
		t.Fatalf("detail sections missing: %v", tt.Texts())
	}
}
func TestSuggestionsKeyboardAndEscape(t *testing.T) {
	app := NewTestApp(nil)
	app.OpenView("search")
	app.query = "短剧"
	app.suggestions = []string{"短剧一", "短剧二"}
	app.suggestionIndex = -1
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.ClickAt(300, 60)
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if app.Query() != "短剧一" {
		t.Fatalf("first suggestion not selected: %s", app.Query())
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if app.Query() != "短剧二" {
		t.Fatal("second keyboard selection lost popup")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	app.mu.Lock()
	count := len(app.suggestions)
	app.mu.Unlock()
	if count != 0 {
		t.Fatal("escape did not close suggestions")
	}
}
func TestHistoryFilteringUsesLastEpisodeCompletion(t *testing.T) {
	rows, _ := convertHistory([]ipc.HistoryItem{{SeriesID: "123456", Title: "中途看完一集", EpisodeNum: 1, TotalEpisodes: 10, PositionSec: 100, DurationSec: 100}, {SeriesID: "223456", Title: "全剧看完", EpisodeNum: 10, TotalEpisodes: 10, PositionSec: 96, DurationSec: 100}}, nil)
	app := NewTestApp(nil)
	app.library.History = Library{Items: rows}
	app.SetLibraryFilter("completed", "")
	visible := app.visibleLibrary(libraryHistory)
	if len(visible) != 1 || visible[0].ID != "223456" {
		t.Fatal("an intermediate episode was treated as a completed series")
	}
	app.SetLibraryFilter("unwatched", "中途")
	if len(app.visibleLibrary(libraryHistory)) != 1 {
		t.Fatal("history text filter missing")
	}
}
func TestSidebarWidthPersistsAndClamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native-ui.json")
	app := NewTestApp(nil)
	app.LoadPreferences(path)
	app.SetSidebarWidth(999)
	app.FlushPreferences()
	restored := NewTestApp(nil)
	restored.LoadPreferences(path)
	if restored.sidebarWidth != 420 {
		t.Fatalf("sidebar preference lost: %v", restored.sidebarWidth)
	}
}

type mutationProvider struct {
	mu    sync.Mutex
	calls []string
}

func (p *mutationProvider) add(s string) { p.mu.Lock(); p.calls = append(p.calls, s); p.mu.Unlock() }
func (p *mutationProvider) ToggleFavorite(_ context.Context, id, _, _, _ string, _ int) (bool, error) {
	p.add("favorite:" + id)
	return false, nil
}
func (p *mutationProvider) ToggleLike(_ context.Context, id, ep, _, _ string, num int) (bool, error) {
	p.add("like:" + id + ":" + ep)
	return false, nil
}
func (p *mutationProvider) ToggleReservation(_ context.Context, id, _, _, _, _ string) (bool, error) {
	p.add("reservation:" + id)
	return false, nil
}
func TestCollectionRemovalPreservesOtherLikedEpisodesAndConfirmsReservations(t *testing.T) {
	provider := &mutationProvider{}
	app := NewTestApp(nil)
	app.libraryMutations = provider
	first, second := CatalogItem{ID: "123456", EpisodeID: "654321", ResumeEpisode: 1}, CatalogItem{ID: "123456", EpisodeID: "654322", ResumeEpisode: 2}
	app.library.Likes = Library{Items: []CatalogItem{first, second}}
	app.RemoveLibraryItem(libraryLikes, first)
	if len(app.library.Likes.Items) != 1 || app.library.Likes.Items[0].EpisodeID != "654322" || !strings.Contains(provider.calls[0], "654321") {
		t.Fatal("unlike removed wrong episode")
	}
	app.confirm = func(string) bool { return false }
	app.library.Reserved = Library{Items: []CatalogItem{first}}
	app.RemoveLibraryItem(libraryReservations, first)
	if len(provider.calls) != 1 || len(app.library.Reserved.Items) != 1 {
		t.Fatal("cancelled confirmation still removed reservation")
	}
}

func TestNoGlobalBackForwardControlsOnAnyNativePage(t *testing.T) {
	for _, view := range []string{"recommend", "discover", "search", "rank", "new", "history", "favorites", "likes", "reservations", "settings", "detail", "actor"} {
		app := NewTestApp(nil)
		app.view = view
		tt := ui.NewTester(app.Render, 1200, 800)
		for _, text := range tt.Texts() {
			if text == "返回" || text == "前进" {
				t.Fatalf("%s still shows global navigation button %q", view, text)
			}
		}
	}
}

func TestRankUsesTheOriginalDefaultHotBoard(t *testing.T) {
	provider := &parityProvider{rankResult: ipc.RankResponse{Content: "all", Board: "ranklist_hot_sc", Items: []ipc.RankItemDTO{{SeriesID: "123456", ItemType: "drama", Title: "热度剧"}}}}
	app := NewTestApp(nil)
	if app.rank.Content != "all" || app.rank.Board != "ranklist_hot_sc" {
		t.Fatalf("rank defaults = %s/%s, want original frontend defaults", app.rank.Content, app.rank.Board)
	}
	app.SetInvalidator(func() {})
	app.featuresSource = provider
	app.testMode = false
	app.view = ""
	app.OpenView("rank")
	waitFor(time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return provider.has("rank:all:ranklist_hot_sc:") && !app.rank.Loading && len(app.rank.Items) == 1
	})
	if !provider.has("rank:all:ranklist_hot_sc:") {
		t.Fatalf("rank calls = %v, want original default board", provider.calls)
	}
	app.mu.Lock()
	rank := app.rank
	app.mu.Unlock()
	if rank.Error != "" || len(rank.Items) != 1 {
		t.Fatalf("rank result = %+v", rank)
	}
}

func TestNonMediaRoutesDoNotShowTheEmbeddedWebView(t *testing.T) {
	app := NewTestApp(nil)
	app.SetMediaHost(&MediaHost{})
	for _, view := range []string{"rank", "history", "discover"} {
		app.view = view
		tt := ui.NewTester(app.Render, 1200, 800)
		if tt.WebViewAt(700, 400) != nil {
			t.Fatalf("%s showed an embedded web view over the page body", view)
		}
	}
}

func TestEveryBrowsingPageMatchesOriginalLayoutAndSkeleton(t *testing.T) {
	provider := &parityProvider{rankResult: ipc.RankResponse{Content: "all", Board: "ranklist_hot_sc", Items: []ipc.RankItemDTO{{SeriesID: "123456", ItemType: "drama", Title: "热度剧", Position: 1}}}, releaseResult: ipc.NewReleaseResponse{CurrentDate: "20261106", Dates: []string{"20261106"}, Items: []ipc.NewReleaseItemDTO{{SeriesID: "223456", Title: "待播剧", Online: false}}}}
	app := NewTestApp(nil)
	app.SetInvalidator(func() {})
	app.featuresSource = provider
	app.detailSource = &fakeDetail{page: &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "323456", Title: "详情剧"}}}
	app.libraryData = &fakeLibrary{}
	type expectation struct {
		view      string
		loading   string
		ready     []string
		openFirst bool
	}
	tests := []expectation{
		{view: "discover", loading: "正在加载目录", ready: []string{"真人剧", "漫剧", "AI剧"}},
		{view: "rank", loading: "正在加载排行榜", ready: []string{"第1名", "热度剧"}},
		{view: "new", loading: "加载中", ready: []string{"热播新剧", "新剧日历"}},
		{view: "search", loading: "加载中", ready: []string{"搜索"}},
		{view: "history", loading: "加载中", ready: []string{"观看历史"}},
		{view: "favorites", loading: "加载中", ready: []string{"我的收藏"}},
		{view: "likes", loading: "加载中", ready: []string{"我的点赞"}},
		{view: "reservations", loading: "加载中", ready: []string{"新剧预约"}},
		{view: "actor", loading: "加载中", ready: []string{"演员甲"}},
	}
	for _, test := range tests {
		blocked := make(chan struct{})
		source := &fakeDetail{page: &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "323456", Title: "详情剧"}}}
		source.delay = blocked
		app := NewTestApp(nil)
		app.SetInvalidator(func() {})
		app.featuresSource = provider
		app.detailSource = source
		app.libraryData = &fakeLibrary{}
		app.view = test.view
		app.testMode = false
		tt := ui.NewTester(app.Render, 1200, 800)
		if test.view == "discover" {
			go app.loadCatalog(false)
			waitFor(200*time.Millisecond, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return app.catalog.Loading })
			tt.Frame()
		}
		if test.view == "rank" {
			blockedRank := make(chan struct{})
			rankSource := &delayedRank{inner: provider, delay: blockedRank}
			app.featuresSource = rankSource
			go app.LoadRank(false)
			waitFor(200*time.Millisecond, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return app.rank.Loading })
			time.Sleep(100 * time.Millisecond)
			tt.Frame()
			if !tt.HasText(test.loading) {
				t.Fatalf("%s missing skeleton/loading state: %v", test.view, tt.Texts())
			}
			close(blockedRank)
			app.featuresSource = provider
			waitFor(time.Second, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return !app.rank.Loading })
		} else {
			switch test.view {
			case "new":
				app.LoadReleases()
			case "search":
				app.SearchSubmitted()
			case "history":
				app.loadLibrary(libraryHistory)
			case "favorites":
				app.loadLibrary(libraryFavorites)
			case "likes":
				app.loadLibrary(libraryLikes)
			case "reservations":
				app.loadLibrary(libraryReservations)
			case "actor":
				provider.profile = &ipc.ActorProfileDTO{UserID: "actor-id", Name: "演员甲", Followers: 100, Likes: 20, Works: []ipc.SeriesCard{{ID: "123456", Title: "作品一"}}, HasMore: false}
				provider.works = &ipc.ActorWorksResponse{Works: []ipc.SeriesCard{{ID: "123456", Title: "作品一"}}, HasMore: false}
				app.OpenActor("actor-id")
				app.loadActor(false)
			}
		}
		tt.Frame()
		waitFor(time.Second, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return !app.loading })
		tt.Frame()
		for _, text := range test.ready {
			if test.view == "rank" && app.rank.Loading {
				continue
			}
			if !tt.HasText(text) {
				t.Fatalf("%s missing ready text %q: %v", test.view, text, tt.Texts())
			}
		}
	}
}

type delayedRank struct {
	inner featureProvider
	delay chan struct{}
}

func (d *delayedRank) GetRankings(ctx context.Context, content, board, cursor string) (ipc.RankResponse, error) {
	select {
	case <-d.delay:
	case <-ctx.Done():
	}
	return d.inner.GetRankings(ctx, content, board, cursor)
}
func (d *delayedRank) GetNewReleases(ctx context.Context, date string) (ipc.NewReleaseResponse, error) {
	return d.inner.GetNewReleases(ctx, date)
}
func (d *delayedRank) GetActorProfile(ctx context.Context, id string) (*ipc.ActorProfileDTO, error) {
	return d.inner.GetActorProfile(ctx, id)
}
func (d *delayedRank) GetActorWorks(ctx context.Context, id, cursor string) (*ipc.ActorWorksResponse, error) {
	return d.inner.GetActorWorks(ctx, id, cursor)
}
func (d *delayedRank) GetSuggestions(ctx context.Context, q string) ([]string, error) {
	return d.inner.GetSuggestions(ctx, q)
}
func (d *delayedRank) GetSeriesSeasons(ctx context.Context, id string) (*ipc.SeriesCollectionDTO, error) {
	return d.inner.GetSeriesSeasons(ctx, id)
}
func (d *delayedRank) GetSeriesAdaptations(ctx context.Context, id, source, title, cover, exclude string) (*ipc.SeriesCollectionDTO, error) {
	return d.inner.GetSeriesAdaptations(ctx, id, source, title, cover, exclude)
}

func TestSkeletonsMatchOriginalPageShapes(t *testing.T) {
	blockedDiscover := make(chan struct{})
	discoverSource := &fakeDetail{}
	discoverSource.delay = blockedDiscover
	app := NewTestApp(nil)
	app.SetInvalidator(func() {})
	app.detailSource = discoverSource
	app.testMode = false
	go app.loadCatalog(false)
	waitFor(200*time.Millisecond, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return app.catalog.Loading })
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.Frame()
	if !tt.HasText("正在加载目录") {
		t.Fatalf("discover skeleton text missing: %v", tt.Texts())
	}
	close(blockedDiscover)
	app.view = "rank"

	blockedRank := make(chan struct{})
	rankSource := &parityProvider{}
	delayed := &delayedRank{inner: rankSource, delay: blockedRank}
	app.featuresSource = delayed
	go app.LoadRank(false)
	waitFor(200*time.Millisecond, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return app.rank.Loading })
	tt.Frame()
	if !tt.HasText("正在加载排行榜") {
		t.Fatalf("rank skeleton text missing: %v", tt.Texts())
	}

	close(blockedRank)
	waitFor(time.Second, func() bool { app.mu.Lock(); defer app.mu.Unlock(); return !app.rank.Loading })
	app.rank.Loading = false
	app.view = "new"
	app.releases.Loading = true
	tt.Frame()
	if !tt.HasText("正在加载新剧") {
		t.Fatalf("new skeleton text missing: %v", tt.Texts())
	}

	app.view = "search"
	app.releases.Loading = false
	app.search.Loading = true
	tt.Frame()
	if !tt.HasText("正在搜索") {
		t.Fatalf("search skeleton text missing: %v", tt.Texts())
	}

	app.view = "history"
	app.search.Loading = false
	app.library.History.Loading = true
	tt.Frame()
	if !tt.HasText("正在加载本地数据") {
		t.Fatalf("library skeleton text missing: %v", tt.Texts())
	}

	app.view = "detail"
	app.library.History.Loading = false
	app.detail.Loading = true
	tt.Frame()
	if !tt.HasText("正在加载详情") {
		t.Fatalf("detail skeleton text missing: %v", tt.Texts())
	}

	app.view = "actor"
	app.detail.Loading = false
	app.actor.Loading = true
	tt.Frame()
	if !tt.HasText("正在加载演员主页") {
		t.Fatalf("actor skeleton text missing: %v", tt.Texts())
	}
}

func TestNativePageScreens(t *testing.T) {
	dir := os.Getenv("MYGUO_NATIVE_SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGUO_NATIVE_SCREENSHOT_DIR to audit native page screenshots")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	provider := &parityProvider{
		rankResult: ipc.RankResponse{Content: "all", Board: "ranklist_hot_sc", Items: []ipc.RankItemDTO{
			{Position: 1, ItemType: "drama", SeriesID: "123456", Title: "排行榜热度剧", EpisodeCount: 48, Heat: "312万热度", Description: "排行榜简介"},
			{Position: 2, ItemType: "actor", UserID: "actor-id", Title: "榜单演员", Heat: "热门短剧演员", Intro: "演员简介", Details: []string{"古装", "都市"}},
		}},
		releaseResult: ipc.NewReleaseResponse{CurrentDate: "20261106", Dates: []string{"20261106", "20261107"}, Items: []ipc.NewReleaseItemDTO{{SeriesID: "223456", Title: "待上线新剧", Online: false, Reserved: true}}},
		profile:       &ipc.ActorProfileDTO{UserID: "actor-id", Name: "演员甲", Description: "演员简介内容", Tags: []string{"古装"}, Followers: 12000, Likes: 34000, HasMore: true},
		works:         &ipc.ActorWorksResponse{Works: []ipc.SeriesCard{{ID: "123456", Title: "演员作品一", EpisodeCount: 32}}, HasMore: true},
	}
	filters := []FilterRow{{Title: "题材", Key: "topic", Options: []FilterOption{{Label: "全部", Value: ""}, {Label: "古装", Value: "1"}, {Label: "都市", Value: "2"}}}}
	detail := Detail{
		ID: "123456", Title: "详情页测试短剧", Description: "这是一段用来核对详情页排版的剧情简介。",
		Tags: []string{"古装", "逆袭"}, Heat: "312万热度", FollowCount: 12000, LikeCount: 34000,
		PublishText: "2026-11-06 更新", StatusText: "连载中", PlayCount: 560000, EpisodeCount: 12,
		Episodes: []CatalogItem{{ID: "e1", Title: "第1集", EpisodeCount: 1}, {ID: "e2", Title: "第2集", EpisodeCount: 2}},
		Related:  []CatalogItem{{ID: "333456", Title: "相关推荐剧", EpisodeCount: 24, Heat: "80万热度"}}, RelatedTitle: "相关推荐",
	}
	save := func(name string, app *App) {
		t.Helper()
		tt := ui.NewTester(app.Render, 1200, 800)
		tt.SetDark(true)
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
	libraryItem := CatalogItem{ID: "123456", Title: "本地记录剧", Category: "真人剧", Description: "记录简介", EpisodeCount: 20, ResumeEpisode: 4, Progress: 42, UpdatedAt: time.Now().Unix()}
	for _, state := range []struct {
		name    string
		view    string
		prepare func(*App)
	}{
		{name: "discover-loading", view: "discover", prepare: func(app *App) { app.catalog.Loading = true }},
		{name: "discover-ready", view: "discover", prepare: func(app *App) {
			app.catalog.Filters = filters
			app.catalog.Items = []CatalogItem{{ID: "123456", Title: "找剧测试短剧", Category: "真人剧", Heat: "280万热度", EpisodeCount: 36, Description: "找剧简介"}}
			app.catalog.HasMore = true
		}},
		{name: "rank-loading", view: "rank", prepare: func(app *App) { app.rank.Loading = true }},
		{name: "rank-ready", view: "rank", prepare: func(app *App) {
			app.rank = RankingState{Content: "all", Board: "ranklist_hot_sc", Items: provider.rankResult.Items, HasMore: true}
		}},
		{name: "new-loading", view: "new", prepare: func(app *App) { app.releases.Loading = true }},
		{name: "new-ready", view: "new", prepare: func(app *App) {
			app.releases.Calendar = true
			app.releases.Dates = provider.releaseResult.Dates
			app.releases.Date = "20261106"
			app.releases.Items = []CatalogItem{{ID: "223456", Title: "待上线新剧", Upcoming: true, Reserved: true, PublishAt: "20261106", EpisodeCount: 18}}
		}},
		{name: "search-loading", view: "search", prepare: func(app *App) { app.search.Loading = true }},
		{name: "search-ready", view: "search", prepare: func(app *App) {
			app.search.Items = []CatalogItem{{ID: "123456", Title: "搜索结果剧", Heat: "190万热度", EpisodeCount: 22, Description: "搜索简介"}}
		}},
		{name: "history-loading", view: "history", prepare: func(app *App) { app.library.History.Loading = true }},
		{name: "history-ready", view: "history", prepare: func(app *App) {
			app.library.History = Library{Title: "观看历史", Items: []CatalogItem{libraryItem}}
		}},
		{name: "favorites-loading", view: "favorites", prepare: func(app *App) { app.library.Favorites.Loading = true }},
		{name: "favorites-ready", view: "favorites", prepare: func(app *App) {
			app.library.Favorites = Library{Title: "我的收藏", Items: []CatalogItem{libraryItem}}
		}},
		{name: "likes-loading", view: "likes", prepare: func(app *App) { app.library.Likes.Loading = true }},
		{name: "likes-ready", view: "likes", prepare: func(app *App) { app.library.Likes = Library{Title: "我的点赞", Items: []CatalogItem{libraryItem}} }},
		{name: "reservations-loading", view: "reservations", prepare: func(app *App) { app.library.Reserved.Loading = true }},
		{name: "reservations-ready", view: "reservations", prepare: func(app *App) {
			item := libraryItem
			item.Upcoming = true
			item.PublishAt = "20261106"
			app.library.Reserved = Library{Title: "新剧预约", Items: []CatalogItem{item}}
		}},
		{name: "detail-loading", view: "detail", prepare: func(app *App) { app.detail.Loading = true }},
		{name: "detail-ready", view: "detail", prepare: func(app *App) { app.detail = detail }},
		{name: "actor-loading", view: "actor", prepare: func(app *App) { app.actor.Loading = true }},
		{name: "actor-ready", view: "actor", prepare: func(app *App) {
			app.actor = ActorState{ID: "actor-id", Profile: provider.profile, Works: []CatalogItem{{ID: "123456", Title: "演员作品一", EpisodeCount: 32}}, HasMore: true}
		}},
		{name: "settings", view: "settings"},
	} {
		state := state
		t.Run(state.name, func(t *testing.T) {
			app := NewTestApp(nil)
			app.featuresSource = provider
			app.view = state.view
			if state.prepare != nil {
				state.prepare(app)
			}
			save(state.name, app)
		})
	}
}
