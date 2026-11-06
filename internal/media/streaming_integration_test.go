package media

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
)

// Hold the upstream connection halfway through the file. Streaming must expose
// playable media while this gate is CLOSED, rather than merely finish quickly.
type gatedMediaWriter struct {
	http.ResponseWriter
	ctx     context.Context
	gate    <-chan struct{}
	limit   int
	written int
}

func (w *gatedMediaWriter) Write(data []byte) (int, error) {
	total := 0
	for len(data) > 0 {
		if w.written >= w.limit {
			select {
			case <-w.gate:
			case <-w.ctx.Done():
				return total, w.ctx.Err()
			}
		}
		size := min(len(data), 16*1024)
		if w.written < w.limit {
			size = min(size, w.limit-w.written)
		}
		n, err := w.ResponseWriter.Write(data[:size])
		total += n
		w.written += n
		data = data[n:]
		w.ResponseWriter.(http.Flusher).Flush()
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func TestStreamingPublishesMediaBeforeUpstreamCompletes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real ffmpeg")
	}
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	fixture := filepath.Join(t.TempDir(), "source.mp4")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x568:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "18",
		"-c:v", "libx264", "-preset", "ultrafast", "-b:v", "600k", "-g", "24",
		"-c:a", "aac", "-movflags", "+faststart", fixture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate streaming fixture: %v: %s", err, output)
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}

	// Other tests use a fake ffmpeg for content-key checks. Keep this real
	// integration isolated from the process-wide executable discovery cache.
	previousCommand := ffmpegCommand
	ffmpegCommand = func() (string, error) { return ffmpegPath, nil }
	t.Cleanup(func() { ffmpegCommand = previousCommand })

	for _, mode := range []string{"hls", "progressive", "adaptive preloads"} {
		t.Run(mode, func(t *testing.T) {
			gate := make(chan struct{})
			var release sync.Once
			var upstreamRequests atomic.Int64
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamRequests.Add(1)
				writer := &gatedMediaWriter{ResponseWriter: w, ctx: r.Context(), gate: gate, limit: len(data) / 2}
				http.ServeContent(writer, r, "source.mp4", time.Time{}, bytes.NewReader(data))
			}))
			defer upstream.Close()
			defer release.Do(func() { close(gate) })
			ms := &MediaServer{cacheDir: t.TempDir(), pbCache: make(map[string]playbackCacheItem)}
			if err := os.MkdirAll(filepath.Join(ms.cacheDir, "episodes"), 0755); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"710000000000001", "710000000000002", "710000000000003", "710000000000004"} {
				ms.pbCache[id] = playbackCacheItem{
					pb:        &model.Playback{Streams: []model.Stream{{Definition: "720p", URL: upstream.URL}}},
					expiresAt: time.Now().Add(time.Minute),
				}
			}
			if mode == "adaptive preloads" {
				testAdaptivePreloadStreaming(t, ms, func() { release.Do(func() { close(gate) }) })
			} else if mode == "hls" {
				partialDir := ms.hlsCacheDir("710000000000002", "720p")
				if err := os.MkdirAll(partialDir, 0755); err != nil {
					t.Fatal(err)
				}
				// Simulate a previous app exit after only its first segment. This
				// must start a new writer, not be reused as a finished rendition.
				for name, content := range map[string]string{
					"index.m3u8": "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2,\nseg000.m4s\n",
					"init.mp4":   "interrupted-init",
					"seg000.m4s": strings.Repeat("x", 2048),
					"seg999.m4s": "stale-segment",
				} {
					if err := os.WriteFile(filepath.Join(partialDir, name), []byte(content), 0644); err != nil {
						t.Fatal(err)
					}
				}
				var streams []*hlsStream
				for _, id := range []string{"710000000000001", "710000000000002"} {
					stream, err := ms.StartHLSStream("series", id, 1, "720p")
					if err != nil {
						t.Fatal(err)
					}
					streams = append(streams, stream)
				}
				defer func() {
					release.Do(func() { close(gate) })
					for _, stream := range streams {
						stream.cancel()
						<-stream.done
					}
				}()
				deadline := time.Now().Add(8 * time.Second)
				for _, stream := range streams {
					for !hlsPlaylistReady(stream.playlistPath) && time.Now().Before(deadline) {
						time.Sleep(20 * time.Millisecond)
					}
					if !hlsPlaylistReady(stream.playlistPath) {
						t.Fatal("current and next video must BOTH publish segments before either download completes")
					}
					if hlsPlaylistComplete(stream.playlistPath) {
						t.Fatal("upstream gate failed: stream is already complete")
					}
					select {
					case <-stream.done:
						t.Fatal("an interrupted cache was incorrectly marked complete")
					default:
					}
					playlist, _ := os.ReadFile(stream.playlistPath)
					t.Logf("published %d segments while upstream remains blocked", strings.Count(string(playlist), "#EXTINF"))
				}
				if mediaFileExists(filepath.Join(partialDir, "seg999.m4s")) {
					t.Fatal("stale segments survived an interrupted rendition restart")
				}
				// Decode a real frame from just init + segment 1 while the
				// upstream is still held. Nonempty output alone is not enough.
				init, err := os.ReadFile(filepath.Join(streams[0].dir, "init.mp4"))
				if err != nil {
					t.Fatal(err)
				}
				segment, err := os.ReadFile(filepath.Join(streams[0].dir, "seg000.m4s"))
				if err != nil {
					t.Fatal(err)
				}
				prefix := filepath.Join(t.TempDir(), "first-segment.mp4")
				if err := os.WriteFile(prefix, append(init, segment...), 0644); err != nil {
					t.Fatal(err)
				}
				decodeCtx, cancelDecode := context.WithTimeout(context.Background(), 5*time.Second)
				output, decodeErr := exec.CommandContext(decodeCtx, ffmpegPath, "-v", "error", "-i", prefix,
					"-frames:v", "1", "-f", "null", "-").CombinedOutput()
				cancelDecode()
				if decodeErr != nil {
					t.Fatalf("first segment is not playable before upstream EOF: %v: %s", decodeErr, output)
				}
				// Rapidly cancel and revisit the next video while it is still
				// downloading. Cleanup from the old writer must precede the new one.
				previous := streams[1]
				ms.retainHLSStreams(map[string]PrefetchTask{streams[0].key: {}}, streams[0].key)
				restarted, err := ms.StartHLSStream("series", "710000000000002", 1, "720p")
				if err != nil || restarted == previous {
					t.Fatalf("canceled rendition was not restarted: %v", err)
				}
				streams[1] = restarted
				deadline = time.Now().Add(8 * time.Second)
				for !hlsPlaylistReady(restarted.playlistPath) && time.Now().Before(deadline) {
					time.Sleep(20 * time.Millisecond)
				}
				if !hlsPlaylistReady(restarted.playlistPath) {
					t.Fatal("old cleanup deleted the restarted rendition")
				}
				// The local media endpoint must serve this growing playlist too.
				response := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodGet, "/hls/710000000000001/720p/index.m3u8", nil)
				ms.handleHLS(response, request)
				if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "TIME-OFFSET=0") {
					t.Fatalf("growing playlist response: %d %s", response.Code, response.Body.String())
				}
				release.Do(func() { close(gate) })
				for _, stream := range streams {
					waitCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
					err := stream.wait(waitCtx)
					cancel()
					if err != nil || !hlsPlaylistComplete(stream.playlistPath) {
						t.Fatalf("completed rendition: %v", err)
					}
				}
				ms.retainHLSStreams(map[string]PrefetchTask{streams[0].key: {}}, streams[0].key)
				if !hlsPlaylistComplete(restarted.playlistPath) {
					t.Fatal("window replacement deleted a completed reusable cache")
				}
				cached, err := ms.StartHLSStream("series", "710000000000002", 1, "720p")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := ms.ClearCache(); err != nil {
					t.Fatal(err)
				}
				regenerated, err := ms.StartHLSStream("series", "710000000000002", 1, "720p")
				if err != nil || regenerated == cached {
					t.Fatalf("cache clear reused deleted output: %v", err)
				}
				waitCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				err = regenerated.wait(waitCtx)
				cancel()
				if err != nil || !hlsPlaylistComplete(regenerated.playlistPath) {
					t.Fatalf("cache clear did not regenerate HLS: %v", err)
				}
				starts, requests := ms.hlsStarts.Load(), upstreamRequests.Load()
				// Replay through the actual local endpoint while alternating the
				// foreground owner. A cached return must never contact upstream.
				for version := int64(1); version <= 6; version++ {
					id := "710000000000001"
					if version%2 == 0 {
						id = "710000000000002"
					}
					ms.SetFeedPreloadWindow(version, id, "720p", nil)
					response := httptest.NewRecorder()
					ms.handleHLS(response, httptest.NewRequest(http.MethodGet, "/hls/"+id+"/720p/index.m3u8", nil))
					if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "#EXT-X-ENDLIST") {
						t.Fatalf("cached return failed: %d %s", response.Code, response.Body.String())
					}
					if ms.hlsStarts.Load() != starts || upstreamRequests.Load() != requests {
						t.Fatalf("cached return restarted work: conversions %d -> %d, requests %d -> %d", starts, ms.hlsStarts.Load(), requests, upstreamRequests.Load())
					}
				}
			} else {
				local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ms.serveProgressiveVideo(w, r, "series", "710000000000001", 1, "720p",
						episodeCachePath(ms.cacheDir, "710000000000001", "720p"))
				}))
				defer local.Close()
				defer release.Do(func() { close(gate) })
				client := &http.Client{Timeout: 8 * time.Second}
				response, err := client.Get(local.URL)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				head := make([]byte, 32*1024)
				if _, err := io.ReadFull(response.Body, head); err != nil {
					t.Fatalf("MP4 must deliver media bytes before EOF: %v", err)
				}
				if !bytes.Contains(head, []byte("moov")) || !bytes.Contains(head, []byte("moof")) {
					t.Fatal("progressive MP4 has no initialization and media fragments")
				}
				release.Do(func() { close(gate) })
				if _, err := io.Copy(io.Discard, response.Body); err != nil {
					t.Fatal(err)
				}
				// The HTTP body can finish just before the writer atomically
				// renames its cache. Wait for that background commit before the
				// test's TempDir cleanup removes the containing directory.
				finalPath := episodeCachePath(ms.cacheDir, "710000000000001", "720p")
				deadline := time.Now().Add(5 * time.Second)
				for !fileExists(finalPath) && time.Now().Before(deadline) {
					time.Sleep(10 * time.Millisecond)
				}
				if !fileExists(finalPath) {
					t.Fatal("progressive cache was not committed after EOF")
				}
			}
		})
	}
}

