package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestEpisodeCachePathSeparatesQualities(t *testing.T) {
	if got := episodeCachePath("/tmp", "710000000000001", DefaultVideoQuality); got != "/tmp/episodes/710000000000001_v2.mp4" {
		t.Fatalf("default path = %q", got)
	}
	if got := episodeCachePath("/tmp", "710000000000001", "1080p"); got != "/tmp/episodes/710000000000001_1080p_v2.mp4" {
		t.Fatalf("1080p path = %q", got)
	}
}

func TestProgressiveVideoURLRequestsStreamingMode(t *testing.T) {
	ms := &MediaServer{port: 12345}
	got := ms.ProgressiveVideoURL("ep1", "series1", 1, "720p")
	if !strings.Contains(got, "http://127.0.0.1:12345/video?") ||
		!strings.Contains(got, "mode=progressive") ||
		!strings.Contains(got, "quality=720p") {
		t.Fatalf("progressive URL = %q", got)
	}
}

func TestQualitySelectionFromPlayback(t *testing.T) {
	pb := &model.Playback{Streams: []model.Stream{
		{Definition: "360p", URL: "https://media.invalid/360"},
		{Definition: "1080p", URL: "https://media.invalid/1080"},
	}}

	selection := qualitySelectionFromPlayback(pb, "720p")
	if selection.Selected != "1080p" {
		t.Fatalf("nearest selection = %q, want 1080p", selection.Selected)
	}

	active := 0
	for _, opt := range selection.Options {
		if opt.ID != "720p" && opt.ID != "1080p" && opt.ID != "360p" && opt.Available {
			t.Fatalf("unexpected quality %q marked available", opt.ID)
		}
		if opt.Active {
			active++
			if opt.ID != "1080p" {
				t.Fatalf("active option = %q, want 1080p", opt.ID)
			}
		}
	}
	if active != 1 {
		t.Fatalf("active options = %d", active)
	}
}

func TestQualitySelectionCarriesStreamDiagnostics(t *testing.T) {
	pb := &model.Playback{Streams: []model.Stream{{
		Definition: "720p",
		URL:        "https://media.invalid/720",
		Codec:      "h264",
		Width:      1280,
		Height:     720,
		Bitrate:    1_500_000,
	}}}

	selection := qualitySelectionFromPlayback(pb, "720p")
	option := selection.Options[1]
	if !option.Available || !option.Active {
		t.Fatalf("unexpected option state: %+v", option)
	}
	if option.Codec != "h264" || option.Width != 1280 || option.Height != 720 || option.Bitrate != 1_500_000 {
		t.Fatalf("stream diagnostics were not propagated: %+v", option)
	}
}

func TestSelectVideoStreamUsesNearestDefinition(t *testing.T) {
	pb := &model.Playback{Streams: []model.Stream{
		{Definition: "360p", URL: "https://media.invalid/360"},
		{Definition: "1080p", URL: "https://media.invalid/1080"},
	}}

	stream, err := selectVideoStream(pb, "480p")
	if err != nil {
		t.Fatalf("selectVideoStream err = %v", err)
	}
	if stream.Definition != "360p" {
		t.Fatalf("selected definition = %q", stream.Definition)
	}
}

func TestSelectVideoStreamNilPlaybackReturnsError(t *testing.T) {
	_, err := selectVideoStream(nil, "720p")
	if err == nil {
		t.Fatal("selectVideoStream(nil) should return error, got nil")
	}
	if !strings.Contains(err.Error(), "playback is unavailable") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestVideoQualityFallbackOrderPrefersLowerQualities(t *testing.T) {
	got := videoQualityFallbackOrder("720p")
	want := []string{"720p", "540p", "480p", "360p", "1080p"}
	if len(got) != len(want) {
		t.Fatalf("fallback order length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fallback order = %v, want %v", got, want)
		}
	}
}

func TestRemoveLegacyPreviewsAndWarmups(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "ep1_preview_v2.mp4")
	warmup := filepath.Join(dir, "ep1_warm_v2.mp4")
	for _, path := range []string{legacy, warmup} {
		if err := os.WriteFile(path, []byte("media"), 0644); err != nil {
			t.Fatalf("write %s failed: %v", path, err)
		}
	}

	removeLegacyPreviews(dir)
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy preview still exists (err=%v)", err)
	}
	if _, err := os.Stat(warmup); !os.IsNotExist(err) {
		t.Fatalf("legacy warmup still exists (err=%v)", err)
	}
}

