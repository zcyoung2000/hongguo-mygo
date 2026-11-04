package ipc

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beilunyang/openhongguo/adapters"
	"github.com/beilunyang/openhongguo/adapters/memstore"
	"github.com/beilunyang/openhongguo/sdk"
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
