package media

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseHTTPByteRange(t *testing.T) {
	tests := []struct {
		name   string
		header string
		size   int64
		want   httpByteRange
		valid  bool
	}{
		{name: "open", header: "bytes=0-", size: 100, want: httpByteRange{start: 0, end: 99, hasEnd: true}, valid: true},
		{name: "bounded", header: "bytes=10-19", size: 100, want: httpByteRange{start: 10, end: 19, hasEnd: true}, valid: true},
		{name: "suffix", header: "bytes=-10", size: 100, want: httpByteRange{start: 90, end: 99, hasEnd: true}, valid: true},
		{name: "multiple rejected", header: "bytes=0-1,5-6", size: 100, valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := parseHTTPByteRange(tt.header, tt.size, false)
			if valid != tt.valid {
				t.Fatalf("valid = %v, want %v", valid, tt.valid)
			}
			if valid && got != tt.want {
				t.Fatalf("range = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestProgressiveRangeResponsesMatchBody(t *testing.T) {
	for _, tc := range []struct {
		header       string
		growing      bool
		status       int
		contentRange string
		length       int
	}{
		{"bytes=0-9999", false, 206, "bytes 0-2047/2048", 2048},
		{"bytes=100-", false, 206, "bytes 100-2047/2048", 1948},
		{"bytes=0-", false, 206, "bytes 0-2047/2048", 2048},
		{"bytes=-9999", false, 206, "bytes 0-2047/2048", 2048},
		{"bytes=2048-", false, 416, "bytes */2048", -1},
		{"bytes=100-", true, 206, "bytes 100-2047/*", 1948},
		{"bytes=0-9999", true, 206, "bytes 0-2047/*", 2048},
	} {
		t.Run(fmt.Sprintf("%s/growing=%v", tc.header, tc.growing), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "tmp_stream.mp4")
			data := bytes.Repeat([]byte{0x51}, 2048)
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
			key := PrefetchTaskKey(PrefetchTask{EpisodeID: "710000000000001", Quality: "720p"})
			stream := &progressiveStream{key: key, tempPath: path, written: 2048, done: !tc.growing, readers: 1}
			ms := &MediaServer{progressiveStreams: map[string]*progressiveStream{key: stream}}
			request := httptest.NewRequest("GET", "/video", nil)
			request.Header.Set("Range", tc.header)
			response := httptest.NewRecorder()
			ms.serveProgressiveVideo(response, request, "series", "710000000000001", 1, "720p", filepath.Join(dir, "final.mp4"))
			if response.Code != tc.status || response.Header().Get("Content-Range") != tc.contentRange {
				t.Fatalf("response = %d, %q", response.Code, response.Header().Get("Content-Range"))
			}
			if tc.length >= 0 && (response.Body.Len() != tc.length || response.Header().Get("Content-Length") != strconv.Itoa(tc.length)) {
				t.Fatalf("body=%d, length=%s, want %d", response.Body.Len(), response.Header().Get("Content-Length"), tc.length)
			}
		})
	}
}

func TestParseHTTPByteRangeAllowsFutureRangeWhileGrowing(t *testing.T) {
	got, valid := parseHTTPByteRange("bytes=2000-2099", 100, true)
	if !valid {
		t.Fatal("expected future range to be accepted while stream is growing")
	}
	if got.start != 2000 || got.end != 2099 || !got.hasEnd {
		t.Fatalf("range = %+v", got)
	}
}

func TestFFmpegProgressiveArgsUsesFragmentedMP4(t *testing.T) {
	args := ffmpegProgressiveArgs("https://media.invalid/video", "https://media.invalid/audio", "key")
	joined := strings.Join(args, " ")
	for _, required := range []string{"-movflags frag_keyframe+empty_moov+default_base_moof", "pipe:1", "-map 1:a:0"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("progressive args missing %q: %v", required, args)
		}
	}
}

func TestServeProgressiveVideoCompletedRange(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "tmp_stream_episode_v2.mp4")
	data := make([]byte, 2048)
	if err := os.WriteFile(mediaPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(mediaPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	stream := &progressiveStream{
		tempPath: mediaPath,
		file:     file,
		written:  int64(len(data)),
		done:     true,
	}
	stream.cond = sync.NewCond(&stream.mu)
	stream.key = PrefetchTaskKey(PrefetchTask{EpisodeID: "episode1", Quality: "720p"})
	server := &MediaServer{progressiveStreams: map[string]*progressiveStream{stream.key: stream}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/video?mode=progressive", nil)
	request.Header.Set("Range", "bytes=0-99")

	server.serveProgressiveVideo(response, request, "series1", "episode1", 1, "720p", filepath.Join(dir, "episode_v2.mp4"))

	if response.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusPartialContent)
	}
	if got := response.Header().Get("Content-Range"); got != "bytes 0-99/2048" {
		t.Fatalf("content range = %q", got)
	}
	if response.Body.Len() != 100 {
		t.Fatalf("body length = %d, want 100", response.Body.Len())
	}
}

func TestGrowingProgressiveSuffixUsesFinalFileSize(t *testing.T) {
	ms, stream, finalPath := growingRangeFixture(t)
	request := httptest.NewRequest("GET", "/video", nil)
	request.Header.Set("Range", "bytes=-100")
	response, done := serveRangeAsync(t, ms, stream, finalPath, request)
	waitForRangeReader(t, stream)
	if err := stream.append(bytes.Repeat([]byte{0x62}, 1024)); err != nil {
		t.Fatal(err)
	}
	stream.finish(nil)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("suffix did not finish after EOF")
	}
	if response.Code != 206 || response.Header().Get("Content-Range") != "bytes 2972-3071/3072" ||
		!bytes.Equal(response.Body.Bytes(), bytes.Repeat([]byte{0x62}, 100)) {
		t.Fatalf("suffix from wrong file size: %d, %v", response.Code, response.Header())
	}
}

