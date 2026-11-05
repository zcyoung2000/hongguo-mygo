package ipc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestRecommendQueueRestoresOnlyUnviewedCardsWithoutNetwork(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	const cover = "https://p3-novel.byteimg.com/origin/test.jpg"
	svc := &HongguoService{store: store, media: &MediaServer{},
		discover: func(context.Context, sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			return &sdk.Page[model.SeriesSummary]{
				Items: []model.SeriesSummary{
					{ID: "a", Title: "A", CoverURL: cover},
					{ID: "b", Title: "B", CoverURL: cover},
					{ID: "c", Title: "C", CoverURL: cover},
				}, Cursor: "next-page", HasMore: true,
			}, nil
		},
	}
	ctx := context.Background()
	if _, err := svc.GetRecommendFeed(ctx, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConsumeRecommendFeedItem(ctx, "", "a"); err != nil {
		t.Fatal(err)
	}
	queued, err := store.GetFeedQueue("推荐")
	if err != nil || len(queued) != 2 || strings.Contains(string(queued[0].Payload), "127.0.0.1") {
		t.Fatalf("queue must store stable catalog metadata: %+v, %v", queued, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	// No SDK client: a network-dependent restore would fail or panic.
	svc = &HongguoService{store: store, media: &MediaServer{}}
	for range 2 {
		restored, err := svc.RestoreRecommendFeed(ctx, "推荐")
		if err != nil {
			t.Fatal(err)
		}
		if len(restored.Items) != 2 || restored.Items[0].ID != "b" || restored.Items[1].ID != "c" ||
			restored.Cursor != "next-page" || restored.Items[0].CoverURL != svc.media.ImageURL(cover) {
			t.Fatalf("restore lost order, metadata or continuation: %+v", restored)
		}
	}
	// Background recommendations append after the restored queue; recently
	// viewed cards still pass through the existing feed deduplication filter.
	svc.discover = func(_ context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
		if opts.Cursor != "next-page" {
			t.Fatalf("background fill rewound the feed: %q", opts.Cursor)
		}
		return &sdk.Page[model.SeriesSummary]{Items: []model.SeriesSummary{{ID: "a"}, {ID: "d", Title: "D"}}, Cursor: "after-d", HasMore: true}, nil
	}
	if _, err := svc.GetRecommendFeed(ctx, "推荐", "next-page"); err != nil {
		t.Fatal(err)
	}
	restored, err := svc.RestoreRecommendFeed(ctx, "推荐")
	if err != nil || len(restored.Items) != 3 || restored.Items[0].ID != "b" || restored.Items[1].ID != "c" || restored.Items[2].ID != "d" {
		t.Fatalf("background fill displaced the pending queue: %+v, %v", restored, err)
	}
}

func TestRestoredFeedPlaybackUsesCompleteLocalAlternativeWithoutSDK(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	t.Chdir(t.TempDir())
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	const series, episode = "710000000000001", "710000000000101"
	for _, quality := range []string{"720p", "540p"} {
		if err := store.RememberEpisode(series, episode, 1, quality); err != nil {
			t.Fatal(err)
		}
	}
	path := episodeCachePath("", episode, "540p")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("local media fixture", 128)), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &HongguoService{store: store, media: &MediaServer{}}
	result, err := svc.PlayEpisode(context.Background(), series, "", 1, "720p", true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsFullCached || result.Quality != "540p" || result.EpisodeID != episode || result.StreamURL != result.FullStreamURL {
		t.Fatalf("complete alternative should start without episode/quality network probes: %+v", result)
	}
}

func TestViewingAcknowledgementDoesNotWaitForBackgroundRecommendations(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	svc := &HongguoService{store: store, media: &MediaServer{}}
	if err := svc.queueRecommendCards("推荐", []SeriesCard{{ID: "shown"}, {ID: "pending"}}); err != nil {
		t.Fatal(err)
	}
	entered, release, filled := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	svc.discover = func(context.Context, sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
		close(entered)
		<-release
		return &sdk.Page[model.SeriesSummary]{Items: []model.SeriesSummary{{ID: "new"}}, Cursor: "next", HasMore: true}, nil
	}
	go func() {
		_, err := svc.GetRecommendFeed(context.Background(), "推荐", "")
		filled <- err
	}()
	<-entered
	consumed := make(chan error, 1)
	go func() { consumed <- svc.ConsumeRecommendFeedItem(context.Background(), "推荐", "shown") }()
	select {
	case err := <-consumed:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("viewing acknowledgement blocked behind the recommendation network")
	}
	close(release)
	if err := <-filled; err != nil {
		t.Fatal(err)
	}
	items, err := store.GetFeedQueue("推荐")
	if err != nil || len(items) != 2 || items[0].SeriesID != "pending" || items[1].SeriesID != "new" {
		t.Fatalf("background fill restored a consumed item or displaced the queue: %+v, %v", items, err)
	}
}