func testAdaptivePreloadStreaming(t *testing.T, ms *MediaServer, release func()) {
	t.Helper()
	current, err := ms.StartHLSStream("series", "710000000000001", 1, "720p")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		release()
		ms.hlsMu.Lock()
		streams := make([]*hlsStream, 0, len(ms.hlsStreams))
		for _, stream := range ms.hlsStreams {
			stream.cancel()
			streams = append(streams, stream)
		}
		ms.hlsMu.Unlock()
		for _, stream := range streams {
			<-stream.done
			if stream.retired != nil {
				<-stream.retired
			}
		}
	}()
	items := []FeedPreloadItem{
		{SeriesID: "next", EpisodeID: "710000000000002", Number: 1, Quality: "720p"},
		{SeriesID: "far", EpisodeID: "710000000000003", Number: 1, Quality: "720p"},
		{SeriesID: "farthest", EpisodeID: "710000000000004", Number: 1, Quality: "720p"},
	}
	var version int64
	var statuses []FeedPreloadStatus
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		version++
		statuses = ms.SetFeedPreloadWindow(version, "710000000000001", "720p", items)
		if len(statuses) != 2 {
			t.Fatalf("preload window must admit at most two videos: %+v", statuses)
		}
		if statuses[1].Active && !statuses[0].Complete && statuses[0].CachedSeconds < 6 {
			t.Fatal("second-next started before the nearest video had a playable prefix")
		}
		if statuses[1].CachedSeconds >= 6 && hlsPlaylistReady(current.playlistPath) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(statuses) != 2 || statuses[0].CachedSeconds < 6 || statuses[1].CachedSeconds < 6 {
		t.Fatalf("next two videos must warm before the nearest finishes downloading: %+v", statuses)
	}
	if statuses[0].Complete || statuses[1].Complete || hlsPlaylistComplete(current.playlistPath) {
		t.Fatal("upstream completed before the adaptive scheduling assertions")
	}
	// Promoting the next video must reuse its existing writer and cache. Old
	// foreground and speculative work are canceled as the window moves/shrinks.
	key := PrefetchTaskKey(PrefetchTask{EpisodeID: items[0].EpisodeID, Quality: "720p"})
	ms.hlsMu.Lock()
	promoted := ms.hlsStreams[key]
	ms.hlsMu.Unlock()
	version++
	ms.SetFeedPreloadWindow(version, items[0].EpisodeID, "720p", items[1:2])
	version++
	ms.SetFeedPreloadWindow(version, items[0].EpisodeID, "720p", nil)
	select {
	case <-promoted.done:
		t.Fatal("backpressure canceled the video just promoted to current")
	default:
	}
	ms.hlsMu.Lock()
	if ms.hlsStreams[key] != promoted {
		ms.hlsMu.Unlock()
		t.Fatal("promotion restarted rather than reusing the warmed writer")
	}
	ms.hlsMu.Unlock()
	starts := ms.hlsStarts.Load()
	ms.SetFeedPreloadWindow(version-1, "710000000000001", "720p", items)
	if ms.hlsStarts.Load() != starts {
		t.Fatal("stale IPC window resurrected canceled background downloads")
	}
	release()
	waitCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := promoted.wait(waitCtx); err != nil || !hlsPlaylistComplete(promoted.playlistPath) {
		t.Fatalf("promoted video did not finish normally after backpressure: %v", err)
	}
	version++
	statuses = ms.SetFeedPreloadWindow(version, "710000000000001", "720p", items)
	if !statuses[0].Complete {
		t.Fatal("completed nearest cache was not reused")
	}
	farKey := PrefetchTaskKey(PrefetchTask{EpisodeID: items[1].EpisodeID, Quality: "720p"})
	ms.hlsMu.Lock()
	far := ms.hlsStreams[farKey]
	ms.hlsMu.Unlock()
	if far == nil {
		t.Fatal("second-next did not start after the nearest episode completed")
	}
	if err := far.wait(waitCtx); err != nil || !hlsPlaylistComplete(far.playlistPath) {
		t.Fatalf("second-next did not finish: %v", err)
	}
}

func TestHLSCanceledWhileQueuedDoesNotWaitForDownloadSlot(t *testing.T) {
	// Occupy all HLS slots without starting any external processes.
	for i := 0; i < cap(hlsSem); i++ {
		hlsSem <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(hlsSem); i++ {
			<-hlsSem
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	stream := &hlsStream{cancel: cancel, done: make(chan struct{})}
	cancel()
	ms := &MediaServer{}
	go ms.runHLSStream(ctx, stream, "series", "710000000000001", 1, "720p")
	select {
	case <-stream.done:
	case <-time.After(time.Second):
		t.Fatal("canceled prefetch is still blocking the media queue")
	}
}