func TestGrowingProgressiveFutureRangeRejectsOffsetBeyondEOF(t *testing.T) {
	ms, stream, finalPath := growingRangeFixture(t)
	request := httptest.NewRequest("GET", "/video", nil)
	request.Header.Set("Range", "bytes=3000-3999")
	response, done := serveRangeAsync(t, ms, stream, finalPath, request)
	waitForRangeReader(t, stream)
	stream.finish(nil)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("future range did not finish after EOF")
	}
	if response.Code != 416 || response.Header().Get("Content-Range") != "bytes */2048" {
		t.Fatalf("future response = %d, %v", response.Code, response.Header())
	}
}

func TestGrowingProgressiveHeadDoesNotSendMediaBody(t *testing.T) {
	ms, stream, finalPath := growingRangeFixture(t)
	request := httptest.NewRequest("HEAD", "/video", nil)
	request.Header.Set("Range", "bytes=100-")
	response := httptest.NewRecorder()
	ms.serveProgressiveVideo(response, request, "series", stream.episodeID, 1, "720p", finalPath)
	if response.Code != 206 || response.Header().Get("Content-Length") != "1948" || response.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d, %v, body %d", response.Code, response.Header(), response.Body.Len())
	}
}

func growingRangeFixture(t *testing.T) (*MediaServer, *progressiveStream, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tmp_stream.mp4")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	id := "710000000000001"
	key := PrefetchTaskKey(PrefetchTask{EpisodeID: id, Quality: "720p"})
	stream := &progressiveStream{key: key, episodeID: id, tempPath: path, file: file, readers: 1}
	stream.cond = sync.NewCond(&stream.mu)
	if err := stream.append(bytes.Repeat([]byte{0x61}, 2048)); err != nil {
		t.Fatal(err)
	}
	return &MediaServer{progressiveStreams: map[string]*progressiveStream{key: stream}}, stream, filepath.Join(dir, "final.mp4")
}

func serveRangeAsync(t *testing.T, ms *MediaServer, stream *progressiveStream, finalPath string, request *http.Request) (*httptest.ResponseRecorder, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(request.Context())
	t.Cleanup(cancel)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ms.serveProgressiveVideo(response, request.WithContext(ctx), "series", stream.episodeID, 1, "720p", finalPath)
	}()
	return response, done
}

func waitForRangeReader(t *testing.T, stream *progressiveStream) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stream.mu.Lock()
		waiting := len(stream.waiters) > 0
		stream.mu.Unlock()
		if waiting {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("range did not wait for unpublished bytes")
}
