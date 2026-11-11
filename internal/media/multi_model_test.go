package media

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestNextEpisodesKeepsOrderedPlayableWindow(t *testing.T) {
	episodes := []model.Episode{
		{ID: "3", Number: 3, Playable: true},
		{ID: "1", Number: 1, Playable: true},
		{ID: "4", Number: 4, Playable: true},
		{ID: "2b", Number: 2},
		{ID: "2", Number: 2, Playable: true},
		{ID: "2c", Number: 2, Playable: true},
	}
	got := NextEpisodes(episodes, 1)
	if len(got) != 2 || got[0].ID != "2" || got[1].ID != "3" {
		t.Fatalf("NextEpisodes = %+v, want episodes 2 and 3", got)
	}
	if len(NextEpisodes(episodes, 4)) != 0 {
		t.Fatal("window after the last episode should be empty")
	}
}

func TestMultiVideoModelBodyAndParser(t *testing.T) {
	episodes := []model.Episode{{ID: "e1"}, {ID: "e2"}}
	body, err := multiVideoModelBody(episodes)
	if err != nil {
		t.Fatal(err)
	}
	var request MultiVideoModelRequest
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	if len(request.VideoID) != 2 || request.VideoID[0] != "e1" || request.BizParam["video_platform"] != float64(3) {
		t.Fatalf("request = %s", body)
	}
	if _, err := multiVideoModelBody([]model.Episode{{ID: "1"}, {ID: "2"}, {ID: "3"}}); err == nil {
		t.Fatal("oversized batch accepted")
	}

	response := []byte(`{"code":0,"data":{"e1":{"streams":[{"url":"https://media.invalid/e1"}]},"e2":{"streams":[{"url":"https://media.invalid/e2"}]}}}`)
	parsed, err := parseMultiVideoModels(response, episodes)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 || parsed["e1"].EpisodeID != "e1" || len(parsed["e2"].Streams) != 1 {
		t.Fatalf("parsed = %+v", parsed)
	}
	if _, err := parseMultiVideoModels([]byte(`{"code":100001}`), episodes); err == nil {
		t.Fatal("API error accepted")
	}
}

func TestMultiModelBatcherChunksDeduplicatesAndCancelsStaleWindow(t *testing.T) {
	requests := make([][]string, 0, 3)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request MultiVideoModelRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		requests = append(requests, request.VideoID)
		_, _ = w.Write([]byte(`{"code":0,"data":{"e1":{"streams":[{"url":"https://a"}]}}}`))
	}))
	defer ts.Close()

	batcher := NewMultiVideoModelBatcher(&sdk.Client{}, &http.Client{})
	batcher.multiVideoModelURL = ts.URL
	episodes := []model.Episode{{ID: "e1"}, {ID: "e2"}, {ID: "e3"}}
	batcher.ReplaceWindow(episodes)
	batcher.Run(context.Background(), func(string, *model.Playback) {})
	if len(requests) != 2 || len(requests[0]) != 2 || len(requests[1]) != 1 {
		t.Fatalf("batch requests = %v", requests)
	}

	secondRun := make(chan string, 1)
	go batcher.Run(context.Background(), func(string, *model.Playback) {})
	select {
	case id := <-secondRun:
		t.Fatalf("completed window repeated request and remembered %q", id)
	case <-time.After(50 * time.Millisecond):
	}

	batcher.ReplaceWindow([]model.Episode{{ID: "fresh", Number: 1, Playable: true}})
	batcher.Run(context.Background(), func(string, *model.Playback) {})
	latest := requests[len(requests)-1:]
	if len(latest) != 1 || len(latest[0]) != 1 || latest[0][0] != "fresh" {
		t.Fatalf("replacement requests = %v", requests)
	}
	batcher.mu.Lock()
	activeLoads := len(batcher.loads)
	batcher.mu.Unlock()
	if activeLoads != 0 {
		t.Fatalf("loads were not released: %d", activeLoads)
	}
	batcher.forgetWindow()
	batcher.mu.Lock()
	window := batcher.window
	batcher.mu.Unlock()
	if window != nil {
		t.Fatal("empty window was retained")
	}
}

func TestMultiModelBatcherFailureDoesNotRememberModels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":100001,"message":"rate limited"}`))
	}))
	defer ts.Close()

	batcher := NewMultiVideoModelBatcher(&sdk.Client{}, &http.Client{})
	batcher.multiVideoModelURL = ts.URL
	batcher.ReplaceWindow([]model.Episode{{ID: "e1"}})
	remembered := make(chan string, 3)
	go batcher.Run(context.Background(), func(id string, pb *model.Playback) {
		if pb != nil {
			remembered <- id
		}
	})
	select {
	case id := <-remembered:
		t.Fatalf("failed request remembered %q", id)
	default:
	}
}

func TestRememberedMultiModelUsesExistingCacheExpiry(t *testing.T) {
	ms := &MediaServer{pbCache: make(map[string]playbackCacheItem)}
	ms.RememberPlayback("episode", &model.Playback{Streams: []model.Stream{{URL: "https://media.invalid/a"}}})
	if _, err := ms.resolvePlayback(context.Background(), "episode"); err != nil {
		t.Fatalf("prefetched model did not hit cache: %v", err)
	}
	if ms.playbackCacheHits.Load() != 1 {
		t.Fatalf("cache hits = %d", ms.playbackCacheHits.Load())
	}
	ms.pbCacheMu.Lock()
	item := ms.pbCache["episode"]
	item.expiresAt = time.Now().Add(-time.Second)
	ms.pbCache["episode"] = item
	ms.pbCacheMu.Unlock()
	if _, err := ms.resolvePlayback(context.Background(), "episode"); !errors.Is(err, errNilPlaybackClient) {
		t.Fatalf("expired model was served: %v", err)
	}
}
