package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestParseHTTPByteRange(t *testing.T) {
	tests := []struct {
		name   string
		header string
		size   int64
		want   httpByteRange
		valid  bool
	}{
		{name: "open", header: "bytes=0-", size: 100, want: httpByteRange{start: 0, fullSize: true}, valid: true},
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
	stream.key = prefetchTaskKey(prefetchTask{episodeID: "episode1", quality: "720p"})
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
