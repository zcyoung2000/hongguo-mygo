package library

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	userstore "myguo/internal/store"
)

func TestProgressSchedulerMergesWindowByEpisode(t *testing.T) {
	now := time.Now()
	scheduler := &progressScheduler{
		entries: make(map[string]*progressFlushRequest),
		now:     func() time.Time { return now },
		window:  5 * time.Second,
		delta:   5,
	}
	first := progressFlushRequest{seriesID: "series", episodeID: "one", positionSec: 10, scheduleCancel: make(chan struct{})}
	if !scheduler.add(first) {
		t.Fatal("first progress was not admitted")
	}
	second := progressFlushRequest{seriesID: "series", episodeID: "one", positionSec: 12, scheduleCancel: make(chan struct{})}
	if scheduler.add(second) {
		t.Fatal("same episode advanced too little but was admitted")
	}
	third := progressFlushRequest{seriesID: "series", episodeID: "one", positionSec: 15, scheduleCancel: make(chan struct{})}
	if !scheduler.add(third) {
		t.Fatal("same episode did not replace after delta threshold")
	}
	other := progressFlushRequest{seriesID: "series", episodeID: "two", positionSec: 1, scheduleCancel: make(chan struct{})}
	if !scheduler.add(other) {
		t.Fatal("different episode was blocked")
	}
	if got := scheduler.take("series", "one"); got == nil || got.positionSec != third.positionSec {
		t.Fatalf("latest same-episode progress = %#v, want third", got)
	}
	if got := scheduler.take("series", "two"); got == nil || got.episodeID != other.episodeID {
		t.Fatalf("parallel episode progress = %#v, want other", got)
	}
}
func newTestProgressScheduler() *progressScheduler {
	return newProgressScheduler(progressSchedulerConfig{
		now:    func() time.Time { return time.Now() },
		window: 20 * time.Millisecond,
		delta:  5,
	})
}

func TestProgressSchedulerForcesAndMergesConcurrentEpisodes(t *testing.T) {
	scheduler := newTestProgressScheduler()
	first := progressFlushRequest{seriesID: "series", episodeID: "one", positionSec: 1, force: true, scheduleCancel: make(chan struct{})}
	second := progressFlushRequest{seriesID: "series", episodeID: "two", positionSec: 1, force: true, scheduleCancel: make(chan struct{})}
	if !scheduler.add(first) || !scheduler.add(second) {
		t.Fatal("forced progress was not admitted")
	}
	if got := scheduler.take("series", "one"); got == nil || got.episodeID != first.episodeID {
		t.Fatalf("forced first = %#v, want first", got)
	}
	if got := scheduler.take("series", "two"); got == nil || got.episodeID != second.episodeID {
		t.Fatalf("forced second = %#v, want second", got)
	}
}

func TestProgressFailureRemainsPendingAndBackoffIsBounded(t *testing.T) {
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"code": 1001})
	}))
	item := HistoryItem{
		SeriesID: "710000000000001", Title: "Pending", EpisodeNum: 1,
		PositionSec: 12, DurationSec: 100, TotalEpisodes: 10, UpdatedAt: time.Now().Unix(),
	}
	if err := store.SaveHistory(item); err != nil {
		t.Fatal(err)
	}
	if err := store.QueueHistoryUpload(item, "710000000000101"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.processPendingHistoryUploads(); err == nil {
		t.Fatal("processPendingHistoryUploads() error = nil, want failure")
	}
	if pending, err := store.PendingHistoryUploads(10); err != nil || len(pending) != 1 || pending[0].PositionSec != 12 {
		t.Fatalf("pending after failure = %#v, %v", pending, err)
	}
	requests := []userstore.PendingHistoryUpload{
		{HistoryItem: item, EpisodeID: "710000000000101"},
	}
	service.markHistoryUploadFailed(requests, context.Canceled)
	first := service.historyUploadRetries[historyUploadKey(requests[0])]
	service.markHistoryUploadFailed(requests, context.Canceled)
	second := service.historyUploadRetries[historyUploadKey(requests[0])]
	if !second.nextAt.After(first.nextAt) {
		t.Fatalf("backoff did not increase: first=%v second=%v", first.nextAt, second.nextAt)
	}
	delay := second.nextAt.Sub(time.Now())
	if delay > historyUploadMaxBackoff {
		t.Fatalf("backoff = %v, want <= %v", delay, historyUploadMaxBackoff)
	}
}

func TestFlushPendingProgressPersistsScheduledRequestsBeforeClose(t *testing.T) {
	service, store := newSyncTestService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		writeJSON(w, map[string]any{"code": 0})
	}))
	service.progressScheduler.mu.Lock()
	service.progressScheduler.entries["710000000000001\x0071000000000101"] = &progressFlushRequest{
		seriesID: "710000000000001", episodeID: "710000000000101", title: "剧", episodeNum: 1,
		totalEpisodes: 2, positionSec: 3, durationSec: 10,
		receivedAt: time.Now(), scheduleCancel: make(chan struct{}),
	}
	service.progressScheduler.entries["710000000000001\x0071000000000102"] = &progressFlushRequest{
		seriesID: "710000000000001", episodeID: "710000000000102", title: "剧", episodeNum: 2,
		totalEpisodes: 2, positionSec: 4, durationSec: 10,
		receivedAt: time.Now(), scheduleCancel: make(chan struct{}),
	}
	service.progressScheduler.mu.Unlock()
	service.WaitForBackgroundSync()
	service.FlushPendingProgress()
	items, err := store.GetHistory()
	if err != nil || len(items) != 1 || items[0].EpisodeNum != 2 || items[0].PositionSec != 4 {
		t.Fatalf("history after flush = %#v, %v; want latest episode", items, err)
	}
}