func TestPrefetchEpisodeQueuesTask(t *testing.T) {
	ms := &MediaServer{
		cacheDir:       t.TempDir(),
		prefetchQ:      make(chan prefetchTask, 10),
		prefetchQueued: make(map[string]prefetchTask),
		prefetchActive: make(map[string]*prefetchExecution),
	}

	ms.PrefetchEpisode("series1", "ep1", 1, "720p")
	select {
	case task := <-ms.prefetchQ:
		if task.seriesID != "series1" || task.episodeID != "ep1" || task.number != 1 || task.quality != "720p" {
			t.Fatalf("unexpected task: %+v", task)
		}
	default:
		t.Fatal("expected prefetch task in queue")
	}
}

func TestServeEpisodeFileSupportsRangeRequests(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "episode_v2.mp4")
	data := make([]byte, 2048)
	if err := os.WriteFile(mediaPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	ms := &MediaServer{cacheDir: dir}
	request := httptest.NewRequest("GET", "/video", nil)
	request.Header.Set("Range", "bytes=0-99")
	response := httptest.NewRecorder()

	ms.serveEpisodeFile(response, request, mediaPath)
	if response.Code != http.StatusPartialContent {
		t.Fatalf("range response code = %d, want %d", response.Code, http.StatusPartialContent)
	}
	if got := response.Header().Get("Content-Range"); got != "bytes 0-99/2048" {
		t.Fatalf("content range = %q", got)
	}
	if response.Body.Len() != 100 {
		t.Fatalf("range body length = %d, want 100", response.Body.Len())
	}
}

func TestEvictDirLRU(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "old.mp4")
	file2 := filepath.Join(dir, "mid.mp4")
	file3 := filepath.Join(dir, "new.mp4")
	data := make([]byte, 100)

	for _, file := range []string{file1, file2, file3} {
		if err := os.WriteFile(file, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Now().Add(-10 * time.Minute)
	midTime := time.Now().Add(-5 * time.Minute)
	newTime := time.Now()
	_ = os.Chtimes(file1, oldTime, oldTime)
	_ = os.Chtimes(file2, midTime, midTime)
	_ = os.Chtimes(file3, newTime, newTime)

	evictDirLRU(dir, 250, 150, nil)
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Errorf("expected file1 to be evicted, err: %v", err)
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Errorf("expected file2 to be evicted, err: %v", err)
	}
	if _, err := os.Stat(file3); err != nil {
		t.Errorf("expected file3 to remain, err: %v", err)
	}
}

func TestEvictDirLRUSkipsPinnedFile(t *testing.T) {
	dir := t.TempDir()
	pinned := filepath.Join(dir, "pinned.mp4")
	evictable := filepath.Join(dir, "evictable.mp4")
	data := make([]byte, 100)
	if err := os.WriteFile(pinned, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evictable, data, 0644); err != nil {
		t.Fatal(err)
	}

	evictDirLRU(dir, 150, 50, map[string]struct{}{pinned: {}})
	if _, err := os.Stat(pinned); err != nil {
		t.Fatalf("expected pinned file to remain: %v", err)
	}
	if _, err := os.Stat(evictable); !os.IsNotExist(err) {
		t.Fatalf("expected evictable file to be removed, err: %v", err)
	}
}

func TestPrefetchEpisodeDeduplicatesAndReplacesWindow(t *testing.T) {
	ms := &MediaServer{
		cacheDir:       t.TempDir(),
		prefetchQ:      make(chan prefetchTask, 8),
		prefetchQueued: make(map[string]prefetchTask),
		prefetchActive: make(map[string]*prefetchExecution),
	}

	ms.PrefetchEpisode("series1", "ep1", 1, "720p")
	ms.PrefetchEpisode("series1", "ep1", 1, "720p")
	if len(ms.prefetchQ) != 1 {
		t.Fatalf("expected one queued task, got %d", len(ms.prefetchQ))
	}

	ms.ReplacePrefetchWindow([]prefetchTask{
		{seriesID: "series2", episodeID: "ep2", number: 1, quality: "720p"},
	})
	if len(ms.prefetchQ) != 1 {
		t.Fatalf("expected replacement to leave one task, got %d", len(ms.prefetchQ))
	}
	task := <-ms.prefetchQ
	if task.episodeID != "ep2" || task.seriesID != "series2" {
		t.Fatalf("unexpected replacement task: %+v", task)
	}
}

