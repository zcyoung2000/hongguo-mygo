package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeHLSFixture(t *testing.T, dir string, complete bool) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	playlist := "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2,\nseg000.m4s\n"
	if complete {
		playlist += "#EXT-X-ENDLIST\n"
	}
	for name, content := range map[string]string{"index.m3u8": playlist, "init.mp4": "init", "seg000.m4s": "media"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "index.m3u8")
}

func TestClearCacheInvalidatesCompletedHLSAndKeepsActiveMedia(t *testing.T) {
	ms := &MediaServer{cacheDir: t.TempDir(), hlsStreams: make(map[string]*hlsStream)}
	for i, id := range []string{"710000000000001", "710000000000002", "710000000000003", "710000000000004"} {
		dir := ms.hlsCacheDir(id, "720p")
		playlist := writeHLSFixture(t, dir, true)
		key := PrefetchTaskKey(PrefetchTask{EpisodeID: id, Quality: "720p"})
		stream := &hlsStream{key: key, dir: dir, playlistPath: playlist, done: make(chan struct{}), cancel: func() {}}
		if i != 1 {
			stream.finish(nil)
		}
		if i == 2 {
			stream.touchMediaRequest()
		}
		ms.hlsStreams[key] = stream
	}
	image := filepath.Join(ms.cacheDir, "image.jpg")
	if err := os.WriteFile(image, []byte("active image"), 0644); err != nil {
		t.Fatal(err)
	}
	release := ms.pinCacheFile(image)
	releaseHLS := ms.pinCacheFile(ms.hlsCacheDir("710000000000004", "720p"))
	cleared, err := ms.ClearCache()
	if err != nil || cleared == 0 {
		t.Fatalf("ClearCache = %d, %v", cleared, err)
	}
	stale := PrefetchTaskKey(PrefetchTask{EpisodeID: "710000000000001", Quality: "720p"})
	if ms.hlsStreams[stale] != nil || ms.HasCachedHLS("710000000000001", "720p") {
		t.Fatal("completed stale HLS survived cache clear")
	}
	for _, id := range []string{"710000000000002", "710000000000003", "710000000000004"} {
		if !ms.HasCachedHLS(id, "720p") {
			t.Fatalf("active or recently played HLS %s was deleted", id)
		}
	}
	if _, err := os.Stat(image); err != nil {
		t.Fatal("pinned file was deleted")
	}
	release()
	releaseHLS()
	if _, err := ms.ClearCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(image); !os.IsNotExist(err) {
		t.Fatal("released file was not cleared")
	}
	if ms.HasCachedHLS("710000000000004", "720p") {
		t.Fatal("released HLS was not cleared")
	}
}

func TestHLSStartupCleanupKeepsOnlyCompleteRenditions(t *testing.T) {
	root := t.TempDir()
	complete := filepath.Join(root, "710000000000001_720p")
	partial := filepath.Join(root, "710000000000002_720p")
	broken := filepath.Join(root, "710000000000003_720p")
	writeHLSFixture(t, complete, true)
	writeHLSFixture(t, partial, false)
	writeHLSFixture(t, broken, true)
	if err := os.Remove(filepath.Join(broken, "seg000.m4s")); err != nil {
		t.Fatal(err)
	}
	if err := pruneIncompleteHLSCache(root); err != nil {
		t.Fatal(err)
	}
	if !hlsPlaylistComplete(filepath.Join(complete, "index.m3u8")) {
		t.Fatal("startup removed complete HLS")
	}
	for _, dir := range []string{partial, broken} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("invalid cache %s survived", dir)
		}
	}
	// Reuse an on-disk complete rendition with no SDK client or ffmpeg: this
	// exercises the exact startup/reopen path without contacting an upstream.
	ms := &MediaServer{cacheDir: t.TempDir()}
	id := "710000000000004"
	writeHLSFixture(t, ms.hlsCacheDir(id, "720p"), true)
	if err := pruneIncompleteHLSCache(filepath.Join(ms.cacheDir, "episodes", "hls")); err != nil {
		t.Fatal(err)
	}
	stream, err := ms.StartHLSStream("series", id, 1, "720p")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := stream.wait(ctx); err != nil {
		t.Fatal(err)
	}
	if ms.hlsStarts.Load() != 0 {
		t.Fatal("cached rendition started a download")
	}
}

func TestFeedRetirementPreservesCacheCompletedBeforeWriterExit(t *testing.T) {
	for _, mode := range []string{"already complete", "completes during retirement", "interrupted"} {
		t.Run(mode, func(t *testing.T) {
			ms := &MediaServer{cacheDir: t.TempDir(), hlsStreams: make(map[string]*hlsStream)}
			id := "710000000000001"
			dir := ms.hlsCacheDir(id, "720p")
			playlist := writeHLSFixture(t, dir, mode == "already complete")
			key := PrefetchTaskKey(PrefetchTask{EpisodeID: id, Quality: "720p"})
			stream := &hlsStream{key: key, dir: dir, playlistPath: playlist, done: make(chan struct{}), cancel: func() {}}
			ms.hlsStreams[key] = stream
			ms.SetFeedPreloadWindow(1, id, "720p", nil)
			ms.SetFeedPreloadWindow(2, "710000000000002", "720p", nil)
			if stream.retired == nil {
				t.Fatal("old foreground writer was not retired")
			}
			if mode == "completes during retirement" {
				if err := appendHLSEndList(playlist); err != nil {
					t.Fatal(err)
				}
			}
			stream.finish(nil)
			select {
			case <-stream.retired:
			case <-time.After(2 * time.Second):
				t.Fatal("retirement did not drain")
			}
			if mode == "interrupted" {
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Fatal("interrupted writer's files survived retirement")
				}
				return
			}
			if !ms.HasCachedHLS(id, "720p") {
				t.Fatal("backward navigation lost a completed rendition")
			}
			cached, err := ms.StartHLSStream("series", id, 1, "720p")
			if err != nil || cached == stream || ms.hlsStarts.Load() != 0 {
				t.Fatalf("revisit must use disk without SDK or conversion: stream=%p err=%v starts=%d", cached, err, ms.hlsStarts.Load())
			}
		})
	}
}
