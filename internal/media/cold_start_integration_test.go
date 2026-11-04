package media

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestColdFragmentsDecodeBeforeNextKeyframeAndKeepCacheWithActiveReader(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real ffmpeg")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	fixture := filepath.Join(t.TempDir(), "long-gop.mp4")
	output, err := exec.CommandContext(ctx, ffmpeg, "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x568:rate=24",
		"-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono", "-t", "24",
		"-c:v", "libx264", "-preset", "fast", "-b:v", "500k",
		"-g", "240", "-keyint_min", "240", "-sc_threshold", "0", "-bf", "3",
		"-c:a", "aac", "-movflags", "+faststart", fixture).CombinedOutput()
	if err != nil {
		t.Fatalf("generate long-GOP fixture: %v: %s", err, output)
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	var release sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &gatedMediaWriter{ResponseWriter: w, ctx: r.Context(), gate: gate, limit: len(data) / 4}
		http.ServeContent(writer, r, "source.mp4", time.Time{}, bytes.NewReader(data))
	}))
	defer upstream.Close()
	defer release.Do(func() { close(gate) })
	previous := ffmpegCommand
	ffmpegCommand = func() (string, error) { return ffmpeg, nil }
	defer func() { ffmpegCommand = previous }()
	ms := &MediaServer{cacheDir: t.TempDir(), pbCache: map[string]playbackCacheItem{
		"710000000000001": {pb: &model.Playback{Streams: []model.Stream{{Definition: "720p", URL: upstream.URL}}}, expiresAt: time.Now().Add(time.Minute)},
	}}
	path := episodeCachePath(ms.cacheDir, "710000000000001", "720p")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	stream, err := ms.startProgressiveStream("series", "710000000000001", 1, "720p", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ms.Close()
	// Keep a consumer attached after the producer finishes, as an MSE reader
	// can pause fetching while it has enough buffered video.
	stream.retainReader()
	defer stream.releaseReader()
	if err := stream.waitUntilContext(64*1024, ctx); err != nil {
		t.Fatal(err)
	}
	_, done, streamErr := stream.snapshot()
	if done || streamErr != nil {
		t.Fatalf("source must still be blocked before its second keyframe: done=%v error=%v", done, streamErr)
	}
	prefix, err := os.ReadFile(stream.tempPath)
	if err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(t.TempDir(), "prefix.mp4")
	if err := os.WriteFile(partial, prefix, 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", partial, "-frames:v", "1", "-f", "null", "-").CombinedOutput(); err != nil {
		t.Fatalf("no decodable cold-start frame before upstream EOF: %v: %s", err, output)
	}
	release.Do(func() { close(gate) })
	if err := stream.waitUntilContext(math.MaxInt64, ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !fileExists(path) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !fileExists(path) {
		t.Fatal("completed file was withheld until the slow consumer disconnected")
	}
	starts := ms.progressiveStarts.Load()
	request := httptest.NewRequest(http.MethodGet, "/video?mode=progressive", nil)
	request.Header.Set("Range", "bytes=0-1")
	response := httptest.NewRecorder()
	ms.serveProgressiveVideo(response, request, "series", "710000000000001", 1, "720p", path)
	if response.Code != http.StatusPartialContent || response.Body.Len() != 2 || strings.HasSuffix(response.Header().Get("Content-Range"), "/*") {
		t.Fatalf("cached replay must advertise the real complete representation: %d %v", response.Code, response.Header())
	}
	if ms.progressiveStarts.Load() != starts {
		t.Fatal("replay started another conversion while the original consumer was attached")
	}
	file, err := openProgressiveFile(stream)
	if err != nil {
		t.Fatalf("active reader lost its file after cache publication: %v", err)
	}
	defer file.Close()
	if _, err := io.ReadFull(file, make([]byte, 8)); err != nil {
		t.Fatal(err)
	}
}
