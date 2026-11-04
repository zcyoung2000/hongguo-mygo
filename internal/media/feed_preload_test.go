package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFeedPreloadStatusCountsOnlyPublishedPlayableSegments(t *testing.T) {
	ms := &MediaServer{cacheDir: t.TempDir()}
	task := PrefetchTask{EpisodeID: "710000000000001", Quality: "540p"}
	dir := ms.hlsCacheDir(task.EpisodeID, task.Quality)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	playlist := "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2.5,\nseg000.m4s\n#EXTINF:3.5,\nseg001.m4s\n"
	for name, data := range map[string]string{
		"index.m3u8": playlist, "init.mp4": "init", "seg000.m4s": "segment", "seg999.m4s.tmp": "unpublished",
	} {
		if err := writeTestFile(filepath.Join(dir, name), data); err != nil {
			t.Fatal(err)
		}
	}
	if status := ms.feedPreloadStatus(task); status.CachedSeconds != 0 || status.Complete {
		t.Fatalf("missing published segment was counted as ready: %+v", status)
	}
	if err := writeTestFile(filepath.Join(dir, "seg001.m4s"), "segment"); err != nil {
		t.Fatal(err)
	}
	status := ms.feedPreloadStatus(task)
	if status.CachedSeconds != 6 || status.Complete || status.Quality != "540p" {
		t.Fatalf("prefix readiness = %+v", status)
	}
	if err := appendHLSEndList(filepath.Join(dir, "index.m3u8")); err != nil {
		t.Fatal(err)
	}
	if status := ms.feedPreloadStatus(task); !status.Complete || status.CachedSeconds != 6 {
		t.Fatalf("completed readiness = %+v", status)
	}
}

func TestFeedPreloadWindowPromotionAndBackpressureKeepCurrentAndCompleteCache(t *testing.T) {
	ms := &MediaServer{cacheDir: t.TempDir(), hlsStreams: make(map[string]*hlsStream)}
	canceled := make(map[string]bool)
	items := []FeedPreloadItem{
		{EpisodeID: "710000000000002", Quality: "540p"},
		{EpisodeID: "710000000000003", Quality: "720p"},
	}
	for _, item := range append([]FeedPreloadItem{{EpisodeID: "710000000000001", Quality: "720p"}}, items...) {
		key := PrefetchTaskKey(PrefetchTask{EpisodeID: item.EpisodeID, Quality: item.Quality})
		dir := ms.hlsCacheDir(item.EpisodeID, item.Quality)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string]string{
			"index.m3u8": "#EXTM3U\n#EXTINF:6,\nseg000.m4s\n#EXT-X-ENDLIST\n",
			"seg000.m4s": strings.Repeat("x", 2048),
		} {
			if err := writeTestFile(filepath.Join(dir, name), data); err != nil {
				t.Fatal(err)
			}
		}
		done := make(chan struct{})
		close(done)
		ms.hlsStreams[key] = &hlsStream{key: key, dir: dir, playlistPath: filepath.Join(dir, "index.m3u8"),
			done: done, cancel: func() { canceled[key] = true }, rendererOwned: true, lastMediaRequestAt: time.Now()}
	}
	ms.SetFeedPreloadWindow(1, "710000000000001", "720p", items)
	ms.SetFeedPreloadWindow(2, items[0].EpisodeID, items[0].Quality, items[1:])
	ms.SetFeedPreloadWindow(3, items[0].EpisodeID, items[0].Quality, nil)
	currentKey := PrefetchTaskKey(PrefetchTask{EpisodeID: items[0].EpisodeID, Quality: items[0].Quality})
	if canceled[currentKey] || len(canceled) != 2 {
		t.Fatalf("backpressure/promotion canceled wrong streams: %v", canceled)
	}
	if !hlsPlaylistComplete(filepath.Join(ms.hlsCacheDir(items[1].EpisodeID, items[1].Quality), "index.m3u8")) {
		t.Fatal("shrinking a window deleted the completed cache")
	}
	ms.SetFeedPreloadWindow(2, "710000000000001", "720p", items)
	if canceled[currentKey] || ms.feedPreloadCurrent != currentKey || ms.hlsStarts.Load() != 0 {
		t.Fatal("stale window displaced the protected current video")
	}
}

func TestFeedPreloadWindowRejectsInvalidEpisodeIDsWithoutStartingWork(t *testing.T) {
	ms := &MediaServer{cacheDir: t.TempDir()}
	items := []FeedPreloadItem{{EpisodeID: "../../outside"}, {EpisodeID: ""}, {EpisodeID: "invalid"}}
	if statuses := ms.SetFeedPreloadWindow(1, "", "720p", items); len(statuses) != 0 || ms.hlsStarts.Load() != 0 {
		t.Fatalf("invalid preloads admitted: %+v", statuses)
	}
}
