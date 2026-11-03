package media

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/beilunyang/openhongguo/sdk/model"
)

func TestEpisodeCachePathRejectsTraversal(t *testing.T) {
	if got := episodeCachePath("/tmp/cache", "../../secret", "720p"); got != "" {
		t.Fatalf("traversal path = %q, want empty", got)
	}
	base := filepath.Join("/tmp/cache", "episodes")
	if got := episodeCachePath("/tmp/cache", "710000000000001", "720p"); got != filepath.Join(base, "710000000000001_v2.mp4") {
		t.Fatalf("valid path = %q", got)
	}
}

func TestVideoAndPosterRejectInvalidIDs(t *testing.T) {
	ms := &MediaServer{cacheDir: t.TempDir(), port: 1}
	handler := http.HandlerFunc(ms.handleVideo)
	request := httptest.NewRequest(http.MethodGet, "/video?id=../../secret&mode=progressive", nil)
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("video status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	poster := httptest.NewRecorder()
	ms.handlePoster(poster, httptest.NewRequest(http.MethodGet, "/poster?id=../../secret", nil))
	if poster.Code != http.StatusOK || poster.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("poster response = %d/%s", poster.Code, poster.Header().Get("Content-Type"))
	}
}

func TestPosterMissingFileServesFallback(t *testing.T) {
	ms := &MediaServer{cacheDir: t.TempDir(), port: 1}
	response := httptest.NewRecorder()
	ms.handlePoster(response, httptest.NewRequest(http.MethodGet, "/poster?id=710000000000001", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("poster response = %d/%s", response.Code, response.Header().Get("Content-Type"))
	}
}

func TestImageProxyAllowsOnlyHongguoCDNHosts(t *testing.T) {
	allowed := []string{"p3-sign.douyinpic.com", "p26-sign.byteimg.com", "example.hongguo.com", "p3-reading-sign.fqnovelpic.com"}
	for _, host := range allowed {
		if !isAllowedImageHost(host) {
			t.Errorf("host %q should be allowed", host)
		}
	}
	for _, host := range []string{"127.0.0.1", "localhost", "example.com", "evil.douyinpic.com.evil.test"} {
		if isAllowedImageHost(host) {
			t.Errorf("host %q should be rejected", host)
		}
	}
}

func TestQualitySelectionMarksUnknownDefinitionAvailable(t *testing.T) {
	pb := &model.Playback{Streams: []model.Stream{
		{URL: "https://media.invalid/video", Height: 640, Width: 360},
	}}
	selection := qualitySelectionFromPlayback(pb, "720p")
	for _, option := range selection.Options {
		if option.ID == "720p" && !option.Available {
			t.Fatal("unknown 640p stream should be presented as nearest 720p")
		}
	}
}
