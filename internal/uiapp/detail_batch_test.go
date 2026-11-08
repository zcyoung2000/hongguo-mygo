package uiapp

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"myguo/internal/ipc"
)

func TestDetailResumeSurvivesPlayerCloseAndIgnoresOtherSeries(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = Detail{ID: "123456", Title: "续播详情"}
	shell := NativeShell{App: app}
	shell.ResumeProgress(ResumeProgress{SeriesID: "123456", EpisodeNum: 8, Position: 725, Session: 0, Sequence: 1})
	if state := app.DetailState(); app.resume.ResumeEpisode != 8 || app.resume.ResumePosition != 725 {
		t.Fatalf("resume not retained: %+v", state)
	}
	app.CloseMedia()
	if app.resume.ResumeEpisode != 8 || app.resume.ResumePosition != 725 {
		t.Fatalf("resume lost after close: %+v", app.resume)
	}
	app.OpenDetail("223456")
	if app.resume.ResumeEpisode != 0 || app.resume.ID == "123456" {
		t.Fatalf("old series resume polluted new detail: %+v", app.resume)
	}
	shell.ResumeProgress(ResumeProgress{SeriesID: "123456", EpisodeNum: 9, Position: 10, Session: 0, Sequence: 1})
	if app.resume.ResumeEpisode != 0 {
		t.Fatalf("stale sequence accepted: %+v", app.resume)
	}
}

func TestDetailActorsUseSchemaIDOrRemoveButtonAffordance(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = Detail{ID: "123456", Actors: testActorPage()}
	tt := renderTester(t, app, 1200, 800)
	if _, ok := tt.AccessByRole(ui.RoleButton, "演员主页：可跳转演员"); !ok {
		t.Fatalf("actor with schema ID missing button role")
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, "演员主页：无ID演员"); ok {
		t.Fatalf("actor without ID has button affordance")
	}
	if _, ok := tt.Find("无ID演员"); !ok {
		t.Fatalf("non-clickable actor disappeared")
	}
}

func TestDetailCollectionEmptyStateDistinguishesFromError(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = Detail{
		ID: "123456", Title: "空态详情",
		Seasons:     DetailCollection{Title: "同系列短剧"},
		Adaptations: DetailCollection{Title: "同一剧本改编", Error: "加载失败"},
	}
	tt := renderTester(t, app, 1200, 800)
	if !tt.HasText("暂无同系列短剧") {
		t.Fatalf("collection empty state missing")
	}
	if !tt.HasText("同一剧本改编") || tt.HasText("暂无同一剧本改编") {
		t.Fatalf("empty state shown for failed collection")
	}
}

func TestDetailStatsOmitMissingFields(t *testing.T) {
	app := NewTestApp()
	app.view = "detail"
	app.detail = Detail{ID: "123456", Title: "字段缺失详情", FollowCount: 12000}
	tt := renderTester(t, app, 1200, 800)
	if tt.HasText("· 人在追") || tt.HasText("· 0次播放") {
		t.Fatalf("detail stats rendered empty fragments")
	}
	if !tt.HasText("1.2万人在追") {
		t.Fatalf("non-empty stat missing")
	}
}

func TestFavoriteDuringDetailLoadDoesNotRefetchContent(t *testing.T) {
	pageCalls := atomic.Int32{}
	seasonCalls := atomic.Int32{}
	adaptionCalls := atomic.Int32{}
	source := &countingDetailProvider{calls: &pageCalls}
	features := &countingFeatureProvider{seasonCalls: &seasonCalls, adaptationCalls: &adaptionCalls}
	app := NewTestApp()
	app.detailSource = source
	app.featuresSource = features
	app.favorite = &instantFavorite{favorite: true}
	app.SetInvalidator(func() {})
	app.testMode = false
	app.mu.Lock()
	app.view = "detail"
	app.detail = Detail{ID: "123456"}
	app.mu.Unlock()
	done := make(chan struct{})
	go func() { app.loadDetail("123456"); close(done) }()
	source.waitStarted(t)
	app.ToggleDetailFavorite()
	close(source.block)
	waitFor(time.Second, func() bool {
		state := app.DetailState()
		return state.ID == "123456" && state.Source != nil
	})
	features.waitSeasonStarted(t)
	waitFor(time.Second, func() bool { return app.DetailState().Favorite })
	waitFor(time.Second, func() bool { return app.DetailState().Seasons.Loading == false })
	if pageCalls.Load() > 2 {
		t.Fatalf("detail page calls exceeded initial plus one stale retry: %d", pageCalls.Load())
	}
	if seasonCalls.Load() != 1 || adaptionCalls.Load() > 1 {
		t.Fatalf("supplement requests raced: seasons=%d adaptations=%d", seasonCalls.Load(), adaptionCalls.Load())
	}
	<-done
}

