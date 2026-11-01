package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/memstore"
	"github.com/beilunyang/openhongguo/sdk"
)

func TestShortFilterSelections(t *testing.T) {
	tests := []struct {
		name   string
		genre  string
		values map[string]string
		want   map[string][]string
	}{
		{
			name:   "uses every selected filter row",
			genre:  "short_play",
			values: map[string]string{"category_dim_role": "cate_26", "category_dim_epoch": "cate_127", "sort": ""},
			want: map[string][]string{
				"genre":              {"short_play"},
				"category_dim_role":  {"cate_26"},
				"category_dim_epoch": {"cate_127"},
			},
		},
		{
			name:   "frontend genre overrides any stale row value",
			genre:  "comic_series",
			values: map[string]string{"genre": "short_play"},
			want:   map[string][]string{"genre": {"comic_series"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortFilterSelections(tt.genre, tt.values)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("shortFilterSelections() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

func TestWarmupFeedCancelsStaleWorkBeforeMetadataCompletes(t *testing.T) {
	media := &MediaServer{
		prefetchQ:      make(chan prefetchTask, 4),
		prefetchQueued: make(map[string]prefetchTask),
		prefetchActive: make(map[string]*prefetchExecution),
	}
	canceled := make(chan struct{})
	media.prefetchActive["episode\x00stale\x00720p"] = &prefetchExecution{
		cancel: func() {
			select {
			case <-canceled:
			default:
				close(canceled)
			}
		},
	}
	svc := &HongguoService{media: media}

	if err := svc.WarmupFeed("series1", "current", nil, nil, "720p", "medium"); err != nil {
		t.Fatalf("WarmupFeed err = %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("stale prefetch was not canceled before adjacent metadata completed")
	}
}

func TestRecordFeedMetricsWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	svc := &HongguoService{appDir: dir}
	err := svc.RecordFeedMetrics([]FeedMetricDTO{{
		Type:           "switch",
		FromIndex:      1,
		ToIndex:        2,
		DurationMS:     123,
		StreamMode:     "progressive",
		CacheHit:       true,
		FullFallback:   true,
		NetworkProfile: "medium",
	}})
	if err != nil {
		t.Fatalf("RecordFeedMetrics err = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "feed-metrics.jsonl"))
	if err != nil {
		t.Fatalf("read metrics err = %v", err)
	}
	var metric FeedMetricDTO
	if err := json.Unmarshal(raw[:len(raw)-1], &metric); err != nil {
		t.Fatalf("decode metrics err = %v", err)
	}
	if metric.Type != "switch" || metric.NetworkProfile != "medium" || !metric.FullFallback {
		t.Fatalf("unexpected metric: %+v", metric)
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

	resp, err := svc.GetDiscover("short_play", map[string]string{"category_dim_role": "cate_26"}, "")
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

func TestParseAccountShortRankBuildsItemsAndCursor(t *testing.T) {
	body := []byte(`{
		"code": 0,
		"data": {
			"has_more": true,
			"next_offset": 2,
			"session_id": "session",
			"cell_view": {
				"cell_selector": {"outer_row": {"items": [
					{"show_name": "全部", "selector_item_id": "all", "is_selected": true, "sub_cell_selector": {"outer_row": {"items": [
						{"show_name": "推荐榜", "selector_item_id": "ranklist_hot_sc", "is_selected": true}
					]}}}
				]}},
				"cell_data": [{
					"video_data": [{
						"series_id": 7100000000000101,
						"title": "Ranked One",
						"cover": "https://example.test/one.jpg",
						"episode_cnt": 90,
						"rec_text_item": {"RecommendText": "992万推荐"}
					}],
					"ugc_user_data": [{
						"user_info": {"base_info": {"user_name": "Actor", "user_id": 7100000000000201}},
						"rec_text_item": {"RecommendText": "999万人气"},
						"intro": "作品一",
						"sub_title_list": [{"content": "1部作品"}]
					}]
				}]
			}
		}
	}`)

	rank, err := parseAccountShortRank(body, "all", "ranklist_hot_sc", rankCursor{})
	if err != nil {
		t.Fatalf("parseAccountShortRank() error = %v", err)
	}
	if rank.Content != "all" || rank.Board != "ranklist_hot_sc" {
		t.Fatalf("selection = %s/%s, want all/ranklist_hot_sc", rank.Content, rank.Board)
	}
	if len(rank.Items) != 2 || rank.Items[0].SeriesID != "7100000000000101" || rank.Items[0].EpisodeCount != 90 {
		t.Fatalf("unexpected video item: %+v", rank.Items)
	}
	if rank.Items[1].ActorID != "7100000000000201" || rank.Items[1].Details[0] != "1部作品" {
		t.Fatalf("unexpected actor item: %+v", rank.Items[1])
	}
	if rank.Cursor == "" || !rank.HasMore {
		t.Fatalf("expected a continuation cursor, got cursor=%q hasMore=%v", rank.Cursor, rank.HasMore)
	}
	next, err := decodeRankCursor(rank.Cursor)
	if err != nil {
		t.Fatalf("decodeRankCursor() error = %v", err)
	}
	if next.Offset != 2 || next.Session != "session" || next.Content != "all" || next.Board != "ranklist_hot_sc" {
		t.Fatalf("unexpected cursor: %+v", next)
	}
}