func TestReplacePrefetchWindowKeepsPriorityOrder(t *testing.T) {
	ms := &MediaServer{
		cacheDir:       t.TempDir(),
		prefetchQ:      make(chan prefetchTask, 4),
		prefetchQueued: make(map[string]prefetchTask),
		prefetchActive: make(map[string]*prefetchExecution),
	}

	ms.ReplacePrefetchWindow([]prefetchTask{
		{seriesID: "series3", episodeID: "ep3", quality: "720p", priority: 3},
		{seriesID: "series1", episodeID: "ep1", quality: "720p", priority: 1},
		{seriesID: "series2", episodeID: "ep2", quality: "720p", priority: 2},
	})

	want := []string{"ep1", "ep2", "ep3"}
	for _, episodeID := range want {
		select {
		case task := <-ms.prefetchQ:
			if task.episodeID != episodeID {
				t.Fatalf("task order = %s, want %s", task.episodeID, episodeID)
			}
		default:
			t.Fatalf("expected task %s in queue", episodeID)
		}
	}
}

func TestMediaNetworkSnapshotProfilesAndRecentFailures(t *testing.T) {
	ms := &MediaServer{
		networkBytesPerSec: 2 * 1024 * 1024,
		networkSamples:     3,
	}
	if profile, _, _ := ms.mediaNetworkSnapshot(); profile != "strong" {
		t.Fatalf("profile = %q, want strong", profile)
	}

	ms.networkErrors = 1
	ms.networkLastErrorAt = time.Now()
	if profile, _, failures := ms.mediaNetworkSnapshot(); profile != "medium" || failures != 1 {
		t.Fatalf("profile = %q, failures = %d, want medium and 1", profile, failures)
	}
}

func TestReplacePrefetchWindowProtectionTasksDoNotQueueDownloads(t *testing.T) {
	ms := &MediaServer{
		cacheDir:       t.TempDir(),
		prefetchQ:      make(chan prefetchTask, 4),
		prefetchQueued: make(map[string]prefetchTask),
		prefetchActive: make(map[string]*prefetchExecution),
	}

	ms.ReplacePrefetchWindow([]prefetchTask{
		{seriesID: "series1", episodeID: "current", number: 1, quality: "720p", protectOnly: true},
		{seriesID: "series2", episodeID: "far-next", number: 1, quality: "720p", prefetchOnly: true},
	})
	if len(ms.prefetchQ) != 0 {
		t.Fatalf("protection-only task queued background work: %+v", <-ms.prefetchQ)
	}
}

func TestRunPrefetchTaskPrefetchOnlyDoesNotDownload(t *testing.T) {
	ms := &MediaServer{}
	ms.runPrefetchTask(context.Background(), prefetchTask{
		seriesID:     "series1",
		episodeID:    "episode1",
		number:       1,
		quality:      "720p",
		prefetchOnly: true,
	}, "prefetch-only")
	if ms.fullDownloads.Load() != 0 {
		t.Fatalf("prefetch-only task started a full download: n=%d", ms.fullDownloads.Load())
	}
}

func TestReplacePrefetchWindowPreservesNewestRendererStream(t *testing.T) {
	oldCanceled := make(chan struct{})
	oldStream := &progressiveStream{
		key:           "episode\x00old\x00720p",
		startedAt:     time.Now().Add(-time.Minute),
		rendererOwned: true,
		cancel: func() {
			select {
			case <-oldCanceled:
			default:
				close(oldCanceled)
			}
		},
	}
	nextDone := make(chan struct{})
	close(nextDone)
	nextStream := &hlsStream{
		key:           "episode\x00next\x00720p",
		startedAt:     time.Now(),
		rendererOwned: true,
		cancel:        func() {},
		done:          nextDone,
	}
	ms := &MediaServer{
		prefetchQ:          make(chan prefetchTask, 4),
		prefetchQueued:     make(map[string]prefetchTask),
		prefetchActive:     make(map[string]*prefetchExecution),
		progressiveStreams: map[string]*progressiveStream{oldStream.key: oldStream},
		hlsStreams:         map[string]*hlsStream{nextStream.key: nextStream},
	}

	ms.ReplacePrefetchWindow([]prefetchTask{
		{seriesID: "series-current", episodeID: "current", quality: "720p", protectOnly: true},
	})
	select {
	case <-oldCanceled:
	default:
		t.Fatal("old renderer stream was not canceled")
	}
	ms.hlsMu.Lock()
	_, retained := ms.hlsStreams[nextStream.key]
	ms.hlsMu.Unlock()
	if !retained {
		t.Fatal("newest renderer-owned HLS stream was not retained")
	}
}
