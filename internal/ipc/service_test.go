package ipc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/memstore"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
	userstore "myguo/internal/store"
)

func TestNonZeroUserID(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "0", want: ""},
		{value: "", want: ""},
		{value: "invalid", want: ""},
		{value: "123", want: "123"},
		{value: " 456 ", want: "456"},
	}
	for _, tt := range tests {
		if got := nonZeroUserID(tt.value); got != tt.want {
			t.Errorf("nonZeroUserID(%q) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestRecommendFeedFiltersRepeatsAcrossCursorGeneration(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	store, err := userstore.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	type page struct {
		ids     []string
		cursor  string
		hasMore bool
		err     error
	}
	resets := 0
	responses := []page{
		{ids: []string{"a", "b"}, cursor: "expired", hasMore: true},
		{err: sdk.ErrAnonymousLimited},
		{ids: []string{"b", "c"}, cursor: "fresh", hasMore: true},
	}
	requests := 0
	svc := &HongguoService{
		store: store,
		media: &MediaServer{},
		discover: func(context.Context, sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
			if requests >= len(responses) {
				return nil, errors.New("unexpected request")
			}
			requests++
			response := responses[requests-1]
			if response.err != nil {
				return nil, response.err
			}
			items := make([]model.SeriesSummary, 0, len(response.ids))
			for _, id := range response.ids {
				items = append(items, model.SeriesSummary{ID: id, Title: "Drama " + id})
			}
			return &sdk.Page[model.SeriesSummary]{Items: items, Cursor: response.cursor, HasMore: response.hasMore}, nil
		},
		resetDevice: func(context.Context) error {
			resets++
			return nil
		},
	}

	first, err := svc.GetRecommendFeed(context.Background(), "推荐", "")
	if err != nil {
		t.Fatalf("first GetRecommendFeed failed: %v", err)
	}
	if len(first.Items) != 2 || first.Items[0].ID != "a" || first.Items[1].ID != "b" {
		t.Fatalf("first feed items = %+v, want a and b", first.Items)
	}

	// The anonymous stream returns limited and recovers with a new anonymous
	// session. The immediately replayed item must still be filtered.
	second, err := svc.GetRecommendFeed(context.Background(), "推荐", "expired")
	if err != nil {
		t.Fatalf("second GetRecommendFeed failed: %v", err)
	}
	if resets == 0 {
		t.Fatal("expired feed should recover through device reset")
	}
	if len(second.Items) != 1 || second.Items[0].ID != "c" {
		t.Fatalf("second feed items = %+v, want only c after cross-generation filtering", second.Items)
	}
}

func TestRecommendFeedContinuesPastRepeatedPages(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	for _, tt := range []struct {
		name           string
		expiredCursor  bool
		emptyFirstPage bool
		wantCursor     string
		wantResets     int
	}{
		{name: "existing session", wantCursor: "e"},
		{name: "expired session", expiredCursor: true, wantCursor: "e", wantResets: 1},
		{name: "renewed session", emptyFirstPage: true, wantCursor: "h", wantResets: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store, err := userstore.NewStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()

			oldIDs := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
			if err := store.SaveFeedSeenIDs("推荐", 1, oldIDs); err != nil {
				t.Fatal(err)
			}
			cursor := ""
			if tt.expiredCursor {
				cursor = "expired"
			}
			if err := store.SaveFeedProgress("推荐", cursor, 1); err != nil {
				t.Fatal(err)
			}
			resets := 0
			var visited []string
			svc := &HongguoService{
				store: store,
				media: &MediaServer{},
				discover: func(_ context.Context, opts sdk.DiscoverOptions) (*sdk.Page[model.SeriesSummary], error) {
					if opts.Cursor == "expired" {
						return nil, sdk.ErrAnonymousLimited
					}
					if tt.emptyFirstPage && resets == 0 {
						return &sdk.Page[model.SeriesSummary]{}, nil
					}
					index := 0
					if opts.Cursor != "" {
						index = -1
						for i, id := range oldIDs {
							if opts.Cursor == id {
								index = i + 1
								break
							}
						}
						if index < 0 {
							return nil, errors.New("unexpected cursor: " + opts.Cursor)
						}
					}
					id, nextCursor, hasMore := "fresh", "", false
					if index < len(oldIDs) {
						id, nextCursor, hasMore = oldIDs[index], oldIDs[index], true
					}
					visited = append(visited, id)
					return &sdk.Page[model.SeriesSummary]{
						Items:  []model.SeriesSummary{{ID: id, Title: id}},
						Cursor: nextCursor, HasMore: hasMore,
					}, nil
				},
				resetDevice: func(context.Context) error {
					resets++
					return nil
				},
			}

			first, err := svc.GetRecommendFeedBatch(context.Background(), "推荐", cursor, 12)
			if err != nil {
				t.Fatal(err)
			}
			if len(first.Items) != 0 || first.Cursor != tt.wantCursor || !first.HasMore {
				t.Fatalf("filtered prefix should preserve continuation %q: %+v", tt.wantCursor, first)
			}
			progress, ok := store.GetFeedProgress("推荐")
			if !ok || progress.Cursor != first.Cursor || progress.Generation != int64(1+tt.wantResets) {
				t.Fatalf("persisted continuation mismatch: %+v", progress)
			}

			second, err := svc.GetRecommendFeedBatch(context.Background(), "推荐", first.Cursor, 12)
			if err != nil {
				t.Fatal(err)
			}
			if len(second.Items) != 1 || second.Items[0].ID != "fresh" {
				t.Fatalf("should reach new content after eight repeated pages: %+v", second)
			}
			if resets != tt.wantResets {
				t.Fatalf("scanning repeated pages reset the session: got %d resets, want %d", resets, tt.wantResets)
			}
			if strings.Join(visited, ",") != "a,b,c,d,e,f,g,h,fresh" {
				t.Fatalf("pages should advance without replaying the prefix: %v", visited)
			}
		})
	}
}

