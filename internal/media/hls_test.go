package media

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

func TestHLSMasterURLUsesPathEndpoints(t *testing.T) {
	ms := &MediaServer{port: 12345}
	got := ms.HLSMasterURL("710000000000001", "series1", 1, "720p")
	if !strings.Contains(got, "http://127.0.0.1:12345/hls/710000000000001/720p/master.m3u8?") {
		t.Fatalf("HLS master URL = %q", got)
	}
}

func TestHLSPlaylistURLUsesSelectedRendition(t *testing.T) {
	ms := &MediaServer{port: 12345}
	got := ms.HLSPlaylistURL("episode1", "series1", 1, "540p")
	if !strings.Contains(got, "http://127.0.0.1:12345/hls/episode1/540p/index.m3u8?") ||
		strings.Contains(got, "master.m3u8") {
		t.Fatalf("HLS playlist URL = %q", got)
	}
}

func TestEvictHLSCacheKeepsPinnedActiveRendition(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "old_720p")
	newDir := filepath.Join(root, "new_720p")
	for _, dir := range []string{oldDir, newDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "seg000.m4s"), make([]byte, 100), 0644); err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newDir, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	evictHLSCache(root, 99, 50, map[string]struct{}{newDir: {}})
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old HLS rendition was not evicted: err=%v", err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("active HLS rendition was evicted: %v", err)
	}
}

func TestParseHLSPath(t *testing.T) {
	episodeID, quality, resource, ok := parseHLSPath("/hls/710000000000001/720p/index.m3u8")
	if !ok || episodeID != "710000000000001" || quality != "720p" || resource != "index.m3u8" {
		t.Fatalf("parseHLSPath = %q, %q, %q, %v", episodeID, quality, resource, ok)
	}
}

func TestFFmpegHLSArgsUseCMAFSegments(t *testing.T) {
	args := ffmpegHLSArgs("https://media.invalid/video", "https://media.invalid/audio", "key", "/tmp/hls")
	joined := strings.Join(args, " ")
	for _, required := range []string{"-f hls", "-hls_time 2", "-hls_segment_type fmp4", "independent_segments"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("HLS args missing %q: %v", required, args)
		}
	}
}

func TestServeHLSMasterIncludesVariants(t *testing.T) {
	ms := &MediaServer{
		pbCache: map[string]playbackCacheItem{
			"710000000000001": {
				pb: &model.Playback{Streams: []model.Stream{
					{Definition: "1080p", URL: "https://media.invalid/1080", Width: 1920, Height: 1080, Bitrate: 3000000, Codec: "h264"},
					{Definition: "540p", URL: "https://media.invalid/540", Width: 960, Height: 540, Bitrate: 1000000, Codec: "h264"},
				}},
				expiresAt: time.Now().Add(time.Minute),
			},
		},
	}
	request := httptest.NewRequest(http.MethodGet, "/hls/710000000000001/720p/master.m3u8", nil)
	response := httptest.NewRecorder()

	ms.serveHLSMaster(response, request, "series1", 1, "710000000000001", "720p")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, "1080p/index.m3u8") {
		t.Fatalf("master offered rendition above requested ceiling: %s", body)
	}
	if !strings.Contains(body, "540p/index.m3u8") {
		t.Fatalf("master omitted variants: %s", body)
	}
	if !strings.Contains(body, "series_id=series1") || !strings.Contains(body, "num=1") {
		t.Fatalf("master dropped variant context: %s", body)
	}
	if strings.Contains(body, "CODECS=") {
		t.Fatalf("master emitted guessed codec metadata: %s", body)
	}
}

func TestEstimatedHLSBandwidthIncludesSeparateAudio(t *testing.T) {
	if got := estimatedHLSBandwidth(1_000_000, 720, true); got != 1_128_000 {
		t.Fatalf("estimated bandwidth = %d, want 1128000", got)
	}
	if got := estimatedHLSBandwidth(0, 540, false); got != 54_000_000 {
		t.Fatalf("estimated fallback bandwidth = %d, want 54000000", got)
	}
}

func TestHLSCodecAttributeAvoidsGuesses(t *testing.T) {
	if got := hlsCodecAttribute("avc1.640028", false); got != `"avc1.640028"` {
		t.Fatalf("valid codec = %q", got)
	}
	if got := hlsCodecAttribute("h264", false); got != "" {
		t.Fatalf("generic codec was guessed: %q", got)
	}
	if got := hlsCodecAttribute("avc1.640028", true); got != "" {
		t.Fatalf("video-only codec was emitted for muxed audio: %q", got)
	}
}

func TestServeHLSPlaylistUsesReadyCache(t *testing.T) {
	dir := t.TempDir()
	playlist := filepath.Join(dir, "index.m3u8")
	if err := writeTestFile(playlist, "#EXTM3U\n#EXTINF:2.000000,\nseg000.m4s\n#EXT-X-ENDLIST\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(filepath.Join(dir, "seg000.m4s"), strings.Repeat("x", 1025)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	close(done)
	streamKey := PrefetchTaskKey(PrefetchTask{EpisodeID: "710000000000001", Quality: "720p"})
	stream := &hlsStream{key: streamKey, dir: dir, playlistPath: playlist, done: done}
	ms := &MediaServer{
		cacheDir:   filepath.Dir(filepath.Dir(dir)),
		hlsStreams: map[string]*hlsStream{stream.key: stream},
	}
	request := httptest.NewRequest(http.MethodGet, "/hls/710000000000001/720p/index.m3u8?series_id=series1&num=1", nil)
	response := httptest.NewRecorder()

	if !ms.serveHLSPlaylist(response, request, "series1", "710000000000001", 1, "720p") {
		t.Fatalf("playlist handler failed: %s", response.Body.String())
	}
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "seg000.m4s") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func TestHLSStreamWaitReturnsStoredError(t *testing.T) {
	stream := &hlsStream{done: make(chan struct{})}
	stream.finish(context.Canceled)
	if err := stream.wait(context.Background()); err != context.Canceled {
		t.Fatalf("wait err = %v, want context.Canceled", err)
	}
}

func TestParseHLSPathRejectsTraversal(t *testing.T) {
	for _, path := range []string{
		"/hls/..%2F..%2Fsecret/720p/index.m3u8",
		"/hls/abc/720p/index.m3u8",
		"/hls/710000000000001/unknown/index.m3u8",
	} {
		if _, _, _, ok := parseHLSPath(path); ok {
			t.Fatalf("parseHLSPath(%q) accepted invalid path", path)
		}
	}
}

func TestHLSMasterMapsUnknownRankToNearestQuality(t *testing.T) {
	ms := &MediaServer{pbCache: map[string]playbackCacheItem{
		"710000000000001": {pb: &model.Playback{Streams: []model.Stream{
			{Definition: "", URL: "https://media.invalid/unknown", Width: 360, Height: 640},
		}}, expiresAt: time.Now().Add(time.Minute)},
	}}
	response := httptest.NewRecorder()
	ms.serveHLSMaster(response, httptest.NewRequest(http.MethodGet, "/hls/710000000000001/720p/master.m3u8", nil), "series", 1, "710000000000001", "720p")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "720p/index.m3u8") {
		t.Fatalf("master response = %d/%s", response.Code, response.Body.String())
	}
}