func testActorPage() []ipc.ActorInfo {
	return []ipc.ActorInfo{{Name: "可跳转演员", Role: "演员", UserID: "710000000000001"}, {Name: "无ID演员", Role: "演员"}}
}

func renderTester(t *testing.T, app *App, width, height int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(app.Render, width, height)
	if tt.FramePending() {
		tt.Advance(200)
	}
	return tt
}

type countingDetailProvider struct {
	calls       *atomic.Int32
	block       chan struct{}
	started     chan struct{}
	once        sync.Once
	startedInit sync.Once
}

func (p *countingDetailProvider) GetSeriesPage(context.Context, string) (*ipc.SeriesDetailPageDTO, error) {
	p.calls.Add(1)
	if p.block == nil {
		p.block = make(chan struct{})
	}
	p.startedInit.Do(func() { p.started = make(chan struct{}); close(p.started) })
	<-p.block
	return &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "详情内容"}, Source: &ipc.SourceNovelDTO{ID: "source", Title: "原著"}}, nil
}
func (p *countingDetailProvider) GetSeriesRelated(context.Context, string) (*ipc.SeriesCollectionDTO, error) {
	return &ipc.SeriesCollectionDTO{}, nil
}
func (p *countingDetailProvider) GetFilterPanel(context.Context, string) ([]ipc.FilterRowDTO, error) {
	return nil, nil
}
func (p *countingDetailProvider) GetDiscover(context.Context, string, map[string]string, string) (ipc.DiscoverResponse, error) {
	return ipc.DiscoverResponse{}, nil
}
func (p *countingDetailProvider) waitStarted(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && p.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	if p.calls.Load() == 0 {
		t.Fatal("detail page not started")
	}
}

type countingFeatureProvider struct {
	seasonCalls, adaptationCalls *atomic.Int32
	seasonStarted                chan struct{}
	seasonOnce                   sync.Once
	seasonStartedInit            sync.Once
	seasonRelease                chan struct{}
	released                     bool
}

func (p *countingFeatureProvider) GetSeriesSeasons(context.Context, string) (*ipc.SeriesCollectionDTO, error) {
	p.seasonCalls.Add(1)
	if p.seasonRelease == nil {
		p.seasonRelease = make(chan struct{})
	}
	p.seasonStartedInit.Do(func() { p.seasonStarted = make(chan struct{}); close(p.seasonStarted) })
	<-p.seasonRelease
	return &ipc.SeriesCollectionDTO{Items: []ipc.SeriesCard{{ID: "season", Title: "同系列"}}}, nil
}
func (p *countingFeatureProvider) GetSeriesAdaptations(context.Context, string, string, string, string, string) (*ipc.SeriesCollectionDTO, error) {
	p.adaptationCalls.Add(1)
	return &ipc.SeriesCollectionDTO{}, nil
}
func (p *countingFeatureProvider) GetRankings(context.Context, string, string, string) (ipc.RankResponse, error) {
	return ipc.RankResponse{}, nil
}
func (p *countingFeatureProvider) GetNewReleases(context.Context, string) (ipc.NewReleaseResponse, error) {
	return ipc.NewReleaseResponse{}, nil
}
func (p *countingFeatureProvider) GetActorProfile(context.Context, string) (*ipc.ActorProfileDTO, error) {
	return nil, nil
}
func (p *countingFeatureProvider) GetActorWorks(context.Context, string, string) (*ipc.ActorWorksResponse, error) {
	return nil, nil
}
func (p *countingFeatureProvider) GetSuggestions(context.Context, string) ([]string, error) {
	return nil, nil
}
func (p *countingFeatureProvider) waitSeasonStarted(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && p.seasonCalls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	if p.seasonCalls.Load() == 0 {
		t.Fatal("season request not started")
	}
}

type instantFavorite struct{ favorite bool }

func (f *instantFavorite) ToggleFavorite(context.Context, string, string, string, string, int) (bool, error) {
	return f.favorite, nil
}
func (f *instantFavorite) ToggleReservation(context.Context, string, string, string, string, string) (bool, error) {
	return false, nil
}
