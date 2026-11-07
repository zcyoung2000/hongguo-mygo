package library

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	userstore "myguo/internal/store"
)

type failingTransport struct {
	mu          sync.Mutex
	failedPosts int
	roundTrips  []http.RoundTripper
}

func (transport *failingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.mu.Lock()
	index := len(transport.roundTrips)
	transport.roundTrips = append(transport.roundTrips, nil)
	transport.mu.Unlock()
	if request.Method == http.MethodPost {
		transport.mu.Lock()
		transport.failedPosts++
		transport.mu.Unlock()
		return nil, fmt.Errorf(`Post "%s": net/http: TLS handshake timeout`, request.URL)
	}
	transport.mu.Lock()
	next := transport.roundTrips[index]
	transport.mu.Unlock()
	if next == nil {
		return nil, fmt.Errorf("missing transport for %s", request.URL.Path)
	}
	return next.RoundTrip(request)
}

func historyRefreshTestItem() HistoryItem {
	return HistoryItem{
		SeriesID:      "710000000000001",
		Title:         "本地真实历史",
		CoverURL:      "https://media.invalid/cover.jpg",
		EpisodeNum:    3,
		EpisodeTitle:  "第3集",
		PositionSec:   45,
		DurationSec:   120,
		TotalEpisodes: 20,
		UpdatedAt:     time.Now().Unix() - 60,
	}
}

func seedQueuedHistoryUpload(t *testing.T, store *userstore.Store) {
	t.Helper()
	item := historyRefreshTestItem()
	if err := store.SaveHistory(item); err != nil {
		t.Fatalf("SaveHistory() error = %v", err)
	}
	if err := store.QueueHistoryUpload(item, "710000000000103"); err != nil {
		t.Fatalf("QueueHistoryUpload() error = %v", err)
	}
}

func assertLocalHistoryPreserved(t *testing.T, store *userstore.Store) {
	t.Helper()
	items, err := store.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}
	want := historyRefreshTestItem()
	if len(items) != 1 || items[0].Title != want.Title || items[0].EpisodeNum != 3 ||
		items[0].PositionSec != 45 || items[0].DurationSec != 120 || items[0].TotalEpisodes != 20 {
		t.Fatalf("local history = %+v, want %+v", items, want)
	}
	pending, err := store.PendingHistoryUploads(10)
	if err != nil {
		t.Fatalf("PendingHistoryUploads() error = %v", err)
	}
	if len(pending) != 1 || pending[0].EpisodeID != "710000000000103" {
		t.Fatalf("failed intent = %+v, want retained episode intent", pending)
	}
}

func TestRefreshHistoryKeepsLocalHistoryWhenUploadRejected(t *testing.T) {
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			log.add(capturedRequest{path: r.URL.Path, body: body})
			writeJSON(w, map[string]any{"code": 100103, "message": "PARAM_INVALID"})
			return
		}
		log.add(capturedRequest{path: r.URL.Path})
		writeJSON(w, map[string]any{"data": map[string]any{"data_list": []any{}, "has_more": false}})
	}))
	seedQueuedHistoryUpload(t, store)

	items, err := service.RefreshHistory(context.Background())
	if err != nil {
		t.Fatalf("RefreshHistory() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "本地真实历史" || items[0].EpisodeNum != 3 {
		t.Fatalf("RefreshHistory() = %+v, want local row", items)
	}
	assertLocalHistoryPreserved(t, store)
	calls := log.matching("read_history/update")
	if len(calls) != 1 {
		t.Fatalf("history update calls = %d, want one", len(calls))
	}
	payload := decodeGZIP(t, calls[0].body)
	updates := payload["update_datas"].([]any)
	update := updates[0].(map[string]any)
	for key, want := range map[string]any{
		"book_id":               float64(710000000000001),
		"vid":                   float64(710000000000103),
		"vid_index":             float64(2),
		"current_play_position": float64(45000),
		"duration":              float64(120),
		"episode_cnt":           float64(20),
	} {
		if update[key] != want {
			t.Fatalf("payload %s = %#v, want %#v", key, update[key], want)
		}
	}
}

func TestRefreshHistoryKeepsLocalHistoryWhenUploadTimesOut(t *testing.T) {
	remote := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"data": map[string]any{"data_list": []any{}, "has_more": false}})
	})
	transport := &failingTransport{}
	service, store := newSyncTestService(t, remote)
	service.httpClient = &http.Client{Transport: transport}
	seedQueuedHistoryUpload(t, store)

	items, err := service.RefreshHistory(context.Background())
	if err != nil {
		t.Fatalf("RefreshHistory() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "本地真实历史" || items[0].EpisodeNum != 3 {
		t.Fatalf("RefreshHistory() = %+v, want local row", items)
	}
	assertLocalHistoryPreserved(t, store)
	if transport.failedPosts != 1 {
		t.Fatalf("failed upload count = %d, want one", transport.failedPosts)
	}
}

func TestFailedHistoryUploadUsesBackoffAndRetainsIntent(t *testing.T) {
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"code": 100103, "message": "PARAM_INVALID"})
	}))
	seedQueuedHistoryUpload(t, store)

	if _, err := service.processPendingHistoryUploads(); err == nil {
		t.Fatal("invalid upload accepted")
	}
	pending, err := store.PendingHistoryUploads(10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %+v, %v; want one retained intent", pending, err)
	}
	if _, err := service.processPendingHistoryUploads(); err != nil {
		t.Fatalf("backoff caused an upstream request: %v", err)
	}
	key := historyUploadKey(pending[0])
	retry := service.historyUploadRetries[key]
	if retry.attempt != 1 {
		t.Fatalf("retry attempt = %d, want one", retry.attempt)
	}
	service.historyUploadRetries[key] = historyUploadRetry{attempt: retry.attempt, nextAt: time.Now().Add(-time.Second)}
	if _, err := service.processPendingHistoryUploads(); err == nil {
		t.Fatal("retry after backoff accepted invalid upload")
	}
	if service.historyUploadRetries[key].attempt != 2 {
		t.Fatal("failed retry was not marked")
	}
	assertLocalHistoryPreserved(t, store)
}

func TestHistoryInvalidUploadRetainsEveryIntent(t *testing.T) {
	log := &requestLog{}
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		payload := decodeGZIP(t, raw)
		updates := payload["update_datas"].([]any)
		log.add(capturedRequest{path: r.URL.Path, body: []byte(fmt.Sprint(len(updates)))})
		writeJSON(w, map[string]any{"code": 100103, "message": "PARAM_INVALID"})
	}))
	for i := 1; i <= 2; i++ {
		item := HistoryItem{
			SeriesID:      fmt.Sprintf("71000000000000%d", i),
			EpisodeNum:    i,
			PositionSec:   float64(i * 10),
			DurationSec:   100,
			TotalEpisodes: 10,
			UpdatedAt:     time.Now().Unix(),
		}
		if err := store.SaveHistory(item); err != nil {
			t.Fatalf("SaveHistory() error = %v", err)
		}
		if err := store.QueueHistoryUpload(item, fmt.Sprintf("7100000000001%02d", i)); err != nil {
			t.Fatalf("QueueHistoryUpload() error = %v", err)
		}
	}

	if _, err := service.processPendingHistoryUploads(); err == nil {
		t.Fatal("invalid upload accepted")
	}
	if pending, err := store.PendingHistoryUploads(10); err != nil || len(pending) != 2 {
		t.Fatalf("pending = %+v, %v; want both failed intents", pending, err)
	}
}
