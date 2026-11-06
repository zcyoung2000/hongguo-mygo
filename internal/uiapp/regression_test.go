package uiapp

import (
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
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
			player := NewTestPlayerBridge()
			app := NewTestApp(player)
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
			if app.View() != target || app.DetailState().Title == "过期详情" || player.state().SeriesID != "" {
				t.Fatalf("late response changed current page: %s %+v %+v", app.View(), app.DetailState(), player.state())
			}
		})
	}
}
func TestProductionAppLoadsInitialDiscover(t *testing.T) {
	source := &fakeDetail{}
	app := NewApp(nil, nil)
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
	app := NewTestApp(nil)
	app.OpenView("search")
	tt := ui.NewTester(app.Render, 1200, 800)
	tt.ClickAt(300, 60)
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
func TestProductionPlaybackFallbackAndCallbacksBind(t *testing.T) {
	service := &ipc.HongguoService{}
	b := NewPlayerBridge(service, nil)
	if b.playbackProvider() != service {
		t.Fatal("production service fallback missing")
	}
	// Binding only the facade must succeed without trying to encode Window/functions.
	mygo.BindAs("RegressionPlayerCallbacks", b.Callbacks())
	code, err := playerScript(map[string]any{"event": "seek", "position": 12.5})
	if err != nil || !strings.Contains(code, `"position":12.5`) || strings.Contains(code, "%!") {
		t.Fatalf("invalid seek script: %s %v", code, err)
	}
}

type numberedPlayback struct{}

func (numberedPlayback) PlayEpisode(_ context.Context, series, episode string, num int, _ string, _ bool) (*ipc.PlaybackDTO, error) {
	if episode == "" {
		episode = string(rune('0'+num)) + "54321"
	}
	return &ipc.PlaybackDTO{SeriesID: series, EpisodeID: episode, EpisodeNum: num, StreamURL: "http://127.0.0.1/video", Title: "第N集", LastPosition: 8, DurationSec: 100}, nil
}

type savedProgress struct {
	series, episode, title, cover string
	num, total                    int
	position, duration            float64
}
type recordingSaver struct{ records chan savedProgress }

func (s *recordingSaver) SaveProgress(_ context.Context, series, episode, title, cover string, num, total int, position, duration float64) error {
	s.records <- savedProgress{series, episode, title, cover, num, total, position, duration}
	return nil
}
func settledPlayer(t *testing.T, b *PlayerBridge, num int) PlaybackState {
	t.Helper()
	waitFor(time.Second, func() bool { s := b.state(); return s.EpisodeNum == num && s.StreamURL != "" })
	s := b.state()
	if s.EpisodeNum != num || s.StreamURL == "" {
		t.Fatalf("play did not resolve: %+v", s)
	}
	return s
}
func TestHistoryResumesEpisodeAndSavesItsOwnMetadata(t *testing.T) {
	b := NewTestPlayerBridge()
	b.playback = numberedPlayback{}
	saver := &recordingSaver{make(chan savedProgress, 8)}
	b.saver = saver
	app := NewTestApp(b)
	app.detailSource = &fakeDetail{page: &ipc.SeriesDetailPageDTO{SeriesDetailDTO: ipc.SeriesDetailDTO{ID: "123456", Title: "真实剧名", CoverURL: "series-cover", EpisodeCount: 12}}}
	cards, err := convertHistory([]ipc.HistoryItem{{SeriesID: "123456", Title: "真实剧名", EpisodeNum: 4, PositionSec: 42, TotalEpisodes: 12}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.OpenItem(cards[0])
	app.loadDetail("123456")
	old := settledPlayer(t, b, 4)
	if old.Position != 42 || old.Title != "真实剧名" || old.TotalEpisodes != 12 {
		t.Fatalf("resume/metadata lost: %+v", old)
	}
	b.Next()
	next := settledPlayer(t, b, 5)
	payload := func(s PlaybackState) progressPayload {
		return progressPayload{SeriesID: s.SeriesID, EpisodeID: s.EpisodeID, Session: s.Session, Position: 50, Duration: 100}
	}
	if err := b.Callbacks().SaveProgress(payload(old)); err != nil {
		t.Fatal(err)
	}
	record := <-saver.records
	if record.title != "真实剧名" || record.cover != "series-cover" || record.num != 4 || record.total != 12 {
		t.Fatalf("mixed session metadata: %+v", record)
	}
	b.Callbacks().Progress(payload(old))
	if b.state().Session != next.Session || b.state().Position == 50 {
		t.Fatal("old progress affected new session")
	}
	if err := b.SaveProgress(payload(next)); err != nil {
		t.Fatal(err)
	}
	<-saver.records
	b.Stop()
	if err := b.SaveProgress(payload(old)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-saver.records:
		t.Fatal("old callback overwrote newer history")
	default:
	}
	b.Callbacks().Next(payload(next))
	if b.state().SeriesID != "" {
		t.Fatal("late ended restarted stopped player")
	}
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

type delayedPlayback struct {
	started chan struct{}
	release chan struct{}
}

func (d delayedPlayback) PlayEpisode(_ context.Context, series, episode string, num int, _ string, _ bool) (*ipc.PlaybackDTO, error) {
	close(d.started)
	<-d.release
	return &ipc.PlaybackDTO{SeriesID: series, EpisodeID: "654321", EpisodeNum: num, StreamURL: "http://127.0.0.1/video", DurationSec: 100}, nil
}
func TestPlaybackResponsePreservesPause(t *testing.T) {
	b := NewTestPlayerBridge()
	source := delayedPlayback{make(chan struct{}), make(chan struct{})}
	b.playback = source
	b.Play("123456", "", 2, 0, true)
	<-source.started
	b.Pause()
	close(source.release)
	if settledPlayer(t, b, 2).Playing {
		t.Fatal("late response restarted paused playback")
	}
}