func TestWarmupWindowLimitsAdaptToNetworkProfile(t *testing.T) {
	tests := map[string]struct {
		next     int
		previous int
	}{
		"weak":   {next: 1, previous: 0},
		"medium": {next: 2, previous: 1},
		"strong": {next: 3, previous: 2},
	}
	for profile, want := range tests {
		next, previous := warmupWindowLimits(profile)
		if next != want.next || previous != want.previous {
			t.Errorf("%s limits = (%d, %d), want (%d, %d)", profile, next, previous, want.next, want.previous)
		}
	}
}

func TestRecordFeedMetricsWritesSQLite(t *testing.T) {
	t.Setenv("MYGUO_TEST_NO_LEGACY_IMPORT", "1")
	dir := t.TempDir()
	store, err := userstore.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore err = %v", err)
	}
	defer store.Close()
	svc := &HongguoService{store: store}
	err = svc.RecordFeedMetrics(context.Background(), []FeedMetricDTO{{
		Type:           "switch",
		FromIndex:      1,
		ToIndex:        2,
		DurationMS:     123,
		StreamMode:     "progressive",
		CacheHit:       true,
		FullFallback:   true,
		NetworkProfile: "medium",
		Event:          "navigation-complete",
		InputID:        42,
		InputSource:    "keyboard",
		SessionID:      "session-1",
		WaitingMS:      350,
		RenderedMS:     700,
	}})
	if err != nil {
		t.Fatalf("RecordFeedMetrics err = %v", err)
	}

	count, err := store.FeedMetricCount()
	if err != nil || count != 1 {
		t.Fatalf("SQLite metric count = %d, %v; want 1, nil", count, err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "userdata.db")+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var payload string
	if err := db.QueryRow(`SELECT diagnostic_json FROM feed_metrics`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var metric FeedMetricDTO
	if err := json.Unmarshal([]byte(payload), &metric); err != nil {
		t.Fatal(err)
	}
	if metric.InputID != 42 || metric.InputSource != "keyboard" || metric.SessionID != "session-1" || metric.WaitingMS != 350 || metric.RenderedMS != 700 {
		t.Fatalf("diagnostic correlation fields were lost: %+v", metric)
	}
}

func TestGetDiscoverUnavailableContentReturnsEmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "device_register") || strings.Contains(r.URL.Path, "register") {
			_, _ = w.Write([]byte(`{"device_id":123456789,"install_id":987654321,"device_id_str":"123456789","install_id_str":"987654321","new_user":1}`))
			return
		}
		// Simulate openhongguo short filter returning empty items
		_, _ = w.Write([]byte(`{"code":0,"data":{"has_more":false,"next_offset":0,"session_id":"s1","video_data":[]}}`))
	}))
	defer ts.Close()

	client, err := adapters.NewClient("", sdk.Options{
		ProfileStore:       memstore.New(),
		DistributionOrigin: ts.URL,
		RegisterOrigin:     ts.URL,
	})
	if err != nil {
		t.Fatalf("NewClient err = %v", err)
	}

	svc := &HongguoService{
		client: client,
		cache:  make(map[string]cacheItem),
	}

	resp, err := svc.GetDiscover(context.Background(), "short_play", map[string]string{"category_dim_role": "cate_26"}, "")
	if err != nil {
		t.Fatalf("GetDiscover should handle content unavailable gracefully, got err: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(resp.Items))
	}
	if resp.HasMore {
		t.Fatalf("expected HasMore to be false, got true")
	}
}
